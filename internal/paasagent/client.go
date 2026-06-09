package paasagent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/shared"
)

type HTTPControlPlaneClient struct {
	config Config
	http   *http.Client
}

func NewHTTPControlPlaneClient(config Config) *HTTPControlPlaneClient {
	return &HTTPControlPlaneClient{config: config.Normalize(), http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *HTTPControlPlaneClient) Heartbeat(ctx context.Context, heartbeat clusteragent.ClusterHeartbeat) error {
	return c.post(ctx, "/api/agent/v1/heartbeat", heartbeat)
}

func (c *HTTPControlPlaneClient) ReportStatus(ctx context.Context, report clusteragent.StatusReport) error {
	return c.post(ctx, "/api/agent/v1/status/report", report)
}

func (c *HTTPControlPlaneClient) ReportEvents(ctx context.Context, report clusteragent.StatusReport) error {
	return c.post(ctx, "/api/agent/v1/events/report", report)
}

func (c *HTTPControlPlaneClient) PullTasks(ctx context.Context) ([]Task, error) {
	req, err := c.request(ctx, http.MethodGet, "/api/agent/v1/tasks", nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Items []Task `json:"items"`
	}
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

func (c *HTTPControlPlaneClient) ReportTaskResult(ctx context.Context, taskID string, status string, message string) error {
	return c.post(ctx, "/api/agent/v1/tasks/result", map[string]string{"task_id": taskID, "status": status, "message": message})
}

func (c *HTTPControlPlaneClient) post(ctx context.Context, path string, body any) error {
	req, err := c.request(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *HTTPControlPlaneClient) request(ctx context.Context, method string, path string, body any) (*http.Request, error) {
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.config.ControlPlaneURL, "/")+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.AgentToken)
	req.Header.Set("X-PaaS-Cluster-ID", c.config.ClusterID.String())
	return req, nil
}

func (c *HTTPControlPlaneClient) do(req *http.Request, target any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return shared.NewError(shared.CodeUnavailable, "control plane request failed")
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}
