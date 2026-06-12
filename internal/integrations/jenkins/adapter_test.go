package jenkins

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/shareinto/paas/internal/modules/build"
	"github.com/shareinto/paas/internal/shared"
)

func TestAdapterTriggerQueueLogAndCancel(t *testing.T) {
	var canceledQueueID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.EscapedPath()
		switch {
		case r.Method == http.MethodPost && path == "/createItem":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && path == "/job/paas/createItem":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && path == "/job/paas/job/order/buildWithParameters":
			if r.FormValue("BUILD_STRATEGY") != "java_springboot" {
				t.Fatalf("missing Java BuildSpec parameter")
			}
			w.Header().Set("Location", serverURL(r)+"/queue/item/1")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && path == "/queue/item/1/api/json":
			_ = json.NewEncoder(w).Encode(map[string]any{"executable": map[string]any{"number": 9}})
		case r.Method == http.MethodGet && path == "/job/paas/job/order/9/api/json":
			_ = json.NewEncoder(w).Encode(map[string]any{"number": 9, "building": false, "result": "FAILURE"})
		case r.Method == http.MethodGet && path == "/job/paas/job/order/9/logText/progressiveText":
			w.Header().Set("X-Text-Size", "42")
			w.Header().Set("X-More-Data", "false")
			_, _ = w.Write([]byte("token=secret build ok"))
		case r.Method == http.MethodPost && path == "/job/paas/job/order/9/stop":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && path == "/queue/cancelItem":
			canceledQueueID = r.URL.Query().Get("id")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && path == "/job/paas/job/order/doDelete":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	adapter := NewAdapter(NewClient(Config{BaseURL: server.URL, Username: "jenkins", Token: "secret"}))
	if err := adapter.EnsureJob(context.Background(), build.BuildJobSpec{JobName: "paas/order", TemplateID: "java"}); err != nil {
		t.Fatalf("ensure job: %v", err)
	}
	item, err := adapter.TriggerBuild(context.Background(), "paas/order", map[string]string{"BUILD_STRATEGY": "java_springboot"})
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	item, err = adapter.GetQueueItem(context.Background(), item.QueueID)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	status, err := adapter.GetBuildStatus(context.Background(), "paas/order", item.BuildNumber)
	if err != nil || status.Status != build.BuildRunFailed {
		t.Fatalf("status: %#v err=%v", status, err)
	}
	logs, err := adapter.ProgressiveText(context.Background(), "paas/order", item.BuildNumber, 0)
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if logs.Text != "token=[REDACTED] build ok" || logs.NextOffset != 42 {
		t.Fatalf("unexpected logs: %#v", logs)
	}
	if err := adapter.CancelBuild(context.Background(), "paas/order", item.BuildNumber); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := adapter.CancelQueueItem(context.Background(), server.URL+"/queue/item/1"); err != nil {
		t.Fatalf("cancel queue: %v", err)
	}
	if canceledQueueID != "1" {
		t.Fatalf("queue cancel should send parsed queue id, got %q", canceledQueueID)
	}
	if err := adapter.DeleteJob(context.Background(), "paas/order"); err != nil {
		t.Fatalf("delete job: %v", err)
	}
}

func TestJenkinsAdapterRetriesUnavailableStatus(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if _, _, ok := r.BasicAuth(); !ok {
			t.Fatalf("missing basic auth")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	adapter := NewAdapter(NewClient(Config{BaseURL: server.URL, Username: "jenkins", Token: "secret", RetryMax: 1}))
	if err := adapter.EnsureJob(context.Background(), build.BuildJobSpec{JobName: "paas/retry", TemplateID: "java"}); err != nil {
		t.Fatalf("ensure job with retry: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected retry, calls=%d", calls)
	}
}

func TestJenkinsAdapterEnsureJobUpdatesExistingJob(t *testing.T) {
	var updated bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/createItem", "/job/paas/createItem", "/job/paas/job/order/createItem":
			w.WriteHeader(http.StatusConflict)
		case "/job/paas/job/order/job/api/config.xml":
			updated = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	adapter := NewAdapter(NewClient(Config{BaseURL: server.URL, RetryMax: 1}))
	if err := adapter.EnsureJob(context.Background(), build.BuildJobSpec{JobName: "paas/order/api", TemplateID: "java"}); err != nil {
		t.Fatalf("existing Jenkins job should be updated: %v", err)
	}
	if !updated {
		t.Fatalf("expected existing Jenkins job config to be updated")
	}
}

func TestJenkinsFakeAdapterFollowsPortContract(t *testing.T) {
	fake := &FakeAdapter{Logs: map[int64]build.ProgressiveText{1: {Text: "ok", NextOffset: 2}}}
	if err := fake.EnsureJob(context.Background(), build.BuildJobSpec{JobName: "paas/order", TemplateID: "java"}); err != nil {
		t.Fatalf("fake ensure: %v", err)
	}
	if err := fake.DeleteJob(context.Background(), "paas/order"); err != nil {
		t.Fatalf("fake delete: %v", err)
	}
	queue, err := fake.TriggerBuild(context.Background(), "paas/order", map[string]string{"BUILD_STRATEGY": "java_tomcat"})
	if err != nil || queue.QueueID != "queue_fake" {
		t.Fatalf("fake trigger: %#v err=%v", queue, err)
	}
	started, err := fake.GetQueueItem(context.Background(), queue.QueueID)
	if err != nil || !started.Started || started.BuildNumber != 1 {
		t.Fatalf("fake queue: %#v err=%v", started, err)
	}
	status, err := fake.GetBuildStatus(context.Background(), "paas/order", 1)
	if err != nil || status.Status != build.BuildRunRunning {
		t.Fatalf("fake status: %#v err=%v", status, err)
	}
	logs, err := fake.ProgressiveText(context.Background(), "paas/order", 1, 0)
	if err != nil || logs.Text != "ok" {
		t.Fatalf("fake logs: %#v err=%v", logs, err)
	}
	if err := fake.CancelBuild(context.Background(), "paas/order", 1); err != nil {
		t.Fatalf("fake cancel: %v", err)
	}
	if err := fake.CancelQueueItem(context.Background(), "queue_fake"); err != nil {
		t.Fatalf("fake queue cancel: %v", err)
	}
	if len(fake.Jobs) != 1 || len(fake.DeletedJobs) != 1 || len(fake.Triggers) != 1 || fake.CancelCalls != 1 || fake.CancelQueueCalls != 1 {
		t.Fatalf("fake calls not recorded: %#v", fake)
	}
}

func TestJenkinsStatusMappingAndRedaction(t *testing.T) {
	cases := map[int]shared.ErrorCode{
		http.StatusUnauthorized:        shared.CodeUnauthenticated,
		http.StatusForbidden:           shared.CodePermissionDenied,
		http.StatusNotFound:            shared.CodeNotFound,
		http.StatusConflict:            shared.CodeConflict,
		http.StatusTooManyRequests:     shared.CodeUnavailable,
		http.StatusInternalServerError: shared.CodeInternal,
	}
	for status, want := range cases {
		if got := mapStatus(status); got != want {
			t.Fatalf("mapStatus(%d) = %s want %s", status, got, want)
		}
	}
	for _, value := range []string{"password=abc", "secret=def", "token=ghi"} {
		redacted := redact(value)
		if redacted == value {
			t.Fatalf("expected %q to be redacted", value)
		}
	}
}

func TestJenkinsCloneRequestAndHTTPErrorMapping(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://jenkins.example/job", io.NopCloser(bytes.NewBufferString("body")))
	cloned, err := cloneRequest(req)
	if err != nil {
		t.Fatalf("clone request: %v", err)
	}
	data, _ := io.ReadAll(cloned.Body)
	if string(data) != "body" || req.GetBody == nil {
		t.Fatalf("body was not replayable")
	}
	clonedAgain, err := cloneRequest(req)
	if err != nil {
		t.Fatalf("clone replayable request: %v", err)
	}
	data, _ = io.ReadAll(clonedAgain.Body)
	if string(data) != "body" {
		t.Fatalf("replayed body = %q", data)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, RetryMax: 1})
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/missing", nil)
	if _, err := client.do(request, nil); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("not found should map not found, got %v", err)
	}
	existingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Error", "A job already exists with the name paas")
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer existingServer.Close()
	existingClient := NewClient(Config{BaseURL: existingServer.URL, RetryMax: 0})
	existingReq, _ := http.NewRequest(http.MethodPost, existingServer.URL+"/createItem", nil)
	if _, err := existingClient.do(existingReq, nil); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("existing item bad request should map conflict, got %v", err)
	}
}

func TestJenkinsAdapterReturnsRequestConstructionErrors(t *testing.T) {
	adapter := NewAdapter(NewClient(Config{BaseURL: "http://[::1"}))
	if err := adapter.EnsureJob(context.Background(), build.BuildJobSpec{}); err == nil {
		t.Fatalf("ensure should fail on invalid base URL")
	}
	if _, err := adapter.TriggerBuild(context.Background(), "paas/order", nil); err == nil {
		t.Fatalf("trigger should fail on invalid base URL")
	}
	if _, err := adapter.GetQueueItem(context.Background(), "http://[::1"); err == nil {
		t.Fatalf("queue should fail on invalid URL")
	}
	if _, err := adapter.ProgressiveText(context.Background(), "paas/order", 1, 0); err == nil {
		t.Fatalf("logs should fail on invalid base URL")
	}
	if _, err := adapter.GetBuildStatus(context.Background(), "paas/order", 1); err == nil {
		t.Fatalf("status should fail on invalid base URL")
	}
	if err := adapter.CancelBuild(context.Background(), "paas/order", 1); err == nil {
		t.Fatalf("cancel should fail on invalid base URL")
	}
	if err := adapter.CancelQueueItem(context.Background(), "http://[::1"); err == nil {
		t.Fatalf("queue cancel should fail on invalid queue URL")
	}
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}
