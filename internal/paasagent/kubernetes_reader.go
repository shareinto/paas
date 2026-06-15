package paasagent

import (
	"context"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/shared"
)

const (
	labelApplicationID = "paas.shareinto.com/application-id"
	labelDeploymentID  = "paas.shareinto.com/deployment-id"
	labelStageKey      = "paas.shareinto.com/stage-key"
)

var argoApplicationGVR = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

type KubernetesClientReader struct {
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
	argoNamespace string
}

func NewKubernetesClientReader(config *rest.Config, argoNamespace string) (*KubernetesClientReader, error) {
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &KubernetesClientReader{client: client, dynamicClient: dynamicClient, argoNamespace: strings.TrimSpace(argoNamespace)}, nil
}

func NewInClusterKubernetesReader(argoNamespace string) (*KubernetesClientReader, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return NewKubernetesClientReader(config, argoNamespace)
}

func NewKubernetesClientReaderFromClients(client kubernetes.Interface, dynamicClient dynamic.Interface, argoNamespace string) *KubernetesClientReader {
	return &KubernetesClientReader{client: client, dynamicClient: dynamicClient, argoNamespace: strings.TrimSpace(argoNamespace)}
}

func (r *KubernetesClientReader) Snapshot(ctx context.Context, namespaces []string) (Snapshot, error) {
	var snapshot Snapshot
	resourceIndex := map[string]RuntimeResource{}
	apps, err := r.listArgoApplications(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.Applications = apps
	argoResources := argoResourceIndex(apps)
	for _, namespace := range normalizeNamespaces(namespaces) {
		deployments, err := r.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Snapshot{}, err
		}
		for _, item := range deployments.Items {
			snapshot.Workloads = append(snapshot.Workloads, deploymentStatus(item))
			resource := deploymentRuntimeResource(item)
			resource = applyRuntimeResourceOwnership(resource, argoResources, resourceIndex)
			snapshot.RuntimeResources = append(snapshot.RuntimeResources, resource)
			resourceIndex[runtimeResourceLookupKey(resource.Kind, resource.Namespace, resource.Name)] = resource
		}
		statefulSets, err := r.client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Snapshot{}, err
		}
		for _, item := range statefulSets.Items {
			snapshot.Workloads = append(snapshot.Workloads, statefulSetStatus(item))
			resource := statefulSetRuntimeResource(item)
			resource = applyRuntimeResourceOwnership(resource, argoResources, resourceIndex)
			snapshot.RuntimeResources = append(snapshot.RuntimeResources, resource)
			resourceIndex[runtimeResourceLookupKey(resource.Kind, resource.Namespace, resource.Name)] = resource
		}
		daemonSets, err := r.client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Snapshot{}, err
		}
		for _, item := range daemonSets.Items {
			snapshot.Workloads = append(snapshot.Workloads, daemonSetStatus(item))
			resource := daemonSetRuntimeResource(item)
			resource = applyRuntimeResourceOwnership(resource, argoResources, resourceIndex)
			snapshot.RuntimeResources = append(snapshot.RuntimeResources, resource)
			resourceIndex[runtimeResourceLookupKey(resource.Kind, resource.Namespace, resource.Name)] = resource
		}
		replicaSets, err := r.client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Snapshot{}, err
		}
		for _, item := range replicaSets.Items {
			snapshot.Workloads = append(snapshot.Workloads, replicaSetStatus(item))
			resource := replicaSetRuntimeResource(item)
			resource = applyRuntimeResourceOwnership(resource, argoResources, resourceIndex)
			snapshot.RuntimeResources = append(snapshot.RuntimeResources, resource)
			resourceIndex[runtimeResourceLookupKey(resource.Kind, resource.Namespace, resource.Name)] = resource
		}
		pods, err := r.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Snapshot{}, err
		}
		for _, item := range pods.Items {
			snapshot.Workloads = append(snapshot.Workloads, podStatus(item))
			resource := podRuntimeResource(item)
			resource = applyRuntimeResourceOwnership(resource, argoResources, resourceIndex)
			snapshot.RuntimeResources = append(snapshot.RuntimeResources, resource)
			resourceIndex[runtimeResourceLookupKey(resource.Kind, resource.Namespace, resource.Name)] = resource
		}
		events, err := r.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Snapshot{}, err
		}
		for _, item := range events.Items {
			snapshot.Events = append(snapshot.Events, eventStatus(item))
			if resource := eventRuntimeResource(item, resourceIndex); resource.ApplicationID != "" && resource.StageKey != "" {
				snapshot.RuntimeResources = append(snapshot.RuntimeResources, resource)
			}
		}
	}
	return snapshot, nil
}

func (r *KubernetesClientReader) Watch(ctx context.Context, namespaces []string, onChange func()) error {
	watchers := make([]interface{ Stop() }, 0)
	for _, namespace := range normalizeNamespaces(namespaces) {
		deployments, err := r.client.AppsV1().Deployments(namespace).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			stopWatchers(watchers)
			return err
		}
		watchers = append(watchers, deployments)
		go notifyOnWatch(ctx, deployments.ResultChan(), onChange)
		pods, err := r.client.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			stopWatchers(watchers)
			return err
		}
		watchers = append(watchers, pods)
		go notifyOnWatch(ctx, pods.ResultChan(), onChange)
		events, err := r.client.CoreV1().Events(namespace).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			stopWatchers(watchers)
			return err
		}
		watchers = append(watchers, events)
		go notifyOnWatch(ctx, events.ResultChan(), onChange)
	}
	argoWatch, err := r.dynamicClient.Resource(argoApplicationGVR).Namespace(r.argoWatchNamespace()).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		stopWatchers(watchers)
		return err
	}
	watchers = append(watchers, argoWatch)
	go notifyOnWatch(ctx, argoWatch.ResultChan(), onChange)
	<-ctx.Done()
	stopWatchers(watchers)
	return ctx.Err()
}

func (r *KubernetesClientReader) RefreshArgoApplication(ctx context.Context, name string) error {
	return r.patchArgoApplication(ctx, name, `{"metadata":{"annotations":{"argocd.argoproj.io/refresh":"normal"}}}`)
}

func (r *KubernetesClientReader) SyncArgoApplication(ctx context.Context, name string) error {
	return r.patchArgoApplication(ctx, name, `{"operation":{"sync":{}}}`)
}

func (r *KubernetesClientReader) RestartRuntimeResource(ctx context.Context, kind string, namespace string, name string) error {
	kind = strings.TrimSpace(kind)
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return shared.NewError(shared.CodeInvalidArgument, "runtime restart target is required")
	}
	patch := []byte(`{"spec":{"template":{"metadata":{"annotations":{"paas.shareinto.com/restarted-at":"` + time.Now().UTC().Format(time.RFC3339Nano) + `"}}}}}`)
	switch kind {
	case "Deployment":
		_, err := r.client.AppsV1().Deployments(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
		return err
	case "StatefulSet":
		_, err := r.client.AppsV1().StatefulSets(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
		return err
	case "DaemonSet":
		_, err := r.client.AppsV1().DaemonSets(namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
		return err
	default:
		return shared.NewError(shared.CodeInvalidArgument, "unsupported runtime restart kind")
	}
}

func (r *KubernetesClientReader) patchArgoApplication(ctx context.Context, name string, patch string) error {
	_, err := r.dynamicClient.Resource(argoApplicationGVR).Namespace(r.argoWatchNamespace()).Patch(ctx, strings.TrimSpace(name), types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	return err
}

func (r *KubernetesClientReader) listArgoApplications(ctx context.Context) ([]ArgoApplication, error) {
	list, err := r.dynamicClient.Resource(argoApplicationGVR).Namespace(r.argoWatchNamespace()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ArgoApplication, 0, len(list.Items))
	for _, item := range list.Items {
		labels := item.GetLabels()
		syncStatus, _, _ := unstructured.NestedString(item.Object, "status", "sync", "status")
		healthStatus, _, _ := unstructured.NestedString(item.Object, "status", "health", "status")
		phase, _, _ := unstructured.NestedString(item.Object, "status", "operationState", "phase")
		message, _, _ := unstructured.NestedString(item.Object, "status", "operationState", "message")
		app := ArgoApplication{
			Name: item.GetName(), ApplicationID: sharedID(labels[labelApplicationID]), StageKey: strings.TrimSpace(labels[labelStageKey]), DeploymentID: sharedID(labels[labelDeploymentID]),
			SyncStatus: syncStatus, HealthStatus: healthStatus, OperationPhase: phase, Message: message,
		}
		resources, _, _ := unstructured.NestedSlice(item.Object, "status", "resources")
		for _, value := range resources {
			resource, ok := value.(map[string]any)
			if !ok {
				continue
			}
			app.Resources = append(app.Resources, ArgoApplicationResource{
				ApplicationID: app.ApplicationID,
				StageKey:      app.StageKey,
				Group:         stringValue(resource["group"]),
				Kind:          stringValue(resource["kind"]),
				Namespace:     stringValue(resource["namespace"]),
				Name:          stringValue(resource["name"]),
				SyncStatus:    stringValue(resource["status"]),
				HealthStatus:  nestedStringValue(resource["health"], "status"),
			})
		}
		out = append(out, app)
	}
	return out, nil
}

func (r *KubernetesClientReader) argoWatchNamespace() string {
	if r.argoNamespace == "" {
		return metav1.NamespaceAll
	}
	return r.argoNamespace
}

func deploymentStatus(item appsv1.Deployment) Workload {
	desired := int32Value(item.Spec.Replicas)
	return workloadFromLabels("Deployment", item.Name, item.Labels, int(desired), int(item.Status.ReadyReplicas), int(item.Status.UpdatedReplicas), int(item.Status.AvailableReplicas))
}

func deploymentRuntimeResource(item appsv1.Deployment) RuntimeResource {
	desired := int(int32Value(item.Spec.Replicas))
	ready := int(item.Status.ReadyReplicas)
	return workloadRuntimeResource("apps", "v1", "Deployment", item.ObjectMeta, desired, ready, statusForReady(desired, ready))
}

func statefulSetStatus(item appsv1.StatefulSet) Workload {
	desired := int32Value(item.Spec.Replicas)
	return workloadFromLabels("StatefulSet", item.Name, item.Labels, int(desired), int(item.Status.ReadyReplicas), int(item.Status.UpdatedReplicas), int(item.Status.AvailableReplicas))
}

func statefulSetRuntimeResource(item appsv1.StatefulSet) RuntimeResource {
	desired := int(int32Value(item.Spec.Replicas))
	ready := int(item.Status.ReadyReplicas)
	return workloadRuntimeResource("apps", "v1", "StatefulSet", item.ObjectMeta, desired, ready, statusForReady(desired, ready))
}

func daemonSetStatus(item appsv1.DaemonSet) Workload {
	return workloadFromLabels("DaemonSet", item.Name, item.Labels, int(item.Status.DesiredNumberScheduled), int(item.Status.NumberReady), int(item.Status.UpdatedNumberScheduled), int(item.Status.NumberAvailable))
}

func daemonSetRuntimeResource(item appsv1.DaemonSet) RuntimeResource {
	desired := int(item.Status.DesiredNumberScheduled)
	ready := int(item.Status.NumberReady)
	return workloadRuntimeResource("apps", "v1", "DaemonSet", item.ObjectMeta, desired, ready, statusForReady(desired, ready))
}

func replicaSetStatus(item appsv1.ReplicaSet) Workload {
	desired := int32Value(item.Spec.Replicas)
	return workloadFromLabels("ReplicaSet", item.Name, item.Labels, int(desired), int(item.Status.ReadyReplicas), int(item.Status.FullyLabeledReplicas), int(item.Status.AvailableReplicas))
}

func replicaSetRuntimeResource(item appsv1.ReplicaSet) RuntimeResource {
	desired := int(int32Value(item.Spec.Replicas))
	ready := int(item.Status.ReadyReplicas)
	return workloadRuntimeResource("apps", "v1", "ReplicaSet", item.ObjectMeta, desired, ready, statusForReady(desired, ready))
}

func podStatus(item corev1.Pod) Workload {
	ready := 0
	for _, condition := range item.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			ready = 1
			break
		}
	}
	return workloadFromLabels("Pod", item.Name, item.Labels, 1, ready, ready, ready)
}

func podRuntimeResource(item corev1.Pod) RuntimeResource {
	ready := 0
	for _, condition := range item.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			ready = 1
			break
		}
	}
	resource := workloadRuntimeResource("", "v1", "Pod", item.ObjectMeta, 1, ready, string(item.Status.Phase))
	for _, container := range item.Status.ContainerStatuses {
		state, message := containerState(container.State)
		resource.Containers = append(resource.Containers, clusteragent.RuntimeContainerStatus{Name: container.Name, Image: container.Image, Ready: container.Ready, RestartCount: int(container.RestartCount), State: state, Message: message})
	}
	return resource
}

func workloadFromLabels(kind string, name string, labels map[string]string, desired int, ready int, updated int, available int) Workload {
	return Workload{Kind: kind, Name: name, ApplicationID: sharedID(labels[labelApplicationID]), StageKey: strings.TrimSpace(labels[labelStageKey]), Desired: desired, Ready: ready, Updated: updated, Available: available}
}

func workloadRuntimeResource(group string, version string, kind string, meta metav1.ObjectMeta, desired int, ready int, status string) RuntimeResource {
	ref := controllerOwner(meta.OwnerReferences)
	return RuntimeResource{
		ApplicationID:   sharedID(meta.Labels[labelApplicationID]),
		StageKey:        strings.TrimSpace(meta.Labels[labelStageKey]),
		Group:           group,
		Version:         version,
		Kind:            kind,
		Namespace:       meta.Namespace,
		Name:            meta.Name,
		ParentKind:      ref.Kind,
		ParentNamespace: meta.Namespace,
		ParentName:      ref.Name,
		Status:          status,
		Desired:         desired,
		Ready:           ready,
	}
}

func eventStatus(item corev1.Event) KubernetesEvent {
	occurredAt := item.LastTimestamp.Time
	if occurredAt.IsZero() {
		occurredAt = item.EventTime.Time
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return KubernetesEvent{Type: item.Type, Resource: item.InvolvedObject.Kind + "/" + item.InvolvedObject.Name, Message: item.Message, OccurredAt: occurredAt}
}

func eventRuntimeResource(item corev1.Event, resources map[string]RuntimeResource) RuntimeResource {
	occurredAt := item.LastTimestamp.Time
	if occurredAt.IsZero() {
		occurredAt = item.EventTime.Time
	}
	parentNamespace := item.InvolvedObject.Namespace
	if parentNamespace == "" {
		parentNamespace = item.Namespace
	}
	parent := resources[runtimeResourceLookupKey(item.InvolvedObject.Kind, parentNamespace, item.InvolvedObject.Name)]
	return RuntimeResource{
		ApplicationID:   parent.ApplicationID,
		StageKey:        parent.StageKey,
		Group:           "",
		Version:         "v1",
		Kind:            "Event",
		Namespace:       item.Namespace,
		Name:            item.Name,
		ParentKind:      item.InvolvedObject.Kind,
		ParentNamespace: parentNamespace,
		ParentName:      item.InvolvedObject.Name,
		Status:          item.Type,
		Message:         item.Message,
		Events:          []clusteragent.RuntimeResourceEvent{{Type: item.Type, Reason: item.Reason, Message: item.Message, Count: int(item.Count), OccurredAt: occurredAt}},
	}
}

func normalizeNamespaces(namespaces []string) []string {
	out := make([]string, 0, len(namespaces))
	for _, namespace := range namespaces {
		namespace = strings.TrimSpace(namespace)
		if namespace != "" {
			out = append(out, namespace)
		}
	}
	if len(out) == 0 {
		return []string{metav1.NamespaceAll}
	}
	return out
}

func int32Value(value *int32) int32 {
	if value == nil {
		return 1
	}
	return *value
}

func sharedID(value string) shared.ID {
	return shared.ID(strings.TrimSpace(value))
}

func statusForReady(desired int, ready int) string {
	if desired <= 0 || ready >= desired {
		return "Healthy"
	}
	return "Progressing"
}

func controllerOwner(refs []metav1.OwnerReference) metav1.OwnerReference {
	for _, ref := range refs {
		if ref.Controller != nil && *ref.Controller {
			return ref
		}
	}
	if len(refs) > 0 {
		return refs[0]
	}
	return metav1.OwnerReference{}
}

func containerState(state corev1.ContainerState) (string, string) {
	if state.Waiting != nil {
		return "waiting", joinReasonMessage(state.Waiting.Reason, state.Waiting.Message)
	}
	if state.Running != nil {
		return "running", ""
	}
	if state.Terminated != nil {
		return "terminated", joinReasonMessage(state.Terminated.Reason, state.Terminated.Message)
	}
	return "unknown", ""
}

func joinReasonMessage(reason string, message string) string {
	reason = strings.TrimSpace(reason)
	message = strings.TrimSpace(message)
	if reason == "" {
		return message
	}
	if message == "" {
		return reason
	}
	return reason + ": " + message
}

func runtimeResourceLookupKey(kind string, namespace string, name string) string {
	return strings.ToLower(strings.TrimSpace(kind)) + "/" + strings.TrimSpace(namespace) + "/" + strings.TrimSpace(name)
}

func argoRuntimeResourceLookupKey(group string, kind string, namespace string, name string) string {
	return strings.TrimSpace(group) + "/" + runtimeResourceLookupKey(kind, namespace, name)
}

func argoResourceIndex(apps []ArgoApplication) map[string]ArgoApplicationResource {
	out := map[string]ArgoApplicationResource{}
	for _, app := range apps {
		for _, resource := range app.Resources {
			if resource.ApplicationID.IsZero() || resource.StageKey == "" || resource.Kind == "" || resource.Name == "" {
				continue
			}
			out[argoRuntimeResourceLookupKey(resource.Group, resource.Kind, resource.Namespace, resource.Name)] = resource
		}
	}
	return out
}

func applyRuntimeResourceOwnership(resource RuntimeResource, argoResources map[string]ArgoApplicationResource, resources map[string]RuntimeResource) RuntimeResource {
	if ref, ok := argoResources[argoRuntimeResourceLookupKey(resource.Group, resource.Kind, resource.Namespace, resource.Name)]; ok {
		resource = inheritArgoResource(resource, ref)
	}
	if (resource.ApplicationID.IsZero() || resource.StageKey == "") && resource.ParentKind != "" && resource.ParentName != "" {
		parent := resources[runtimeResourceLookupKey(resource.ParentKind, resource.ParentNamespace, resource.ParentName)]
		if resource.ApplicationID.IsZero() {
			resource.ApplicationID = parent.ApplicationID
		}
		if resource.StageKey == "" {
			resource.StageKey = parent.StageKey
		}
		if resource.HealthStatus == "" {
			resource.HealthStatus = parent.HealthStatus
		}
	}
	return resource
}

func inheritArgoResource(resource RuntimeResource, ref ArgoApplicationResource) RuntimeResource {
	if resource.ApplicationID.IsZero() {
		resource.ApplicationID = ref.ApplicationID
	}
	if resource.StageKey == "" {
		resource.StageKey = ref.StageKey
	}
	if resource.HealthStatus == "" {
		resource.HealthStatus = ref.HealthStatus
	}
	if resource.Status == "" {
		resource.Status = ref.SyncStatus
	}
	return resource
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func nestedStringValue(value any, key string) string {
	values, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return stringValue(values[key])
}

func stopWatchers(watchers []interface{ Stop() }) {
	for _, watcher := range watchers {
		watcher.Stop()
	}
}

func notifyOnWatch(ctx context.Context, ch <-chan watch.Event, onChange func()) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			if onChange != nil {
				onChange()
			}
		}
	}
}
