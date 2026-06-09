package gitlab

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"

	"github.com/shareinto/paas/internal/modules/gitops"
)

type ManifestRepositoryAdapter struct {
	client    *Client
	projectID string
}

func NewManifestRepositoryAdapter(client *Client, projectID string) *ManifestRepositoryAdapter {
	return &ManifestRepositoryAdapter{client: client, projectID: projectID}
}

func (a *ManifestRepositoryAdapter) ReadFile(ctx context.Context, path string, ref string) (string, error) {
	var out struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	req, err := a.client.newRequest(http.MethodGet, "/api/v4/projects/"+url.PathEscape(a.projectID)+"/repository/files/"+url.PathEscape(path)+"?ref="+url.QueryEscape(ref), nil)
	if err != nil {
		return "", err
	}
	if err := decodeResponseFromClient(ctx, a.client, req, &out); err != nil {
		return "", err
	}
	if out.Encoding == "base64" {
		data, err := base64.StdEncoding.DecodeString(out.Content)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return out.Content, nil
}

func (a *ManifestRepositoryAdapter) CommitFiles(ctx context.Context, spec gitops.CommitSpec) (gitops.CommitResult, error) {
	actions := make([]map[string]string, 0, len(spec.Files))
	for _, file := range spec.Files {
		actions = append(actions, map[string]string{"action": "update", "file_path": file.Path, "content": file.Content})
	}
	var out struct {
		ID string `json:"id"`
	}
	req, err := a.client.newRequest(http.MethodPost, "/api/v4/projects/"+url.PathEscape(a.projectID)+"/repository/commits", map[string]any{"branch": spec.Branch, "commit_message": spec.Message, "actions": actions})
	if err != nil {
		return gitops.CommitResult{}, err
	}
	if err := decodeResponseFromClient(ctx, a.client, req, &out); err != nil {
		return gitops.CommitResult{}, err
	}
	return gitops.CommitResult{CommitSHA: out.ID}, nil
}

func (a *ManifestRepositoryAdapter) CreateMergeRequest(ctx context.Context, spec gitops.MergeRequestSpec) (gitops.MergeRequestResult, error) {
	if _, err := a.CommitFiles(ctx, gitops.CommitSpec{Branch: spec.SourceBranch, Message: spec.Title, Files: spec.Files}); err != nil {
		return gitops.MergeRequestResult{}, err
	}
	var out struct {
		IID    int64  `json:"iid"`
		SHA    string `json:"sha"`
		WebURL string `json:"web_url"`
	}
	req, err := a.client.newRequest(http.MethodPost, "/api/v4/projects/"+url.PathEscape(a.projectID)+"/merge_requests", map[string]any{"source_branch": spec.SourceBranch, "target_branch": spec.TargetBranch, "title": spec.Title})
	if err != nil {
		return gitops.MergeRequestResult{}, err
	}
	if err := decodeResponseFromClient(ctx, a.client, req, &out); err != nil {
		return gitops.MergeRequestResult{}, err
	}
	return gitops.MergeRequestResult{ID: strconvFormat(out.IID), CommitSHA: out.SHA, WebURL: out.WebURL}, nil
}

func (a *ManifestRepositoryAdapter) GetMergeRequest(ctx context.Context, mrID string) (gitops.MergeRequest, error) {
	var out struct {
		IID    int64  `json:"iid"`
		State  string `json:"state"`
		WebURL string `json:"web_url"`
	}
	req, err := a.client.newRequest(http.MethodGet, "/api/v4/projects/"+url.PathEscape(a.projectID)+"/merge_requests/"+url.PathEscape(mrID), nil)
	if err != nil {
		return gitops.MergeRequest{}, err
	}
	if err := decodeResponseFromClient(ctx, a.client, req, &out); err != nil {
		return gitops.MergeRequest{}, err
	}
	return gitops.MergeRequest{ID: strconvFormat(out.IID), State: out.State, WebURL: out.WebURL, Merged: out.State == "merged"}, nil
}

func decodeResponseFromClient(ctx context.Context, client *Client, req *http.Request, target any) error {
	return client.do(req.WithContext(ctx), target)
}

func strconvFormat(v int64) string {
	if v == 0 {
		return ""
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	return string(buf)
}
