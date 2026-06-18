package paasagent

import (
	"context"
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

func TestKubernetesClientReaderSnapshotCollectsArgoWorkloadsAndEvents(t *testing.T) {
	replicas := int32(3)
	argo := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      "order-dev",
			"namespace": "argocd",
			"labels": map[string]any{
				labelApplicationID: "app_1",
				labelDeploymentID:  "deployment_1",
				labelStageKey:      "dev",
			},
		},
		"status": map[string]any{
			"sync":           map[string]any{"status": "Synced"},
			"health":         map[string]any{"status": "Healthy"},
			"operationState": map[string]any{"phase": "Succeeded", "message": "ok"},
		},
	}}
	client := k8sfake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "order-api", Namespace: "apps", Labels: workloadLabels()}, Spec: appsv1.DeploymentSpec{Replicas: &replicas}, Status: appsv1.DeploymentStatus{ReadyReplicas: 2, UpdatedReplicas: 3, AvailableReplicas: 2}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "order-db", Namespace: "apps", Labels: workloadLabels()}, Spec: appsv1.StatefulSetSpec{Replicas: &replicas}, Status: appsv1.StatefulSetStatus{ReadyReplicas: 3, UpdatedReplicas: 3, AvailableReplicas: 3}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "log-agent", Namespace: "apps", Labels: workloadLabels()}, Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 2, NumberReady: 2, UpdatedNumberScheduled: 2, NumberAvailable: 2}},
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "order-rs", Namespace: "apps", Labels: workloadLabels()}, Spec: appsv1.ReplicaSetSpec{Replicas: &replicas}, Status: appsv1.ReplicaSetStatus{ReadyReplicas: 2, FullyLabeledReplicas: 3, AvailableReplicas: 2}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "order-pod", Namespace: "apps", Labels: workloadLabels()}, Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "order-event", Namespace: "apps"}, Type: "Warning", InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "order-pod"}, Message: "重启", LastTimestamp: metav1.NewTime(time.Date(2026, 5, 30, 16, 0, 0, 0, time.UTC))},
	)
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{argoApplicationGVR: "ApplicationList"}, argo)
	reader := NewKubernetesClientReaderFromClients(client, dynamicClient, "argocd")
	snapshot, err := reader.Snapshot(context.Background(), []string{"apps"})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Applications) != 1 || snapshot.Applications[0].OperationPhase != "Succeeded" || snapshot.Applications[0].DeploymentID != "deployment_1" {
		t.Fatalf("unexpected argo applications: %+v", snapshot.Applications)
	}
	if len(snapshot.Workloads) != 5 {
		t.Fatalf("expected all workload kinds, got %+v", snapshot.Workloads)
	}
	if snapshot.Workloads[0].Desired != 3 || snapshot.Workloads[0].Ready != 2 {
		t.Fatalf("unexpected deployment replicas: %+v", snapshot.Workloads[0])
	}
	if len(snapshot.Events) != 1 || snapshot.Events[0].Resource != "Pod/order-pod" {
		t.Fatalf("unexpected events: %+v", snapshot.Events)
	}
	if err := reader.RefreshArgoApplication(context.Background(), "order-dev"); err != nil {
		t.Fatalf("refresh argo: %v", err)
	}
	if err := reader.SyncArgoApplication(context.Background(), "order-dev"); err != nil {
		t.Fatalf("sync argo: %v", err)
	}
}

func workloadLabels() map[string]string {
	return map[string]string{labelApplicationID: "app_1", labelStageKey: "dev"}
}

func TestKubernetesReaderSnapshotCollectsRuntimeResources(t *testing.T) {
	replicas := int32(2)
	ownerController := true
	client := k8sfake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "order-api", Namespace: "apps", Labels: workloadLabels()}, Spec: appsv1.DeploymentSpec{Replicas: &replicas}, Status: appsv1.DeploymentStatus{ReadyReplicas: 1, UpdatedReplicas: 2, AvailableReplicas: 1}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "order-db", Namespace: "apps", Labels: workloadLabels()}, Spec: appsv1.StatefulSetSpec{Replicas: &replicas}, Status: appsv1.StatefulSetStatus{ReadyReplicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "log-agent", Namespace: "apps", Labels: workloadLabels()}, Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 2, NumberReady: 2, UpdatedNumberScheduled: 2, NumberAvailable: 2}},
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "order-rs", Namespace: "apps", Labels: workloadLabels(), OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "order-api", Controller: &ownerController}}}, Spec: appsv1.ReplicaSetSpec{Replicas: &replicas}, Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1, FullyLabeledReplicas: 2, AvailableReplicas: 1}},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "order-pod", Namespace: "apps", Labels: workloadLabels(), OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "order-rs", Controller: &ownerController}}},
			Status: corev1.PodStatus{
				Phase:      corev1.PodPending,
				Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}},
				ContainerStatuses: []corev1.ContainerStatus{{
					Name: "app", Image: "registry.local/order-api:v1", Ready: false, RestartCount: 3,
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "拉取镜像失败"}},
				}},
			},
		},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "order-event", Namespace: "apps"}, Type: "Warning", Reason: "FailedPull", InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "apps", Name: "order-pod"}, Message: "拉取镜像失败", Count: 2, LastTimestamp: metav1.NewTime(time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC))},
	)
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{argoApplicationGVR: "ApplicationList"})
	reader := NewKubernetesClientReaderFromClients(client, dynamicClient, "argocd")
	snapshot, err := reader.Snapshot(context.Background(), []string{"apps"})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	byKind := map[string]RuntimeResource{}
	for _, resource := range snapshot.RuntimeResources {
		byKind[resource.Kind] = resource
	}
	for _, kind := range []string{"Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Pod", "Event"} {
		if byKind[kind].Kind == "" {
			t.Fatalf("runtime resource kind %s missing, got %+v", kind, snapshot.RuntimeResources)
		}
	}
	if got := byKind["Deployment"]; got.ApplicationID != "app_1" || got.StageKey != "dev" || got.Group != "apps" || got.Ready != 1 || got.Desired != 2 {
		t.Fatalf("deployment runtime resource mismatch: %+v", got)
	}
	if got := byKind["Pod"]; len(got.Containers) != 1 || got.Containers[0].State != "waiting" || got.Containers[0].Message != "ImagePullBackOff: 拉取镜像失败" || got.ParentKind != "ReplicaSet" {
		t.Fatalf("pod runtime resource should include container and parent state: %+v", got)
	}
	if got := byKind["Event"]; got.Status != "Warning" || got.ParentKind != "Pod" || got.ParentName != "order-pod" {
		t.Fatalf("event runtime resource mismatch: %+v", got)
	}
	report := ToStatusReport("cluster_1", snapshot, time.Date(2026, 6, 15, 10, 1, 0, 0, time.UTC))
	if len(report.RuntimeResources) != len(snapshot.RuntimeResources) {
		t.Fatalf("status report should include runtime resources, got %+v", report.RuntimeResources)
	}
}

func TestPodRuntimeResourceMarksDeletingPodTerminating(t *testing.T) {
	deletingAt := metav1.NewTime(time.Date(2026, 6, 16, 16, 58, 0, 0, time.UTC))
	resource := podRuntimeResource(corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "order-pod", Namespace: "apps", Labels: workloadLabels(), DeletionTimestamp: &deletingAt},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
	})
	if resource.Status != "Terminating" || resource.HealthStatus != "Terminating" || resource.Message != "Pod 正在终止" {
		t.Fatalf("deleting pod should be marked terminating, got %+v", resource)
	}
}

func TestRuntimeInvalidationDetectsPodTerminatingTransition(t *testing.T) {
	oldPod := podRuntimeResource(corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "order-pod", Namespace: "apps", Labels: workloadLabels()},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	})
	deletingAt := metav1.NewTime(time.Date(2026, 6, 16, 16, 59, 0, 0, time.UTC))
	newPod := podRuntimeResource(corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "order-pod", Namespace: "apps", Labels: workloadLabels(), DeletionTimestamp: &deletingAt},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	})
	invalidations := runtimeInvalidationsBetween(Snapshot{RuntimeResources: []RuntimeResource{oldPod}}, Snapshot{RuntimeResources: []RuntimeResource{newPod}})
	if _, ok := invalidations[RuntimeInvalidation{ApplicationID: "app_1", StageKey: "dev"}]; !ok {
		t.Fatalf("terminating transition should invalidate app/stage, got %+v", invalidations)
	}
}

func TestKubernetesReaderRuntimeResourcesInheritArgoApplicationOwnership(t *testing.T) {
	replicas := int32(1)
	ownerController := true
	argo := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      "order-dev",
			"namespace": "argocd",
			"labels": map[string]any{
				labelApplicationID: "app_1",
				labelDeploymentID:  "deployment_1",
				labelStageKey:      "dev",
			},
		},
		"status": map[string]any{
			"sync":   map[string]any{"status": "Synced"},
			"health": map[string]any{"status": "Healthy"},
			"resources": []any{
				map[string]any{"group": "apps", "kind": "Deployment", "namespace": "apps", "name": "order-api", "status": "Synced", "health": map[string]any{"status": "Healthy"}},
			},
		},
	}}
	client := k8sfake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "order-api", Namespace: "apps"}, Spec: appsv1.DeploymentSpec{Replicas: &replicas}, Status: appsv1.DeploymentStatus{ReadyReplicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1}},
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "order-rs", Namespace: "apps", OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "order-api", Controller: &ownerController}}}, Spec: appsv1.ReplicaSetSpec{Replicas: &replicas}, Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1, FullyLabeledReplicas: 1, AvailableReplicas: 1}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "order-pod", Namespace: "apps", OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "order-rs", Controller: &ownerController}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "order-event", Namespace: "apps"}, Type: "Normal", Reason: "Started", InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "apps", Name: "order-pod"}, Message: "started", LastTimestamp: metav1.NewTime(time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC))},
	)
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{argoApplicationGVR: "ApplicationList"}, argo)
	reader := NewKubernetesClientReaderFromClients(client, dynamicClient, "argocd")
	snapshot, err := reader.Snapshot(context.Background(), []string{"apps"})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Applications) != 1 || len(snapshot.Applications[0].Resources) != 1 {
		t.Fatalf("argo managed resources not parsed: %+v", snapshot.Applications)
	}
	byKind := map[string]RuntimeResource{}
	for _, resource := range snapshot.RuntimeResources {
		byKind[resource.Kind] = resource
	}
	for _, kind := range []string{"Deployment", "ReplicaSet", "Pod", "Event"} {
		got := byKind[kind]
		if got.ApplicationID != "app_1" || got.StageKey != "dev" {
			t.Fatalf("%s should inherit argo application ownership, got %+v", kind, got)
		}
	}
	if got := byKind["Deployment"]; got.HealthStatus != "Healthy" {
		t.Fatalf("deployment should inherit argo resource health, got %+v", got)
	}
}

func TestKubernetesReaderStageRuntimeResourcesUseArgoTreeAndOwnerRefs(t *testing.T) {
	replicas := int32(1)
	ownerController := true
	argo := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      "order-dev",
			"namespace": "argocd",
			"labels": map[string]any{
				labelApplicationID: "app_1",
				labelDeploymentID:  "deployment_1",
				labelStageKey:      "dev",
			},
		},
		"status": map[string]any{
			"sync":   map[string]any{"status": "Synced"},
			"health": map[string]any{"status": "Healthy"},
			"resources": []any{
				map[string]any{"group": "apps", "kind": "Deployment", "namespace": "apps", "name": "order-api", "status": "Synced", "health": map[string]any{"status": "Healthy"}},
			},
		},
	}}
	client := k8sfake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "order-api", Namespace: "apps"}, Spec: appsv1.DeploymentSpec{Replicas: &replicas}, Status: appsv1.DeploymentStatus{ReadyReplicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1}},
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "order-rs", Namespace: "apps", OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "order-api", Controller: &ownerController}}}, Spec: appsv1.ReplicaSetSpec{Replicas: &replicas}, Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1, FullyLabeledReplicas: 1, AvailableReplicas: 1}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "order-pod", Namespace: "apps", OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "order-rs", Controller: &ownerController}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}},
	)
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{argoApplicationGVR: "ApplicationList"}, argo)
	reader := NewKubernetesClientReaderFromClients(client, dynamicClient, "argocd")

	resources, err := reader.ListRuntimeResources(context.Background(), []string{"apps"}, "app_1", "dev")
	if err != nil {
		t.Fatalf("ListRuntimeResources() error = %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("stage runtime resources should include deployment and pod only, got %+v", resources)
	}
	if resources[0].Kind != "Deployment" || resources[0].ApplicationID != "app_1" || resources[0].StageKey != "dev" {
		t.Fatalf("deployment should inherit argo ownership, got %+v", resources[0])
	}
	if resources[1].Kind != "Pod" || resources[1].ApplicationID != "app_1" || resources[1].StageKey != "dev" || resources[1].ParentKind != "ReplicaSet" {
		t.Fatalf("pod should inherit ownership through ReplicaSet ownerRef, got %+v", resources[1])
	}
}

func TestKubernetesReaderConstructors(t *testing.T) {
	reader, err := NewKubernetesClientReader(&rest.Config{Host: "https://127.0.0.1"}, " argocd ")
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	if reader.argoWatchNamespace() != "argocd" {
		t.Fatalf("namespace not trimmed")
	}
	if reader.restConfig.QPS != 50 || reader.restConfig.Burst != 100 {
		t.Fatalf("unexpected kubernetes client rate limits: qps=%v burst=%d", reader.restConfig.QPS, reader.restConfig.Burst)
	}
	if _, err := NewInClusterKubernetesReader("argocd"); err == nil {
		t.Fatalf("in-cluster reader should fail outside Kubernetes")
	}
	if namespaces := normalizeNamespaces([]string{" ", "apps"}); len(namespaces) != 1 || namespaces[0] != "apps" {
		t.Fatalf("unexpected namespaces: %#v", namespaces)
	}
	if got := int32Value(nil); got != 1 {
		t.Fatalf("nil replicas should default to 1, got %d", got)
	}
}

func TestKubernetesClientReaderWatchReportsChangesAndStops(t *testing.T) {
	client := k8sfake.NewSimpleClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{argoApplicationGVR: "ApplicationList"})
	watchers := make(chan *watch.FakeWatcher, 4)
	client.PrependWatchReactor("*", func(k8stesting.Action) (bool, watch.Interface, error) {
		w := watch.NewFake()
		watchers <- w
		return true, w, nil
	})
	dynamicClient.PrependWatchReactor("*", func(k8stesting.Action) (bool, watch.Interface, error) {
		w := watch.NewFake()
		watchers <- w
		return true, w, nil
	})
	reader := NewKubernetesClientReaderFromClients(client, dynamicClient, "argocd")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changed := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- reader.Watch(ctx, []string{"apps"}, func() { changed <- struct{}{} })
	}()
	var first *watch.FakeWatcher
	for i := 0; i < 4; i++ {
		select {
		case w := <-watchers:
			if first == nil {
				first = w
			}
		case <-time.After(time.Second):
			t.Fatalf("watcher %d was not created", i)
		}
	}
	first.Add(&corev1.Pod{})
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatalf("watch event did not trigger callback")
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("watch error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("watch did not stop")
	}
}

func TestKubernetesReaderSnapshotAndWatchErrorBranches(t *testing.T) {
	errBoom := errors.New("boom")
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{argoApplicationGVR: "ApplicationList"})
	client := k8sfake.NewSimpleClientset()
	client.PrependReactor("list", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errBoom
	})
	reader := NewKubernetesClientReaderFromClients(client, dynamicClient, "")
	if _, err := reader.Snapshot(context.Background(), []string{"apps"}); !errors.Is(err, errBoom) {
		t.Fatalf("deployment list error = %v", err)
	}

	client = k8sfake.NewSimpleClientset()
	client.PrependWatchReactor("deployments", func(k8stesting.Action) (bool, watch.Interface, error) {
		return true, nil, errBoom
	})
	reader = NewKubernetesClientReaderFromClients(client, dynamicClient, "")
	if err := reader.Watch(context.Background(), []string{"apps"}, nil); !errors.Is(err, errBoom) {
		t.Fatalf("deployment watch error = %v", err)
	}

	client = k8sfake.NewSimpleClientset()
	client.PrependWatchReactor("deployments", func(k8stesting.Action) (bool, watch.Interface, error) {
		return true, watch.NewFake(), nil
	})
	client.PrependWatchReactor("pods", func(k8stesting.Action) (bool, watch.Interface, error) {
		return true, nil, errBoom
	})
	reader = NewKubernetesClientReaderFromClients(client, dynamicClient, "")
	if err := reader.Watch(context.Background(), []string{"apps"}, nil); !errors.Is(err, errBoom) {
		t.Fatalf("pod watch error = %v", err)
	}

	client = k8sfake.NewSimpleClientset()
	client.PrependWatchReactor("*", func(action k8stesting.Action) (bool, watch.Interface, error) {
		if action.GetResource().Resource == "events" {
			return true, nil, errBoom
		}
		return true, watch.NewFake(), nil
	})
	reader = NewKubernetesClientReaderFromClients(client, dynamicClient, "")
	if err := reader.Watch(context.Background(), []string{"apps"}, nil); !errors.Is(err, errBoom) {
		t.Fatalf("event watch error = %v", err)
	}

	client = k8sfake.NewSimpleClientset()
	client.PrependWatchReactor("*", func(k8stesting.Action) (bool, watch.Interface, error) {
		return true, watch.NewFake(), nil
	})
	dynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{argoApplicationGVR: "ApplicationList"})
	dynamicClient.PrependWatchReactor("*", func(k8stesting.Action) (bool, watch.Interface, error) {
		return true, nil, errBoom
	})
	reader = NewKubernetesClientReaderFromClients(client, dynamicClient, "")
	if err := reader.Watch(context.Background(), []string{"apps"}, nil); !errors.Is(err, errBoom) {
		t.Fatalf("argo watch error = %v", err)
	}
}

func TestKubernetesEventStatusTimeFallbacks(t *testing.T) {
	eventTime := metav1.MicroTime{Time: time.Date(2026, 5, 30, 17, 0, 0, 0, time.UTC)}
	fromEventTime := eventStatus(corev1.Event{EventTime: eventTime, InvolvedObject: corev1.ObjectReference{Kind: "Deployment", Name: "order"}, Message: "事件"})
	if !fromEventTime.OccurredAt.Equal(eventTime.Time) {
		t.Fatalf("expected event time fallback, got %#v", fromEventTime)
	}
	fromNow := eventStatus(corev1.Event{InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "order"}, Message: "事件"})
	if fromNow.OccurredAt.IsZero() {
		t.Fatalf("expected current time fallback")
	}
}
