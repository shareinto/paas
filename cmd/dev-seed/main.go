package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultAPIBase   = "http://127.0.0.1:8080"
	defaultSourceURL = "http://192.168.100.80/paas/sbg/macc/log-receiver.git"
)

type seedOptions struct {
	APIBase   string
	SourceURL string
	DryRun    bool
	Client    *http.Client
}

func main() {
	var opts seedOptions
	flag.StringVar(&opts.APIBase, "api-base", firstNonEmpty(os.Getenv("PAAS_DEV_SEED_API_BASE"), defaultAPIBase), "PaaS control plane API base URL")
	flag.StringVar(&opts.SourceURL, "source-url", firstNonEmpty(os.Getenv("PAAS_DEV_SEED_SOURCE_URL"), defaultSourceURL), "source HTTP URL for the default build pipeline")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "print planned seed data without calling API or MySQL")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := runSeed(ctx, opts); err != nil {
		fmt.Fprintf(os.Stderr, "dev seed failed: %v\n", err)
		os.Exit(1)
	}
}

func runSeed(ctx context.Context, opts seedOptions) error {
	opts.APIBase = strings.TrimRight(firstNonEmpty(opts.APIBase, defaultAPIBase), "/")
	opts.SourceURL = firstNonEmpty(strings.TrimSpace(opts.SourceURL), defaultSourceURL)
	if opts.DryRun {
		fmt.Printf("dry-run: seed tenant=sbg project=macc application=log-receiver workload=log-receiver pipeline=main api_base=%s source_url=%s\n", opts.APIBase, opts.SourceURL)
		return nil
	}
	client := seedClient{base: opts.APIBase, http: opts.Client}
	if client.http == nil {
		client.http = &http.Client{Timeout: 15 * time.Second}
	}
	actor := map[string]string{"type": "user", "id": "usr_admin"}

	tenantID, err := client.ensureByName(ctx, "GET", "/api/tenants", "POST", "/api/tenants", "sbg", map[string]any{
		"actor":        actor,
		"name":         "sbg",
		"display_name": "sbg",
		"description":  "SBG 测试租户",
	})
	if err != nil {
		return err
	}
	projectID, err := client.ensureByName(ctx, "GET", "/api/projects?tenant_id="+tenantID, "POST", "/api/projects", "macc", map[string]any{
		"actor":        actor,
		"tenant_id":    tenantID,
		"name":         "macc",
		"display_name": "macc",
		"description":  "MACC 测试项目",
	})
	if err != nil {
		return err
	}
	applicationID, err := client.ensureByName(ctx, "GET", "/api/projects/"+projectID+"/applications?page=1&page_size=100", "POST", "/api/applications", "log-receiver", map[string]any{
		"actor":        actor,
		"project_id":   projectID,
		"name":         "log-receiver",
		"display_name": "log-receiver",
		"description":  "日志接收应用",
	})
	if err != nil {
		return err
	}
	workloadID, err := client.ensureByName(ctx, "GET", "/api/applications/"+applicationID+"/workloads", "POST", "/api/applications/"+applicationID+"/workloads", "log-receiver", map[string]any{
		"actor":         actor,
		"name":          "log-receiver",
		"display_name":  "log-receiver",
		"workload_type": "Deployment",
		"description":   "日志接收服务工作负载",
	})
	if err != nil {
		return err
	}
	runtimeID, err := client.selectRuntimeEnvironment(ctx)
	if err != nil {
		return err
	}
	buildEnvironmentID, err := client.selectBuildEnvironment(ctx)
	if err != nil {
		return err
	}
	_, err = client.ensureByName(ctx, "GET", "/api/apps/"+applicationID+"/build-pipelines?page=1&page_size=100", "POST", "/api/apps/"+applicationID+"/build-pipelines", "main", map[string]any{
		"actor":                   actor,
		"workload_id":             workloadID,
		"name":                    "main",
		"display_name":            "main",
		"description":             "默认构建流水线",
		"runtime_environment_ids": []string{runtimeID},
		"sources": []map[string]any{{
			"key":                  "main",
			"display_name":         "主代码源",
			"source_type":          "git",
			"source_url":           opts.SourceURL,
			"source_ref":           "main",
			"build_environment_id": buildEnvironmentID,
			"source_path":          ".",
			"default_ref":          "main",
			"is_primary":           true,
			"build_spec": map[string]any{
				"source_path":           ".",
				"default_ref":           "main",
				"build_command":         "gradle --no-daemon --parallel --max-workers=12 :log-receiver:bootJar -x test",
				"artifact_copy_command": `cp log-receiver/build/libs/log-receiver.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"`,
				"runtime_base_image":    "",
				"artifact_deploy_path":  "",
			},
		}},
	})
	return err
}

type seedClient struct {
	base string
	http *http.Client
}

type httpStatusError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e httpStatusError) Error() string {
	return fmt.Sprintf("%s %s returned %d: %s", e.Method, e.Path, e.StatusCode, strings.TrimSpace(e.Body))
}

func (c seedClient) ensureByName(ctx context.Context, listMethod, listPath, createMethod, createPath, name string, payload map[string]any) (string, error) {
	items, err := c.list(ctx, listMethod, listPath)
	if err != nil {
		return "", err
	}
	if id := findIDByName(items, name); id != "" {
		return id, nil
	}
	var created map[string]any
	if err := c.doJSON(ctx, createMethod, createPath, payload, &created); err != nil {
		return "", err
	}
	id, _ := created["id"].(string)
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("%s %s response missing id", createMethod, createPath)
	}
	return id, nil
}

func (c seedClient) selectRuntimeEnvironment(ctx context.Context) (string, error) {
	items, err := c.list(ctx, "GET", "/api/runtime-environments?page=1&page_size=100")
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if asString(item["id"]) == "runtime_env_java17" && isEnabled(item) {
			return "runtime_env_java17", nil
		}
	}
	for _, item := range items {
		if isEnabled(item) {
			return asString(item["id"]), nil
		}
	}
	return "", fmt.Errorf("no enabled runtime environment found")
}

func (c seedClient) selectBuildEnvironment(ctx context.Context) (string, error) {
	items, err := c.list(ctx, "GET", "/api/build-environments?page=1&page_size=100")
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if asString(item["id"]) == "build_env_gradle7_jdk11" && isEnabled(item) {
			return "build_env_gradle7_jdk11", nil
		}
	}
	for _, item := range items {
		if isEnabled(item) {
			return asString(item["id"]), nil
		}
	}
	return "", fmt.Errorf("no enabled build environment found")
}

func (c seedClient) list(ctx context.Context, method, path string) ([]map[string]any, error) {
	var response struct {
		Items []map[string]any `json:"items"`
	}
	if err := c.doJSON(ctx, method, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c seedClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpStatusError{Method: method, Path: path, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(respBody))}
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode %s %s response: %w", method, path, err)
	}
	return nil
}

func findIDByName(items []map[string]any, name string) string {
	for _, item := range items {
		if asString(item["name"]) == name {
			return asString(item["id"])
		}
	}
	return ""
}

func isEnabled(item map[string]any) bool {
	status := asString(item["status"])
	return status == "" || status == "enabled"
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
