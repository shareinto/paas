package gitops

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shareinto/paas/internal/modules/delivery"
	"github.com/shareinto/paas/internal/shared"
)

func (s *Service) renderK8sManifests(ctx context.Context, app ApplicationRef, stageKey string, binding ClusterBindingRef, deploymentID shared.ID, artifacts []delivery.GitOpsArtifactSpec) (manifests string, workloadSummary string, err error) {
	var docs []string
	var summary []string

	type workloadEntry struct {
		workload   WorkloadRef
		config     WorkloadStageConfigRef
		containers map[string]containerImage
	}

	entries := map[string]*workloadEntry{}

	for _, artifact := range artifacts {
		if artifact.WorkloadID.IsZero() {
			continue
		}
		workload, err := s.getWorkload(ctx, app.ID, artifact.WorkloadID)
		if err != nil {
			return "", "", err
		}
		config, err := s.getWorkloadStageConfig(ctx, artifact.WorkloadID, stageKey)
		if err != nil {
			if shared.CodeOf(err) != shared.CodeNotFound {
				return "", "", err
			}
			config, err = s.getWorkloadDefaultConfig(ctx, artifact.WorkloadID)
			if err != nil && shared.CodeOf(err) != shared.CodeNotFound {
				return "", "", err
			}
		}

		name := strings.TrimSpace(workload.Name)
		if name == "" {
			name = artifact.WorkloadID.String()
		}
		entry, ok := entries[name]
		if !ok {
			entry = &workloadEntry{workload: workload, config: config, containers: map[string]containerImage{}}
			entries[name] = entry
		}
		cName := normalizeContainerName(artifact.ContainerName)
		entry.containers[cName] = containerImage{Repository: artifact.Repository, Tag: artifact.Tag, Digest: artifact.Digest}
		summary = append(summary, fmt.Sprintf("%s/%s=%s", name, cName, imageSummary(artifact.Repository, artifact.Tag, artifact.Digest)))
	}

	for name, entry := range entries {
		resourceName := truncate63(name + "-" + stageKey)
		labels := buildLabels(name, resourceName, string(app.Name), string(app.ID), stageKey, string(deploymentID))
		ns := binding.Namespace

		// ConfigMap for config files
		if len(entry.config.ConfigFiles) > 0 {
			docs = append(docs, renderConfigMap(resourceName+"-config", ns, labels, entry.config.ConfigFiles))
		}

		// Main workload (Deployment or StatefulSet)
		docs = append(docs, renderWorkload(entry.workload, entry.config, entry.containers, resourceName, ns, labels))

		// Service
		if len(entry.config.ServicePorts) > 0 {
			docs = append(docs, renderService(resourceName, ns, labels, entry.config.ServicePorts))
		}

		// Ingress
		if len(entry.config.IngressHosts) > 0 {
			docs = append(docs, renderIngress(resourceName, ns, labels, entry.config.IngressHosts, entry.config.ServicePorts))
		}
	}

	return strings.Join(docs, "\n---\n"), strings.Join(summary, "\n"), nil
}

type containerImage struct {
	Repository string
	Tag        string
	Digest     string
}

func truncate63(s string) string {
	if len(s) <= 63 {
		return s
	}
	return s[:63]
}

func buildLabels(workloadName, resourceName, appName, appID, stageKey, deploymentID string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":         workloadName,
		"app.kubernetes.io/instance":     resourceName,
		"app.kubernetes.io/managed-by":   "paas",
		"app.kubernetes.io/part-of":      appName,
		"paas.shareinto.com/application-id": appID,
		"paas.shareinto.com/stage-key":      stageKey,
		"paas.shareinto.com/deployment-id":  deploymentID,
	}
}

func renderLabelsYAML(labels map[string]string, indent string) string {
	var b strings.Builder
	for _, key := range []string{
		"app.kubernetes.io/name",
		"app.kubernetes.io/instance",
		"app.kubernetes.io/managed-by",
		"app.kubernetes.io/part-of",
		"paas.shareinto.com/application-id",
		"paas.shareinto.com/stage-key",
		"paas.shareinto.com/deployment-id",
	} {
		if v, ok := labels[key]; ok {
			fmt.Fprintf(&b, "%s%s: \"%s\"\n", indent, key, v)
		}
	}
	return b.String()
}

func renderSelectorLabelsYAML(labels map[string]string, indent string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%sapp.kubernetes.io/name: \"%s\"\n", indent, labels["app.kubernetes.io/name"])
	fmt.Fprintf(&b, "%sapp.kubernetes.io/instance: \"%s\"\n", indent, labels["app.kubernetes.io/instance"])
	return b.String()
}

func imageRef(img containerImage) string {
	ref := img.Repository
	if img.Tag != "" {
		ref += ":" + img.Tag
	}
	if img.Digest != "" {
		ref += "@" + img.Digest
	}
	return ref
}

func renderWorkload(workload WorkloadRef, config WorkloadStageConfigRef, containers map[string]containerImage, resourceName, namespace string, labels map[string]string) string {
	kind := normalizeWorkloadKind(workload.WorkloadType)
	replicas := config.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	// Build containers YAML
	containersYAML := renderContainers(config, containers)

	// Build volumes
	volumesYAML := renderVolumes(resourceName, config)

	// Build init containers
	initContainersYAML := ""
	if len(config.InitContainers) > 0 {
		initContainersYAML = renderInitContainers(config.InitContainers)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: apps/v1\n")
	fmt.Fprintf(&b, "kind: %s\n", kind)
	fmt.Fprintf(&b, "metadata:\n")
	fmt.Fprintf(&b, "  name: %s\n", resourceName)
	fmt.Fprintf(&b, "  namespace: %s\n", namespace)
	fmt.Fprintf(&b, "  labels:\n")
	b.WriteString(renderLabelsYAML(labels, "    "))
	fmt.Fprintf(&b, "spec:\n")
	fmt.Fprintf(&b, "  replicas: %d\n", replicas)
	fmt.Fprintf(&b, "  selector:\n")
	fmt.Fprintf(&b, "    matchLabels:\n")
	b.WriteString(renderSelectorLabelsYAML(labels, "      "))
	fmt.Fprintf(&b, "  template:\n")
	fmt.Fprintf(&b, "    metadata:\n")
	fmt.Fprintf(&b, "      labels:\n")
	b.WriteString(renderLabelsYAML(labels, "        "))
	fmt.Fprintf(&b, "    spec:\n")
	if initContainersYAML != "" {
		b.WriteString(initContainersYAML)
	}
	fmt.Fprintf(&b, "      containers:\n")
	b.WriteString(containersYAML)
	if volumesYAML != "" {
		fmt.Fprintf(&b, "      volumes:\n")
		b.WriteString(volumesYAML)
	}

	return b.String()
}

func renderContainers(config WorkloadStageConfigRef, containers map[string]containerImage) string {
	var b strings.Builder

	// Primary container "app"
	appImg, hasApp := containers["app"]
	if !hasApp {
		// Use first available
		for _, img := range containers {
			appImg = img
			break
		}
	}

	fmt.Fprintf(&b, "      - name: app\n")
	fmt.Fprintf(&b, "        image: %s\n", imageRef(appImg))

	// Ports
	if len(config.ServicePorts) > 0 {
		fmt.Fprintf(&b, "        ports:\n")
		for _, p := range config.ServicePorts {
			fmt.Fprintf(&b, "        - containerPort: %d\n", p.TargetPort)
			if p.Protocol != "" {
				fmt.Fprintf(&b, "          protocol: %s\n", strings.ToUpper(p.Protocol))
			}
		}
	}

	// Env
	if len(config.EnvVars) > 0 || len(config.SecretRefs) > 0 {
		fmt.Fprintf(&b, "        env:\n")
		for _, e := range config.EnvVars {
			fmt.Fprintf(&b, "        - name: %s\n", e.Name)
			fmt.Fprintf(&b, "          value: \"%s\"\n", e.Value)
		}
		for _, s := range config.SecretRefs {
			fmt.Fprintf(&b, "        - name: %s\n", s.Name)
			fmt.Fprintf(&b, "          valueFrom:\n")
			fmt.Fprintf(&b, "            secretKeyRef:\n")
			fmt.Fprintf(&b, "              name: %s\n", s.SecretRef)
			fmt.Fprintf(&b, "              key: %s\n", s.Name)
		}
	}

	// Resources
	resYAML := renderResources(config)
	if resYAML != "" {
		b.WriteString(resYAML)
	}

	// Probes
	for _, probe := range config.Probes {
		b.WriteString(renderProbe(probe))
	}

	// VolumeMounts
	vmYAML := renderVolumeMounts(config)
	if vmYAML != "" {
		fmt.Fprintf(&b, "        volumeMounts:\n")
		b.WriteString(vmYAML)
	}

	// Additional containers (sidecars)
	for name, img := range containers {
		if name == "app" {
			continue
		}
		fmt.Fprintf(&b, "      - name: %s\n", name)
		fmt.Fprintf(&b, "        image: %s\n", imageRef(img))
	}

	return b.String()
}

func renderResources(config WorkloadStageConfigRef) string {
	hasRequests := config.ResourceRequests.CPU != "" || config.ResourceRequests.Memory != ""
	hasLimits := config.ResourceLimits.CPU != "" || config.ResourceLimits.Memory != ""
	if !hasRequests && !hasLimits {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "        resources:\n")
	if hasRequests {
		fmt.Fprintf(&b, "          requests:\n")
		if config.ResourceRequests.CPU != "" {
			fmt.Fprintf(&b, "            cpu: \"%s\"\n", config.ResourceRequests.CPU)
		}
		if config.ResourceRequests.Memory != "" {
			fmt.Fprintf(&b, "            memory: \"%s\"\n", config.ResourceRequests.Memory)
		}
	}
	if hasLimits {
		fmt.Fprintf(&b, "          limits:\n")
		if config.ResourceLimits.CPU != "" {
			fmt.Fprintf(&b, "            cpu: \"%s\"\n", config.ResourceLimits.CPU)
		}
		if config.ResourceLimits.Memory != "" {
			fmt.Fprintf(&b, "            memory: \"%s\"\n", config.ResourceLimits.Memory)
		}
	}
	return b.String()
}

func renderProbe(probe WorkloadProbeRef) string {
	var probeName string
	switch probe.Name {
	case "liveness":
		probeName = "livenessProbe"
	case "readiness":
		probeName = "readinessProbe"
	case "startup":
		probeName = "startupProbe"
	default:
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "        %s:\n", probeName)
	switch probe.Type {
	case "http":
		fmt.Fprintf(&b, "          httpGet:\n")
		fmt.Fprintf(&b, "            path: %s\n", probe.Path)
		fmt.Fprintf(&b, "            port: %d\n", probe.Port)
	case "tcp":
		fmt.Fprintf(&b, "          tcpSocket:\n")
		fmt.Fprintf(&b, "            port: %d\n", probe.Port)
	case "exec":
		fmt.Fprintf(&b, "          exec:\n")
		fmt.Fprintf(&b, "            command:\n")
		for _, cmd := range probe.Command {
			fmt.Fprintf(&b, "            - %s\n", cmd)
		}
	}
	if probe.InitialDelaySeconds > 0 {
		fmt.Fprintf(&b, "          initialDelaySeconds: %d\n", probe.InitialDelaySeconds)
	}
	if probe.PeriodSeconds > 0 {
		fmt.Fprintf(&b, "          periodSeconds: %d\n", probe.PeriodSeconds)
	}
	return b.String()
}

func renderVolumeMounts(config WorkloadStageConfigRef) string {
	var b strings.Builder
	if len(config.ConfigFiles) > 0 {
		fmt.Fprintf(&b, "        - name: config-volume\n")
		fmt.Fprintf(&b, "          mountPath: %s\n", configFileMountPath(config.ConfigFiles))
	}
	for i, wd := range config.WritableDirs {
		fmt.Fprintf(&b, "        - name: writable-%d\n", i)
		fmt.Fprintf(&b, "          mountPath: %s\n", wd.MountPath)
	}
	for _, vm := range config.VolumeMounts {
		fmt.Fprintf(&b, "        - name: %s\n", vm.Name)
		fmt.Fprintf(&b, "          mountPath: %s\n", vm.MountPath)
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String()
}

func renderVolumes(resourceName string, config WorkloadStageConfigRef) string {
	var b strings.Builder
	if len(config.ConfigFiles) > 0 {
		fmt.Fprintf(&b, "      - name: config-volume\n")
		fmt.Fprintf(&b, "        configMap:\n")
		fmt.Fprintf(&b, "          name: %s-config\n", resourceName)
	}
	for i, wd := range config.WritableDirs {
		fmt.Fprintf(&b, "      - name: writable-%d\n", i)
		fmt.Fprintf(&b, "        emptyDir:\n")
		if wd.SizeLimit != "" {
			fmt.Fprintf(&b, "          sizeLimit: %s\n", wd.SizeLimit)
		} else {
			fmt.Fprintf(&b, "          {}\n")
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String()
}

func renderInitContainers(initContainers []WorkloadInitContainerRef) string {
	var b strings.Builder
	fmt.Fprintf(&b, "      initContainers:\n")
	for _, ic := range initContainers {
		fmt.Fprintf(&b, "      - name: %s\n", ic.Name)
		fmt.Fprintf(&b, "        image: %s\n", ic.Image)
		if len(ic.Command) > 0 {
			fmt.Fprintf(&b, "        command:\n")
			for _, cmd := range ic.Command {
				fmt.Fprintf(&b, "        - %s\n", cmd)
			}
		}
	}
	return b.String()
}

func renderService(resourceName, namespace string, labels map[string]string, ports []WorkloadServicePortRef) string {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: v1\n")
	fmt.Fprintf(&b, "kind: Service\n")
	fmt.Fprintf(&b, "metadata:\n")
	fmt.Fprintf(&b, "  name: %s\n", resourceName)
	fmt.Fprintf(&b, "  namespace: %s\n", namespace)
	fmt.Fprintf(&b, "  labels:\n")
	b.WriteString(renderLabelsYAML(labels, "    "))
	fmt.Fprintf(&b, "spec:\n")
	fmt.Fprintf(&b, "  selector:\n")
	b.WriteString(renderSelectorLabelsYAML(labels, "    "))
	fmt.Fprintf(&b, "  ports:\n")
	for _, p := range ports {
		fmt.Fprintf(&b, "  - port: %d\n", p.Port)
		fmt.Fprintf(&b, "    targetPort: %d\n", p.TargetPort)
		if p.Protocol != "" {
			fmt.Fprintf(&b, "    protocol: %s\n", strings.ToUpper(p.Protocol))
		}
		if p.Name != "" {
			fmt.Fprintf(&b, "    name: %s\n", p.Name)
		}
	}
	return b.String()
}

func renderIngress(resourceName, namespace string, labels map[string]string, hosts []WorkloadIngressHostRef, ports []WorkloadServicePortRef) string {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: networking.k8s.io/v1\n")
	fmt.Fprintf(&b, "kind: Ingress\n")
	fmt.Fprintf(&b, "metadata:\n")
	fmt.Fprintf(&b, "  name: %s\n", resourceName)
	fmt.Fprintf(&b, "  namespace: %s\n", namespace)
	fmt.Fprintf(&b, "  labels:\n")
	b.WriteString(renderLabelsYAML(labels, "    "))

	// Annotations
	var annotations []string
	for _, h := range hosts {
		if h.TLSRedirect {
			annotations = append(annotations, "nginx.ingress.kubernetes.io/ssl-redirect: \"true\"")
		}
		if h.Rewrite && h.RewritePath != "" {
			annotations = append(annotations, fmt.Sprintf("nginx.ingress.kubernetes.io/rewrite-target: %s", h.RewritePath))
		}
	}
	if len(annotations) > 0 {
		fmt.Fprintf(&b, "  annotations:\n")
		for _, a := range annotations {
			fmt.Fprintf(&b, "    %s\n", a)
		}
	}

	fmt.Fprintf(&b, "spec:\n")
	fmt.Fprintf(&b, "  ingressClassName: nginx\n")

	// TLS
	var tlsHosts []string
	for _, h := range hosts {
		if h.TLS {
			tlsHosts = append(tlsHosts, h.Host)
		}
	}
	if len(tlsHosts) > 0 {
		fmt.Fprintf(&b, "  tls:\n")
		fmt.Fprintf(&b, "  - hosts:\n")
		for _, h := range tlsHosts {
			fmt.Fprintf(&b, "    - %s\n", h)
		}
		fmt.Fprintf(&b, "    secretName: %s-tls\n", resourceName)
	}

	fmt.Fprintf(&b, "  rules:\n")
	defaultPort := 80
	if len(ports) > 0 {
		defaultPort = ports[0].Port
	}
	for _, h := range hosts {
		fmt.Fprintf(&b, "  - host: %s\n", h.Host)
		fmt.Fprintf(&b, "    http:\n")
		fmt.Fprintf(&b, "      paths:\n")
		path := h.Path
		if path == "" {
			path = "/"
		}
		pathType := h.PathType
		if pathType == "" {
			pathType = "Prefix"
		}
		port := defaultPort
		if h.ServicePort != "" {
			fmt.Sscanf(h.ServicePort, "%d", &port)
		}
		fmt.Fprintf(&b, "      - path: %s\n", path)
		fmt.Fprintf(&b, "        pathType: %s\n", pathType)
		fmt.Fprintf(&b, "        backend:\n")
		fmt.Fprintf(&b, "          service:\n")
		fmt.Fprintf(&b, "            name: %s\n", resourceName)
		fmt.Fprintf(&b, "            port:\n")
		fmt.Fprintf(&b, "              number: %d\n", port)
	}
	return b.String()
}

func renderConfigMap(name, namespace string, labels map[string]string, configFiles []WorkloadConfigFileRef) string {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: v1\n")
	fmt.Fprintf(&b, "kind: ConfigMap\n")
	fmt.Fprintf(&b, "metadata:\n")
	fmt.Fprintf(&b, "  name: %s\n", name)
	fmt.Fprintf(&b, "  namespace: %s\n", namespace)
	fmt.Fprintf(&b, "  labels:\n")
	b.WriteString(renderLabelsYAML(labels, "    "))
	fmt.Fprintf(&b, "data:\n")
	for _, cf := range configFiles {
		key := sanitizeConfigFileKey(cf.MountPath)
		content := cf.Content
		if cf.Base64Encoded {
			decoded, err := base64.StdEncoding.DecodeString(content)
			if err == nil {
				content = string(decoded)
			}
		}
		fmt.Fprintf(&b, "  %s: |\n", key)
		for _, line := range strings.Split(content, "\n") {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}
	return b.String()
}

func sanitizeConfigFileKey(mountPath string) string {
	name := filepath.Base(mountPath)
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

func configFileMountPath(configFiles []WorkloadConfigFileRef) string {
	if len(configFiles) == 0 {
		return "/etc/config"
	}
	dir := filepath.Dir(configFiles[0].MountPath)
	if dir == "" || dir == "." {
		return "/etc/config"
	}
	return dir
}
