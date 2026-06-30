package gitops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
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
		config, err := s.resolveWorkloadConfig(ctx, artifact.WorkloadID, stageKey)
		if err != nil && shared.CodeOf(err) != shared.CodeNotFound {
			return "", "", err
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
		resourceName := truncate63(name)
		labels := buildLabels(name, resourceName, string(app.Name), string(app.ID), stageKey, string(deploymentID))
		ns := binding.Namespace

		// ConfigMap for config files
		configFiles := allConfigFiles(entry.config)
		if len(configFiles) > 0 {
			docs = append(docs, renderConfigMap(resourceName+"-config", ns, labels, configFiles))
		}

		// Main workload (Deployment or StatefulSet)
		docs = append(docs, renderWorkload(entry.workload, entry.config, entry.containers, resourceName, ns, labels))

		// Service
		if len(entry.config.ServicePorts) > 0 {
			docs = append(docs, renderService(resourceName, ns, labels, entry.config.ServicePorts))
		}

		// Ingress
		if len(entry.config.IngressHosts) > 0 {
			docs = append(docs, renderIngress(resourceName, ns, labels, entry.config.IngressHosts, entry.config.ServicePorts, entry.config.ValuesOverride))
		}
	}

	return strings.Join(docs, "\n---\n"), strings.Join(summary, "\n"), nil
}

type containerImage struct {
	Repository string
	Tag        string
	Digest     string
}

type containerRenderConfig struct {
	EnvVars          []WorkloadEnvVarRef
	SecretRefs       []WorkloadSecretRef
	RawEnvVars       []map[string]any
	RawPorts         []map[string]any
	ConfigFiles      []WorkloadConfigFileRef
	WritableDirs     []WorkloadWritableDirRef
	VolumeMounts     []WorkloadVolumeMountRef
	RawVolumeMounts  []map[string]any
	RawResources     map[string]any
	RawProbes        map[string]map[string]any
	Probes           []WorkloadProbeRef
	ResourceRequests WorkloadResourceListRef
	ResourceLimits   WorkloadResourceListRef
}

const (
	appLogsVolumeName  = "app-logs"
	appLogsHostPath    = "/cloud"
	appLogsMountPath   = "/logs"
	appLogsSubPathExpr = "macc/$(APP_NAME)/$(POD_NAME)"
	appNameFieldPath   = "metadata.labels['app']"
	podNameFieldPath   = "metadata.name"
)

func truncate63(s string) string {
	if len(s) <= 63 {
		return s
	}
	return s[:63]
}

func buildLabels(workloadName, resourceName, appName, appID, stageKey, deploymentID string) map[string]string {
	return map[string]string{
		"app":                               workloadName,
		"app.kubernetes.io/name":            workloadName,
		"app.kubernetes.io/instance":        resourceName,
		"app.kubernetes.io/managed-by":      "paas",
		"app.kubernetes.io/part-of":         appName,
		"paas.shareinto.com/application-id": appID,
		"paas.shareinto.com/stage-key":      stageKey,
		"paas.shareinto.com/deployment-id":  deploymentID,
	}
}

func renderLabelsYAML(labels map[string]string, indent string) string {
	var b strings.Builder
	for _, key := range []string{
		"app",
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

	// Build template-managed init container.
	initContainersYAML := renderInitContainer(config)

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
	if workloadHostNetwork(config) {
		fmt.Fprintf(&b, "      hostNetwork: true\n")
	}
	b.WriteString(renderDNSConfig())
	b.WriteString(renderSoftPodAntiAffinity(labels))
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

func workloadHostNetwork(config WorkloadStageConfigRef) bool {
	switch strings.ToLower(strings.TrimSpace(config.NetworkMode)) {
	case "host", "host_network", "hostnetwork":
		return true
	}
	if v, ok := config.ValuesOverride["hostNetwork"].(bool); ok && v {
		return true
	}
	if v, ok := config.ValuesOverride["host_network"].(bool); ok && v {
		return true
	}
	if v, ok := config.ValuesOverride["networkMode"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "host", "host_network", "hostnetwork":
			return true
		}
	}
	if v, ok := config.ValuesOverride["network_mode"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "host", "host_network", "hostnetwork":
			return true
		}
	}
	return false
}

func renderSoftPodAntiAffinity(labels map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "      affinity:\n")
	fmt.Fprintf(&b, "        podAntiAffinity:\n")
	fmt.Fprintf(&b, "          preferredDuringSchedulingIgnoredDuringExecution:\n")
	fmt.Fprintf(&b, "          - weight: 100\n")
	fmt.Fprintf(&b, "            podAffinityTerm:\n")
	fmt.Fprintf(&b, "              labelSelector:\n")
	fmt.Fprintf(&b, "                matchLabels:\n")
	fmt.Fprintf(&b, "                  app.kubernetes.io/name: \"%s\"\n", labels["app.kubernetes.io/name"])
	fmt.Fprintf(&b, "                  app.kubernetes.io/instance: \"%s\"\n", labels["app.kubernetes.io/instance"])
	fmt.Fprintf(&b, "              topologyKey: \"kubernetes.io/hostname\"\n")
	return b.String()
}

func renderDNSConfig() string {
	var b strings.Builder
	fmt.Fprintf(&b, "      dnsConfig:\n")
	fmt.Fprintf(&b, "        options:\n")
	fmt.Fprintf(&b, "        - name: ndots\n")
	fmt.Fprintf(&b, "          value: \"1\"\n")
	return b.String()
}

func renderContainers(config WorkloadStageConfigRef, containers map[string]containerImage) string {
	var b strings.Builder
	names := sortedContainerNames(containers)
	if len(names) == 0 {
		names = []string{"app"}
		containers = map[string]containerImage{"app": {}}
	}
	overrides := containerOverrides(config.ValuesOverride)
	for idx, name := range names {
		settings := containerSettings(config, name, idx == 0, overrides[name])
		fmt.Fprintf(&b, "      - name: %s\n", name)
		fmt.Fprintf(&b, "        image: %s\n", imageRef(containers[name]))
		renderContainerDetails(&b, config, name, settings)
	}

	return b.String()
}

func sortedContainerNames(containers map[string]containerImage) []string {
	names := make([]string, 0, len(containers))
	for name := range containers {
		if strings.TrimSpace(name) == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func renderContainerDetails(b *strings.Builder, config WorkloadStageConfigRef, name string, settings containerRenderConfig) {
	if len(settings.RawPorts) > 0 {
		fmt.Fprintf(b, "        ports:\n")
		for _, raw := range settings.RawPorts {
			b.WriteString(renderRawListItem(raw, "        "))
		}
	} else if len(config.ServicePorts) > 0 {
		fmt.Fprintf(b, "        ports:\n")
		for _, p := range config.ServicePorts {
			fmt.Fprintf(b, "        - containerPort: %d\n", p.TargetPort)
			if p.Protocol != "" {
				fmt.Fprintf(b, "          protocol: %s\n", strings.ToUpper(p.Protocol))
			}
		}
	}
	if len(settings.EnvVars) > 0 || len(settings.SecretRefs) > 0 || len(settings.RawEnvVars) > 0 || shouldRenderAppLogsMount() {
		fmt.Fprintf(b, "        env:\n")
		seenEnv := map[string]struct{}{}
		for _, e := range settings.EnvVars {
			envName := strings.TrimSpace(e.Name)
			if envName == "" {
				continue
			}
			seenEnv[envName] = struct{}{}
			fmt.Fprintf(b, "        - name: %s\n", e.Name)
			fmt.Fprintf(b, "          value: \"%s\"\n", e.Value)
		}
		for _, s := range settings.SecretRefs {
			envName := strings.TrimSpace(s.Name)
			if envName == "" {
				continue
			}
			if _, ok := seenEnv[envName]; ok {
				continue
			}
			seenEnv[envName] = struct{}{}
			fmt.Fprintf(b, "        - name: %s\n", s.Name)
			fmt.Fprintf(b, "          valueFrom:\n")
			fmt.Fprintf(b, "            secretKeyRef:\n")
			fmt.Fprintf(b, "              name: %s\n", s.SecretRef)
			fmt.Fprintf(b, "              key: %s\n", s.Name)
		}
		for _, raw := range settings.RawEnvVars {
			envName := strings.TrimSpace(fmt.Sprint(raw["name"]))
			if envName != "" {
				if _, ok := seenEnv[envName]; ok {
					continue
				}
				seenEnv[envName] = struct{}{}
			}
			b.WriteString(renderRawListItem(raw, "        "))
		}
		renderDefaultPodIdentityEnv(b, seenEnv, "        ")
	}
	if len(settings.RawResources) > 0 {
		fmt.Fprintf(b, "        resources:\n")
		renderRawYAML(b, settings.RawResources, "          ")
	} else if resYAML := renderContainerResources(settings); resYAML != "" {
		b.WriteString(resYAML)
	}
	for _, probe := range settings.Probes {
		b.WriteString(renderProbe(probe))
	}
	for _, probeName := range []string{"livenessProbe", "readinessProbe", "startupProbe"} {
		if raw := settings.RawProbes[probeName]; len(raw) > 0 {
			fmt.Fprintf(b, "        %s:\n", probeName)
			renderRawYAML(b, raw, "          ")
		}
	}
	if vmYAML := renderVolumeMounts(name, settings); vmYAML != "" {
		fmt.Fprintf(b, "        volumeMounts:\n")
		b.WriteString(vmYAML)
	}
}

func renderResources(config WorkloadStageConfigRef) string {
	return renderResourceLists(config.ResourceRequests, config.ResourceLimits)
}

func renderContainerResources(config containerRenderConfig) string {
	return renderResourceLists(config.ResourceRequests, config.ResourceLimits)
}

func renderResourceLists(requests WorkloadResourceListRef, limits WorkloadResourceListRef) string {
	hasRequests := requests.CPU != "" || requests.Memory != ""
	hasLimits := limits.CPU != "" || limits.Memory != ""
	if !hasRequests && !hasLimits {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "        resources:\n")
	if hasRequests {
		fmt.Fprintf(&b, "          requests:\n")
		if requests.CPU != "" {
			fmt.Fprintf(&b, "            cpu: \"%s\"\n", requests.CPU)
		}
		if requests.Memory != "" {
			fmt.Fprintf(&b, "            memory: \"%s\"\n", requests.Memory)
		}
	}
	if hasLimits {
		fmt.Fprintf(&b, "          limits:\n")
		if limits.CPU != "" {
			fmt.Fprintf(&b, "            cpu: \"%s\"\n", limits.CPU)
		}
		if limits.Memory != "" {
			fmt.Fprintf(&b, "            memory: \"%s\"\n", limits.Memory)
		}
	}
	return b.String()
}

func containerSettings(config WorkloadStageConfigRef, name string, primary bool, override containerRenderConfig) containerRenderConfig {
	settings := override
	if primary && emptyContainerSettings(settings) {
		settings = containerRenderConfig{
			EnvVars:          config.EnvVars,
			SecretRefs:       config.SecretRefs,
			ConfigFiles:      config.ConfigFiles,
			WritableDirs:     config.WritableDirs,
			VolumeMounts:     config.VolumeMounts,
			Probes:           config.Probes,
			ResourceRequests: config.ResourceRequests,
			ResourceLimits:   config.ResourceLimits,
		}
	}
	return settings
}

func emptyContainerSettings(settings containerRenderConfig) bool {
	return len(settings.EnvVars) == 0 &&
		len(settings.SecretRefs) == 0 &&
		len(settings.RawEnvVars) == 0 &&
		len(settings.RawPorts) == 0 &&
		len(settings.ConfigFiles) == 0 &&
		len(settings.WritableDirs) == 0 &&
		len(settings.VolumeMounts) == 0 &&
		len(settings.RawVolumeMounts) == 0 &&
		len(settings.RawResources) == 0 &&
		len(settings.RawProbes) == 0 &&
		len(settings.Probes) == 0 &&
		settings.ResourceRequests.CPU == "" &&
		settings.ResourceRequests.Memory == "" &&
		settings.ResourceLimits.CPU == "" &&
		settings.ResourceLimits.Memory == ""
}

type valuesOverrideContainer struct {
	Name             string                   `json:"name"`
	EnvVars          []WorkloadEnvVarRef      `json:"env_vars"`
	SecretRefs       []WorkloadSecretRef      `json:"secret_refs"`
	ConfigFiles      []WorkloadConfigFileRef  `json:"config_files"`
	WritableDirs     []WorkloadWritableDirRef `json:"writable_dirs"`
	VolumeMounts     []WorkloadVolumeMountRef `json:"volume_mounts"`
	RawVolumeMounts  []map[string]any         `json:"raw_volume_mounts"`
	Probes           []WorkloadProbeRef       `json:"probes"`
	LivenessProbe    map[string]any           `json:"liveness_probe"`
	ReadinessProbe   map[string]any           `json:"readiness_probe"`
	StartupProbe     map[string]any           `json:"startup_probe"`
	ResourceRequests WorkloadResourceListRef  `json:"resource_requests"`
	ResourceLimits   WorkloadResourceListRef  `json:"resource_limits"`
	CPU              string                   `json:"cpu"`
	Memory           string                   `json:"memory"`
	LimitCPU         string                   `json:"limit_cpu"`
	LimitMemory      string                   `json:"limit_memory"`
}

func containerOverrides(valuesOverride map[string]any) map[string]containerRenderConfig {
	out := map[string]containerRenderConfig{}
	raw, ok := valuesOverride["containers"]
	if !ok || raw == nil {
		return out
	}
	var items []valuesOverrideContainer
	if err := mapAny(raw, &items); err != nil {
		return out
	}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		req := item.ResourceRequests
		lim := item.ResourceLimits
		if req.CPU == "" {
			req.CPU = item.CPU
		}
		if req.Memory == "" {
			req.Memory = item.Memory
		}
		if lim.CPU == "" {
			lim.CPU = item.LimitCPU
		}
		if lim.Memory == "" {
			lim.Memory = item.LimitMemory
		}
		rawProbes := rawContainerProbes(valuesOverride, name)
		if probe := normalizeRawProbe(item.LivenessProbe); len(probe) > 0 {
			rawProbes["livenessProbe"] = probe
		}
		if probe := normalizeRawProbe(item.ReadinessProbe); len(probe) > 0 {
			rawProbes["readinessProbe"] = probe
		}
		if probe := normalizeRawProbe(item.StartupProbe); len(probe) > 0 {
			rawProbes["startupProbe"] = probe
		}
		out[name] = containerRenderConfig{
			EnvVars:          item.EnvVars,
			SecretRefs:       item.SecretRefs,
			RawEnvVars:       rawContainerList(valuesOverride, name, "env"),
			RawPorts:         rawContainerList(valuesOverride, name, "ports"),
			ConfigFiles:      item.ConfigFiles,
			WritableDirs:     item.WritableDirs,
			VolumeMounts:     item.VolumeMounts,
			RawVolumeMounts:  append(item.RawVolumeMounts, rawContainerVolumeMounts(valuesOverride, name)...),
			RawResources:     rawContainerMap(valuesOverride, name, "resources"),
			RawProbes:        rawProbes,
			Probes:           item.Probes,
			ResourceRequests: req,
			ResourceLimits:   lim,
		}
	}
	return out
}

func mapAny(raw any, target any) error {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
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

func renderVolumeMounts(containerName string, settings containerRenderConfig) string {
	var b strings.Builder
	seenMountPath := map[string]struct{}{}
	configFileDirs := configFileParentDirs(settings.ConfigFiles)
	for _, cf := range settings.ConfigFiles {
		mountPath := strings.TrimSpace(cf.MountPath)
		if mountPath == "" {
			continue
		}
		seenMountPath[mountPath] = struct{}{}
		fmt.Fprintf(&b, "        - name: config-volume\n")
		fmt.Fprintf(&b, "          mountPath: %s\n", cf.MountPath)
		fmt.Fprintf(&b, "          subPath: %s\n", configFileKey(containerName, cf))
	}
	if shouldRenderAppLogsMount() {
		addAppLogsVolumeMount(&b, seenMountPath, "        ")
	}
	for i, wd := range settings.WritableDirs {
		mountPath := strings.TrimSpace(wd.MountPath)
		if mountPath == "" {
			continue
		}
		if _, ok := seenMountPath[mountPath]; ok {
			continue
		}
		seenMountPath[mountPath] = struct{}{}
		fmt.Fprintf(&b, "        - name: %s\n", writableVolumeName(containerName, i))
		fmt.Fprintf(&b, "          mountPath: %s\n", wd.MountPath)
	}
	for _, vm := range settings.VolumeMounts {
		mountPath := strings.TrimSpace(vm.MountPath)
		if mountPath == "" {
			continue
		}
		if _, ok := seenMountPath[mountPath]; ok {
			continue
		}
		seenMountPath[mountPath] = struct{}{}
		fmt.Fprintf(&b, "        - name: %s\n", vm.Name)
		fmt.Fprintf(&b, "          mountPath: %s\n", vm.MountPath)
	}
	for _, raw := range settings.RawVolumeMounts {
		mountPath := strings.TrimSpace(fmt.Sprint(raw["mountPath"]))
		if mountPath == "" {
			mountPath = strings.TrimSpace(fmt.Sprint(raw["mount_path"]))
		}
		if shouldSkipDirectoryMountForConfigFiles(raw, mountPath, configFileDirs) {
			continue
		}
		if mountPath != "" {
			if _, ok := seenMountPath[mountPath]; ok {
				continue
			}
			seenMountPath[mountPath] = struct{}{}
		}
		b.WriteString(renderRawListItem(raw, "        "))
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String()
}

func configFileParentDirs(files []WorkloadConfigFileRef) map[string]struct{} {
	out := map[string]struct{}{}
	for _, file := range files {
		mountPath := strings.TrimSpace(file.MountPath)
		if mountPath == "" {
			continue
		}
		dir := filepath.Clean(filepath.Dir(mountPath))
		if dir == "." || dir == "/" {
			continue
		}
		out[dir] = struct{}{}
	}
	return out
}

func shouldSkipDirectoryMountForConfigFiles(raw map[string]any, mountPath string, configFileDirs map[string]struct{}) bool {
	if mountPath == "" || len(configFileDirs) == 0 {
		return false
	}
	if hasRawSubPath(raw) {
		return false
	}
	clean := filepath.Clean(mountPath)
	_, ok := configFileDirs[clean]
	return ok
}

func hasRawSubPath(raw map[string]any) bool {
	for _, key := range []string{"subPath", "sub_path", "subPathExpr", "sub_path_expr"} {
		if strings.TrimSpace(fmt.Sprint(raw[key])) != "" && raw[key] != nil {
			return true
		}
	}
	return false
}

func renderVolumes(resourceName string, config WorkloadStageConfigRef) string {
	var b strings.Builder
	seenVolumes := map[string]struct{}{}
	addVolumeName := func(name string) bool {
		name = strings.TrimSpace(name)
		if name == "" {
			return false
		}
		if _, ok := seenVolumes[name]; ok {
			return false
		}
		seenVolumes[name] = struct{}{}
		return true
	}
	if len(allConfigFiles(config)) > 0 {
		addVolumeName("config-volume")
		fmt.Fprintf(&b, "      - name: config-volume\n")
		fmt.Fprintf(&b, "        configMap:\n")
		fmt.Fprintf(&b, "          name: %s-config\n", resourceName)
	}
	if shouldRenderAppLogsMount() {
		addVolumeName(appLogsVolumeName)
		fmt.Fprintf(&b, "      - name: %s\n", appLogsVolumeName)
		fmt.Fprintf(&b, "        hostPath:\n")
		fmt.Fprintf(&b, "          path: %s\n", appLogsHostPath)
		fmt.Fprintf(&b, "          type: DirectoryOrCreate\n")
	}
	overrides := containerOverrides(config.ValuesOverride)
	for name, settings := range overrides {
		for i, wd := range settings.WritableDirs {
			volumeName := writableVolumeName(name, i)
			if !addVolumeName(volumeName) {
				continue
			}
			fmt.Fprintf(&b, "      - name: %s\n", volumeName)
			fmt.Fprintf(&b, "        emptyDir:\n")
			if wd.SizeLimit != "" {
				fmt.Fprintf(&b, "          sizeLimit: %s\n", wd.SizeLimit)
			} else {
				fmt.Fprintf(&b, "          {}\n")
			}
		}
	}
	if len(overrides) == 0 {
		for i, wd := range config.WritableDirs {
			volumeName := writableVolumeName("app", i)
			if !addVolumeName(volumeName) {
				continue
			}
			fmt.Fprintf(&b, "      - name: %s\n", volumeName)
			fmt.Fprintf(&b, "        emptyDir:\n")
			if wd.SizeLimit != "" {
				fmt.Fprintf(&b, "          sizeLimit: %s\n", wd.SizeLimit)
			} else {
				fmt.Fprintf(&b, "          {}\n")
			}
		}
	}
	usedRawVolumes := usedRawVolumeNames(config)
	for _, raw := range rawVolumes(config.ValuesOverride) {
		name := strings.TrimSpace(fmt.Sprint(raw["name"]))
		if name == "" {
			continue
		}
		if _, ok := usedRawVolumes[name]; !ok {
			continue
		}
		if !addVolumeName(name) {
			continue
		}
		b.WriteString(renderRawListItem(raw, "      "))
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String()
}

func usedRawVolumeNames(config WorkloadStageConfigRef) map[string]struct{} {
	used := map[string]struct{}{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" {
			used[name] = struct{}{}
		}
	}
	for _, vm := range config.VolumeMounts {
		add(vm.Name)
	}
	overrides := containerOverrides(config.ValuesOverride)
	for _, settings := range overrides {
		configFileDirs := configFileParentDirs(settings.ConfigFiles)
		for _, vm := range settings.VolumeMounts {
			add(vm.Name)
		}
		for _, raw := range settings.RawVolumeMounts {
			mountPath := strings.TrimSpace(fmt.Sprint(raw["mountPath"]))
			if mountPath == "" {
				mountPath = strings.TrimSpace(fmt.Sprint(raw["mount_path"]))
			}
			if shouldSkipDirectoryMountForConfigFiles(raw, mountPath, configFileDirs) {
				continue
			}
			add(fmt.Sprint(raw["name"]))
		}
	}
	if len(overrides) == 0 {
		for _, raw := range rawContainerVolumeMounts(config.ValuesOverride, "app") {
			add(fmt.Sprint(raw["name"]))
		}
	}
	return used
}

func renderInitContainer(config WorkloadStageConfigRef) string {
	var b strings.Builder
	mounts := initContainerVolumeMounts(config)
	envs := initContainerEnvVars(mounts)
	initSpecs := initContainerDirSpecs(config, mounts)
	fmt.Fprintf(&b, "      initContainers:\n")
	fmt.Fprintf(&b, "      - name: init\n")
	fmt.Fprintf(&b, "        image: cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/busybox:1.34.1\n")
	fmt.Fprintf(&b, "        securityContext:\n")
	fmt.Fprintf(&b, "          runAsUser: 0\n")
	fmt.Fprintf(&b, "        command:\n")
	fmt.Fprintf(&b, "        - sh\n")
	fmt.Fprintf(&b, "        - -c\n")
	fmt.Fprintf(&b, "        - %s\n", initContainerInitCommand(initSpecs))
	if len(envs) > 0 {
		fmt.Fprintf(&b, "        env:\n")
		for _, env := range envs {
			b.WriteString(renderRawListItem(env, "        "))
		}
	}
	if len(mounts) > 0 {
		fmt.Fprintf(&b, "        volumeMounts:\n")
		for _, mount := range mounts {
			b.WriteString(renderRawListItem(mount, "        "))
		}
	}
	return b.String()
}

type initDirSpec struct {
	Path       string
	OwnerGroup string
	Mode       string
}

func initContainerDirSpecs(config WorkloadStageConfigRef, mounts []map[string]any) []initDirSpec {
	specsByPath := map[string]initDirSpec{
		"/logs":       {Path: "/logs", OwnerGroup: "10001:0", Mode: "775"},
		"/logs/nginx": {Path: "/logs/nginx", OwnerGroup: "10001:0", Mode: "775"},
	}
	for _, mount := range mounts {
		mountPath := strings.TrimSpace(fmt.Sprint(mount["mountPath"]))
		if mountPath == "" {
			mountPath = strings.TrimSpace(fmt.Sprint(mount["mount_path"]))
		}
		if mountPath == "" {
			continue
		}
		if _, ok := specsByPath[mountPath]; !ok {
			specsByPath[mountPath] = initDirSpec{Path: mountPath}
		}
	}
	addWritableDirs := func(dirs []WorkloadWritableDirRef) {
		for _, wd := range dirs {
			path := strings.TrimSpace(wd.MountPath)
			if path == "" {
				continue
			}
			spec := specsByPath[path]
			spec.Path = path
			if strings.TrimSpace(wd.OwnerGroup) != "" {
				spec.OwnerGroup = strings.TrimSpace(wd.OwnerGroup)
			}
			if strings.TrimSpace(wd.Mode) != "" {
				spec.Mode = strings.TrimSpace(wd.Mode)
			}
			specsByPath[path] = spec
		}
	}
	overrides := containerOverrides(config.ValuesOverride)
	if len(overrides) > 0 {
		names := make([]string, 0, len(overrides))
		for name := range overrides {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			addWritableDirs(overrides[name].WritableDirs)
		}
	} else {
		addWritableDirs(config.WritableDirs)
	}
	specs := make([]initDirSpec, 0, len(specsByPath))
	for _, spec := range specsByPath {
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Path < specs[j].Path })
	return specs
}

func initContainerInitCommand(specs []initDirSpec) string {
	paths := make([]string, 0, len(specs))
	for _, spec := range specs {
		if strings.TrimSpace(spec.Path) != "" {
			paths = append(paths, shellQuote(spec.Path))
		}
	}
	var parts []string
	if len(paths) > 0 {
		parts = append(parts, "mkdir -p "+strings.Join(paths, " "))
	}
	for _, spec := range specs {
		path := strings.TrimSpace(spec.Path)
		if path == "" {
			continue
		}
		if ownerGroup := strings.TrimSpace(spec.OwnerGroup); ownerGroup != "" {
			parts = append(parts, "chown "+shellQuote(ownerGroup)+" "+shellQuote(path))
		}
		if mode := strings.TrimSpace(spec.Mode); mode != "" {
			parts = append(parts, "chmod "+shellQuote(mode)+" "+shellQuote(path))
		}
	}
	if len(parts) == 0 {
		return "true"
	}
	return strings.Join(parts, " && ")
}

func shellQuote(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func initContainerVolumeMounts(config WorkloadStageConfigRef) []map[string]any {
	var mounts []map[string]any
	seenMountPath := map[string]struct{}{}
	add := func(item map[string]any) {
		mountPath := strings.TrimSpace(fmt.Sprint(item["mountPath"]))
		if mountPath == "" {
			mountPath = strings.TrimSpace(fmt.Sprint(item["mount_path"]))
		}
		if mountPath == "" {
			return
		}
		if _, ok := seenMountPath[mountPath]; ok {
			return
		}
		seenMountPath[mountPath] = struct{}{}
		mounts = append(mounts, item)
	}
	addWritableDirs := func(containerName string, dirs []WorkloadWritableDirRef) {
		for i, wd := range dirs {
			if strings.TrimSpace(wd.MountPath) == "" {
				continue
			}
			add(map[string]any{
				"name":      writableVolumeName(containerName, i),
				"mountPath": wd.MountPath,
			})
		}
	}
	if shouldRenderAppLogsMount() {
		add(defaultAppLogsVolumeMount())
	}
	overrides := containerOverrides(config.ValuesOverride)
	if len(overrides) > 0 {
		names := make([]string, 0, len(overrides))
		for name := range overrides {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			settings := overrides[name]
			configFileDirs := configFileParentDirs(settings.ConfigFiles)
			addWritableDirs(name, settings.WritableDirs)
			for _, raw := range settings.RawVolumeMounts {
				mountPath := strings.TrimSpace(fmt.Sprint(raw["mountPath"]))
				if mountPath == "" {
					mountPath = strings.TrimSpace(fmt.Sprint(raw["mount_path"]))
				}
				if hasRawFixedSubPath(raw) {
					continue
				}
				if shouldSkipDirectoryMountForConfigFiles(raw, mountPath, configFileDirs) {
					continue
				}
				if strings.TrimSpace(fmt.Sprint(raw["name"])) == "" {
					continue
				}
				add(raw)
			}
		}
		return mounts
	}
	addWritableDirs("app", config.WritableDirs)
	for _, raw := range rawContainerVolumeMounts(config.ValuesOverride, "app") {
		if strings.TrimSpace(fmt.Sprint(raw["name"])) == "" {
			continue
		}
		add(raw)
	}
	return mounts
}

func hasRawFixedSubPath(raw map[string]any) bool {
	for _, key := range []string{"subPath", "sub_path"} {
		if strings.TrimSpace(fmt.Sprint(raw[key])) != "" && raw[key] != nil {
			return true
		}
	}
	return false
}

func initContainerEnvVars(mounts []map[string]any) []map[string]any {
	needs := map[string]struct{}{}
	for _, mount := range mounts {
		for _, key := range []string{"subPathExpr", "sub_path_expr"} {
			expr := fmt.Sprint(mount[key])
			for _, name := range shellVariableNames(expr) {
				needs[name] = struct{}{}
			}
		}
	}
	var envs []map[string]any
	if _, ok := needs["APP_NAME"]; ok {
		envs = append(envs, map[string]any{
			"name":      "APP_NAME",
			"valueFrom": map[string]any{"fieldRef": map[string]any{"fieldPath": appNameFieldPath}},
		})
	}
	if _, ok := needs["POD_NAME"]; ok {
		envs = append(envs, map[string]any{
			"name":      "POD_NAME",
			"valueFrom": map[string]any{"fieldRef": map[string]any{"fieldPath": podNameFieldPath}},
		})
	}
	return envs
}

func shouldRenderAppLogsMount() bool {
	return true
}

func defaultAppLogsVolumeMount() map[string]any {
	return map[string]any{
		"name":        appLogsVolumeName,
		"mountPath":   appLogsMountPath,
		"subPathExpr": appLogsSubPathExpr,
	}
}

func addAppLogsVolumeMount(b *strings.Builder, seenMountPath map[string]struct{}, indent string) {
	if _, ok := seenMountPath[appLogsMountPath]; ok {
		return
	}
	seenMountPath[appLogsMountPath] = struct{}{}
	fmt.Fprintf(b, "%s- name: %s\n", indent, appLogsVolumeName)
	fmt.Fprintf(b, "%s  mountPath: %s\n", indent, appLogsMountPath)
	fmt.Fprintf(b, "%s  subPathExpr: %s\n", indent, appLogsSubPathExpr)
}

func renderDefaultPodIdentityEnv(b *strings.Builder, seenEnv map[string]struct{}, indent string) {
	if _, ok := seenEnv["APP_NAME"]; !ok {
		seenEnv["APP_NAME"] = struct{}{}
		fmt.Fprintf(b, "%s- name: APP_NAME\n", indent)
		fmt.Fprintf(b, "%s  valueFrom:\n", indent)
		fmt.Fprintf(b, "%s    fieldRef:\n", indent)
		fmt.Fprintf(b, "%s      fieldPath: %s\n", indent, appNameFieldPath)
	}
	if _, ok := seenEnv["POD_NAME"]; !ok {
		seenEnv["POD_NAME"] = struct{}{}
		fmt.Fprintf(b, "%s- name: POD_NAME\n", indent)
		fmt.Fprintf(b, "%s  valueFrom:\n", indent)
		fmt.Fprintf(b, "%s    fieldRef:\n", indent)
		fmt.Fprintf(b, "%s      fieldPath: %s\n", indent, podNameFieldPath)
	}
}

func shellVariableNames(expr string) []string {
	var names []string
	for {
		start := strings.Index(expr, "$(")
		if start < 0 {
			return names
		}
		rest := expr[start+2:]
		end := strings.Index(rest, ")")
		if end < 0 {
			return names
		}
		name := rest[:end]
		if name != "" {
			names = append(names, name)
		}
		expr = rest[end+1:]
	}
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

func renderIngress(resourceName, namespace string, labels map[string]string, hosts []WorkloadIngressHostRef, ports []WorkloadServicePortRef, valuesOverride map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: networking.k8s.io/v1\n")
	fmt.Fprintf(&b, "kind: Ingress\n")
	fmt.Fprintf(&b, "metadata:\n")
	fmt.Fprintf(&b, "  name: %s\n", resourceName)
	fmt.Fprintf(&b, "  namespace: %s\n", namespace)
	fmt.Fprintf(&b, "  labels:\n")
	b.WriteString(renderLabelsYAML(labels, "    "))

	// TLS
	var tlsHosts []string
	for _, h := range hosts {
		if h.TLS {
			tlsHosts = append(tlsHosts, h.Host)
		}
	}

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
	for _, annotation := range ingressAnnotations(valuesOverride, len(tlsHosts) > 0) {
		annotations = append(annotations, annotation)
	}
	if len(annotations) > 0 {
		fmt.Fprintf(&b, "  annotations:\n")
		for _, a := range annotations {
			fmt.Fprintf(&b, "    %s\n", a)
		}
	}

	fmt.Fprintf(&b, "spec:\n")
	fmt.Fprintf(&b, "  ingressClassName: higress\n")

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

func ingressAnnotations(valuesOverride map[string]any, hasTLS bool) []string {
	merged := map[string]string{}
	collectAnnotationMap(merged, valuesOverride["ingressAnnotations"])
	if compat, ok := valuesOverride["k8sCompat"].(map[string]any); ok {
		if ingress, ok := compat["ingress"].(map[string]any); ok {
			collectAnnotationMap(merged, ingress["annotations"])
		}
	}
	if hasTLS {
		if _, ok := merged["cert-manager.io/cluster-issuer"]; !ok {
			merged["cert-manager.io/cluster-issuer"] = "letsencrypt-prod"
		}
	}
	if len(merged) == 0 {
		return nil
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	annotations := make([]string, 0, len(keys))
	for _, key := range keys {
		annotations = append(annotations, fmt.Sprintf("%s: %q", key, merged[key]))
	}
	return annotations
}

func collectAnnotationMap(target map[string]string, raw any) {
	switch annotations := raw.(type) {
	case map[string]string:
		for key, value := range annotations {
			if key == "" {
				continue
			}
			target[key] = value
		}
	case map[string]any:
		for key, value := range annotations {
			if key == "" || value == nil {
				continue
			}
			target[key] = fmt.Sprint(value)
		}
	}
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
		fmt.Fprintf(&b, "  %s: |2\n", key)
		for _, line := range strings.Split(content, "\n") {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}
	return b.String()
}

func allConfigFiles(config WorkloadStageConfigRef) []WorkloadConfigFileRef {
	files := append([]WorkloadConfigFileRef{}, config.ConfigFiles...)
	overrides := containerOverrides(config.ValuesOverride)
	names := make([]string, 0, len(overrides))
	for name := range overrides {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, file := range overrides[name].ConfigFiles {
			file.MountPath = configFileKey(name, file)
			files = append(files, file)
		}
	}
	return files
}

func configFileKey(containerName string, file WorkloadConfigFileRef) string {
	base := sanitizeConfigFileKey(file.MountPath)
	if strings.TrimSpace(containerName) == "" || containerName == "app" {
		return base
	}
	return sanitizeConfigFileKey(containerName + "-" + base)
}

func sanitizeConfigFileKey(mountPath string) string {
	name := filepath.Base(mountPath)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.Trim(name, ".")
	if name == "" {
		name = "config"
	}
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

func writableVolumeName(containerName string, index int) string {
	name := sanitizeConfigFileKey(containerName)
	if name == "" {
		name = "app"
	}
	return fmt.Sprintf("writable-%s-%d", name, index)
}

func rawVolumes(valuesOverride map[string]any) []map[string]any {
	var out []map[string]any
	if valuesOverride == nil {
		return out
	}
	for _, raw := range []any{valuesOverride["volumes"]} {
		out = append(out, rawMapSlice(raw)...)
	}
	if compat, ok := valuesOverride["k8sCompat"].(map[string]any); ok {
		out = append(out, rawMapSlice(compat["volumes"])...)
	}
	return out
}

func rawContainerVolumeMounts(valuesOverride map[string]any, containerName string) []map[string]any {
	return rawContainerList(valuesOverride, containerName, "volumeMounts", "volume_mounts")
}

func rawContainerList(valuesOverride map[string]any, containerName string, keys ...string) []map[string]any {
	var out []map[string]any
	if compat, ok := valuesOverride["k8sCompat"].(map[string]any); ok {
		if containers, ok := compat["containers"].([]any); ok {
			for _, raw := range containers {
				item, ok := raw.(map[string]any)
				if !ok || strings.TrimSpace(fmt.Sprint(item["name"])) != containerName {
					continue
				}
				for _, key := range keys {
					out = append(out, rawMapSlice(item[key])...)
				}
			}
		}
	}
	return out
}

func rawContainerMap(valuesOverride map[string]any, containerName string, key string) map[string]any {
	if compat, ok := valuesOverride["k8sCompat"].(map[string]any); ok {
		if containers, ok := compat["containers"].([]any); ok {
			for _, raw := range containers {
				item, ok := raw.(map[string]any)
				if !ok || strings.TrimSpace(fmt.Sprint(item["name"])) != containerName {
					continue
				}
				if value, ok := item[key].(map[string]any); ok {
					return value
				}
			}
		}
	}
	return nil
}

func rawContainerProbes(valuesOverride map[string]any, containerName string) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, spec := range []struct {
		raw string
		out string
	}{
		{raw: "livenessProbe", out: "livenessProbe"},
		{raw: "readinessProbe", out: "readinessProbe"},
		{raw: "startupProbe", out: "startupProbe"},
		{raw: "liveness_probe", out: "livenessProbe"},
		{raw: "readiness_probe", out: "readinessProbe"},
		{raw: "startup_probe", out: "startupProbe"},
	} {
		if raw := normalizeRawProbe(rawContainerMap(valuesOverride, containerName, spec.raw)); len(raw) > 0 {
			out[spec.out] = raw
		}
	}
	return out
}

func normalizeRawProbe(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	if enabled, ok := raw["enabled"].(bool); ok && !enabled {
		return nil
	}
	out := map[string]any{}
	for _, key := range []string{"httpGet", "tcpSocket", "exec", "grpc"} {
		if value, ok := raw[key]; ok && value != nil {
			out[key] = value
		}
	}
	probeType := strings.ToLower(strings.TrimSpace(firstString(raw, "probeType", "probe_type", "type")))
	if len(out) == 0 {
		switch probeType {
		case "tcp", "tcpsocket", "tcp_socket":
			if port, ok := firstInt(raw, "port", "targetPort", "target_port"); ok {
				out["tcpSocket"] = map[string]any{"port": port}
			}
		case "exec":
			if command, ok := firstStringSlice(raw, "command"); ok {
				out["exec"] = map[string]any{"command": command}
			}
		default:
			path := strings.TrimSpace(firstString(raw, "path"))
			port, ok := firstInt(raw, "port", "targetPort", "target_port")
			if path != "" && ok {
				out["httpGet"] = map[string]any{"path": path, "port": port}
			}
		}
	}
	for _, spec := range []struct {
		out  string
		keys []string
	}{
		{out: "initialDelaySeconds", keys: []string{"initialDelaySeconds", "initial_delay_seconds", "initial-delay-seconds"}},
		{out: "periodSeconds", keys: []string{"periodSeconds", "period_seconds", "period-seconds"}},
		{out: "timeoutSeconds", keys: []string{"timeoutSeconds", "timeout_seconds", "timeout-seconds"}},
		{out: "failureThreshold", keys: []string{"failureThreshold", "failure_threshold", "failure-threshold"}},
		{out: "successThreshold", keys: []string{"successThreshold", "success_threshold", "success-threshold"}},
	} {
		if value, ok := firstInt(raw, spec.keys...); ok {
			out[spec.out] = value
		}
	}
	return out
}

func firstString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key]; ok && value != nil {
			return fmt.Sprint(value)
		}
	}
	return ""
}

func firstInt(raw map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed, true
		case int8:
			return int(typed), true
		case int16:
			return int(typed), true
		case int32:
			return int(typed), true
		case int64:
			return int(typed), true
		case uint:
			return int(typed), true
		case uint8:
			return int(typed), true
		case uint16:
			return int(typed), true
		case uint32:
			return int(typed), true
		case uint64:
			return int(typed), true
		case float64:
			return int(typed), true
		case float32:
			return int(typed), true
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(typed))
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func firstStringSlice(raw map[string]any, keys ...string) ([]string, bool) {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case []string:
			if len(typed) > 0 {
				return typed, true
			}
		case []any:
			out := make([]string, 0, len(typed))
			for _, item := range typed {
				out = append(out, fmt.Sprint(item))
			}
			if len(out) > 0 {
				return out, true
			}
		case string:
			if strings.TrimSpace(typed) != "" {
				return []string{typed}, true
			}
		}
	}
	return nil, false
}

func rawMapSlice(raw any) []map[string]any {
	var out []map[string]any
	switch items := raw.(type) {
	case []map[string]any:
		return items
	case []any:
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

func renderRawListItem(item map[string]any, indent string) string {
	if len(item) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s- ", indent)
	renderRawMapInline(&b, item, indent)
	return b.String()
}

func renderRawMapInline(b *strings.Builder, item map[string]any, indent string) {
	keys := sortedMapKeys(item)
	first := true
	for _, key := range keys {
		value := item[key]
		if value == nil {
			continue
		}
		if first {
			switch typed := value.(type) {
			case map[string]any:
				fmt.Fprintf(b, "%s:\n", yamlKey(key))
				renderRawYAML(b, typed, indent+"    ")
			case []any:
				fmt.Fprintf(b, "%s:\n", yamlKey(key))
				renderRawYAML(b, typed, indent+"    ")
			default:
				fmt.Fprintf(b, "%s: %s\n", yamlKey(key), yamlScalar(typed))
			}
			first = false
			continue
		}
		renderRawYAMLKeyValue(b, key, value, indent+"  ")
	}
}

func renderRawYAMLKeyValue(b *strings.Builder, key string, value any, indent string) {
	switch typed := value.(type) {
	case map[string]any:
		fmt.Fprintf(b, "%s%s:\n", indent, yamlKey(key))
		renderRawYAML(b, typed, indent+"  ")
	case []any:
		fmt.Fprintf(b, "%s%s:\n", indent, yamlKey(key))
		renderRawYAML(b, typed, indent+"  ")
	default:
		fmt.Fprintf(b, "%s%s: %s\n", indent, yamlKey(key), yamlScalar(typed))
	}
}

func renderRawYAML(b *strings.Builder, value any, indent string) {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedMapKeys(typed) {
			renderRawYAMLKeyValue(b, key, typed[key], indent)
		}
	case []any:
		for _, item := range typed {
			switch v := item.(type) {
			case map[string]any:
				b.WriteString(renderRawListItem(v, indent))
			default:
				fmt.Fprintf(b, "%s- %s\n", indent, yamlScalar(v))
			}
		}
	}
}

func sortedMapKeys(item map[string]any) []string {
	keys := make([]string, 0, len(item))
	for key := range item {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func yamlKey(key string) string {
	return strings.ReplaceAll(key, "_", "-")
}

func yamlScalar(value any) string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return "\"\""
		}
		if strings.ContainsAny(v, ":#{}[]&,*?|-<>=!%@`\"'\n\t") || strings.HasPrefix(v, " ") || strings.HasSuffix(v, " ") {
			return fmt.Sprintf("%q", v)
		}
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(v)
	default:
		if reflect.ValueOf(value).IsValid() {
			return fmt.Sprintf("%q", fmt.Sprint(v))
		}
		return "null"
	}
}
