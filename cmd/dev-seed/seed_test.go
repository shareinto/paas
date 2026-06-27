package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSeedCreatesDevDataInOrderAndUsesSourceURLInPipeline(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/tenants":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		case "POST /api/tenants":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "tenant_sbg", "name": "sbg"})
		case "GET /api/projects":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		case "POST /api/projects":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "project_macc", "tenant_id": "tenant_sbg", "name": "macc"})
		case "GET /api/projects/project_macc/applications":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		case "POST /api/applications":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "app_log_receiver", "name": "log-receiver"})
		case "GET /api/applications/app_log_receiver/workloads":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
		case "POST /api/applications/app_log_receiver/workloads":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "workload_log_receiver", "name": "log-receiver"})
		case "GET /api/runtime-environments":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{
				map[string]any{"id": "runtime_env_node", "name": "node", "status": "enabled"},
				map[string]any{"id": "runtime_env_java17", "name": "java17", "status": "enabled"},
			}, "total": 2})
		case "GET /api/build-environments":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{
				map[string]any{"id": "build_env_gradle7_jdk11", "name": "gradle7-jdk11", "status": "enabled"},
			}, "total": 1})
		case "GET /api/apps/app_log_receiver/build-pipelines":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		case "POST /api/apps/app_log_receiver/build-pipelines":
			var payload struct {
				Sources []map[string]any `json:"sources"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode build pipeline payload: %v", err)
			}
			if len(payload.Sources) != 1 || payload.Sources[0]["source_type"] != "git" || payload.Sources[0]["source_url"] != defaultSourceURL || payload.Sources[0]["source_ref"] != "main" {
				t.Fatalf("build pipeline sources = %#v", payload.Sources)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "pipeline_main", "name": "main"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	if err := runSeed(context.Background(), seedOptions{APIBase: server.URL, SourceURL: defaultSourceURL}); err != nil {
		t.Fatalf("runSeed() error = %v", err)
	}

	want := []string{
		"GET /api/tenants",
		"POST /api/tenants",
		"GET /api/projects",
		"POST /api/projects",
		"GET /api/projects/project_macc/applications",
		"POST /api/applications",
		"GET /api/applications/app_log_receiver/workloads",
		"POST /api/applications/app_log_receiver/workloads",
		"GET /api/runtime-environments",
		"GET /api/build-environments",
		"GET /api/apps/app_log_receiver/build-pipelines",
		"POST /api/apps/app_log_receiver/build-pipelines",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("request order mismatch\nwant: %#v\n got: %#v", want, calls)
	}
}

func TestSeedReturnsBuildPipelineCreateFailure(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/tenants":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "tenant_sbg", "name": "sbg"}}, "total": 1})
		case "GET /api/projects":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "project_macc", "tenant_id": "tenant_sbg", "name": "macc"}}, "total": 1})
		case "GET /api/projects/project_macc/applications":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "app_log_receiver", "name": "log-receiver"}}, "total": 1})
		case "GET /api/applications/app_log_receiver/workloads":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "workload_log_receiver", "name": "log-receiver", "status": "enabled"}}})
		case "GET /api/runtime-environments":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "runtime_env_java17", "name": "java17", "status": "enabled"}}, "total": 1})
		case "GET /api/build-environments":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "build_env_gradle7_jdk11", "name": "gradle7-jdk11", "status": "enabled"}}, "total": 1})
		case "GET /api/apps/app_log_receiver/build-pipelines":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		case "POST /api/apps/app_log_receiver/build-pipelines":
			w.WriteHeader(http.StatusPreconditionFailed)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "failed_precondition", "message": "请求处理失败"}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	if err := runSeed(context.Background(), seedOptions{APIBase: server.URL, SourceURL: defaultSourceURL}); err == nil {
		t.Fatalf("runSeed() should return build pipeline create error")
	}

	want := []string{
		"GET /api/tenants",
		"GET /api/projects",
		"GET /api/projects/project_macc/applications",
		"GET /api/applications/app_log_receiver/workloads",
		"GET /api/runtime-environments",
		"GET /api/build-environments",
		"GET /api/apps/app_log_receiver/build-pipelines",
		"POST /api/apps/app_log_receiver/build-pipelines",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("request order mismatch\nwant: %#v\n got: %#v", want, calls)
	}
}

func TestSeedSkipsExistingDevData(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/tenants":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "tenant_sbg", "name": "sbg"}}, "total": 1})
		case "GET /api/projects":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "project_macc", "tenantId": "tenant_sbg", "name": "macc"}}, "total": 1})
		case "GET /api/projects/project_macc/applications":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "app_log_receiver", "name": "log-receiver"}}, "total": 1})
		case "GET /api/applications/app_log_receiver/workloads":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "workload_log_receiver", "name": "log-receiver", "status": "enabled"}}})
		case "GET /api/runtime-environments":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "runtime_env_springboot_jdk11_aliyun", "name": "springboot-jdk11-aliyun", "status": "enabled"}}, "total": 1})
		case "GET /api/build-environments":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "build_env_gradle7_jdk11", "name": "gradle7-jdk11", "status": "enabled"}}, "total": 1})
		case "GET /api/apps/app_log_receiver/build-pipelines":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "pipeline_main", "name": "main"}}, "total": 1})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	if err := runSeed(context.Background(), seedOptions{APIBase: server.URL, SourceURL: defaultSourceURL}); err != nil {
		t.Fatalf("runSeed() error = %v", err)
	}
	for _, call := range calls {
		if len(call) >= 4 && call[:4] == "POST" {
			t.Fatalf("existing seed data should not be recreated, calls=%#v", calls)
		}
	}
}
