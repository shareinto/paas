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

	"github.com/shareinto/paas/internal/shared"
)

const (
	labelApplicationID = "paas.shareinto.com/application-id"
	labelEnvironmentID = "paas.shareinto.com/environment-id"
	labelDeploymentID  = "paas.shareinto.com/deployment-id"
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
	apps, err := r.listArgoApplications(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.Applications = apps
	for _, namespace := range normalizeNamespaces(namespaces) {
		deployments, err := r.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Snapshot{}, err
		}
		for _, item := range deployments.Items {
			snapshot.Workloads = append(snapshot.Workloads, deploymentStatus(item))
		}
		statefulSets, err := r.client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Snapshot{}, err
		}
		for _, item := range statefulSets.Items {
			snapshot.Workloads = append(snapshot.Workloads, statefulSetStatus(item))
		}
		daemonSets, err := r.client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Snapshot{}, err
		}
		for _, item := range daemonSets.Items {
			snapshot.Workloads = append(snapshot.Workloads, daemonSetStatus(item))
		}
		replicaSets, err := r.client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Snapshot{}, err
		}
		for _, item := range replicaSets.Items {
			snapshot.Workloads = append(snapshot.Workloads, replicaSetStatus(item))
		}
		pods, err := r.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Snapshot{}, err
		}
		for _, item := range pods.Items {
			snapshot.Workloads = append(snapshot.Workloads, podStatus(item))
		}
		events, err := r.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return Snapshot{}, err
		}
		for _, item := range events.Items {
			snapshot.Events = append(snapshot.Events, eventStatus(item))
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
		out = append(out, ArgoApplication{
			Name: item.GetName(), ApplicationID: sharedID(labels[labelApplicationID]), EnvironmentID: sharedID(labels[labelEnvironmentID]), DeploymentID: sharedID(labels[labelDeploymentID]),
			SyncStatus: syncStatus, HealthStatus: healthStatus, OperationPhase: phase, Message: message,
		})
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

func statefulSetStatus(item appsv1.StatefulSet) Workload {
	desired := int32Value(item.Spec.Replicas)
	return workloadFromLabels("StatefulSet", item.Name, item.Labels, int(desired), int(item.Status.ReadyReplicas), int(item.Status.UpdatedReplicas), int(item.Status.AvailableReplicas))
}

func daemonSetStatus(item appsv1.DaemonSet) Workload {
	return workloadFromLabels("DaemonSet", item.Name, item.Labels, int(item.Status.DesiredNumberScheduled), int(item.Status.NumberReady), int(item.Status.UpdatedNumberScheduled), int(item.Status.NumberAvailable))
}

func replicaSetStatus(item appsv1.ReplicaSet) Workload {
	desired := int32Value(item.Spec.Replicas)
	return workloadFromLabels("ReplicaSet", item.Name, item.Labels, int(desired), int(item.Status.ReadyReplicas), int(item.Status.FullyLabeledReplicas), int(item.Status.AvailableReplicas))
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

func workloadFromLabels(kind string, name string, labels map[string]string, desired int, ready int, updated int, available int) Workload {
	return Workload{Kind: kind, Name: name, ApplicationID: sharedID(labels[labelApplicationID]), EnvironmentID: sharedID(labels[labelEnvironmentID]), Desired: desired, Ready: ready, Updated: updated, Available: available}
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
