package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/shareinto/paas/internal/modules/sourcerepository"
)

type SourceRepositoryAdapter struct {
	client        *Client
	rootGroupPath string
}

func NewSourceRepositoryAdapter(client *Client) *SourceRepositoryAdapter {
	return &SourceRepositoryAdapter{client: client}
}

func NewSourceRepositoryAdapterWithNamespace(client *Client, rootGroupPath string) *SourceRepositoryAdapter {
	return &SourceRepositoryAdapter{client: client, rootGroupPath: normalizePath(rootGroupPath)}
}

func (a *SourceRepositoryAdapter) CreateProject(ctx context.Context, spec sourcerepository.GitProjectSpec) (sourcerepository.GitProject, error) {
	var out gitProjectResponse
	body := map[string]any{"name": spec.RepositoryName, "path": spec.RepositoryName, "default_branch": spec.DefaultBranch}
	var namespaceFullPath string
	if a.rootGroupPath != "" {
		namespace, err := a.ensureRepositoryNamespace(ctx, spec)
		if err != nil {
			return sourcerepository.GitProject{}, err
		}
		body["namespace_id"] = namespace.ID
		namespaceFullPath = namespace.FullPath
	}
	req, err := a.client.newRequest(http.MethodPost, "/api/v4/projects", body)
	if err != nil {
		return sourcerepository.GitProject{}, err
	}
	req = req.WithContext(ctx)
	if err := a.do(req, &out); err != nil {
		if namespaceFullPath != "" && isAlreadyExistsError(err) {
			return a.getProject(ctx, namespaceFullPath+"/"+normalizePath(spec.RepositoryName))
		}
		return sourcerepository.GitProject{}, err
	}
	return out.toDomain(), nil
}

func (a *SourceRepositoryAdapter) ensureRepositoryNamespace(ctx context.Context, spec sourcerepository.GitProjectSpec) (gitGroup, error) {
	root, err := a.ensureGroup(ctx, gitGroupSpec{Name: a.rootGroupPath, Path: a.rootGroupPath, FullPath: a.rootGroupPath})
	if err != nil {
		return gitGroup{}, err
	}
	tenantPath := normalizePath(firstNonEmpty(spec.TenantName, spec.TenantID.String()))
	tenant, err := a.ensureGroup(ctx, gitGroupSpec{Name: tenantPath, Path: tenantPath, FullPath: root.FullPath + "/" + tenantPath, ParentID: root.ID})
	if err != nil {
		return gitGroup{}, err
	}
	projectPath := normalizePath(firstNonEmpty(spec.ProjectName, spec.ProjectID.String()))
	project, err := a.ensureGroup(ctx, gitGroupSpec{Name: projectPath, Path: projectPath, FullPath: tenant.FullPath + "/" + projectPath, ParentID: tenant.ID})
	if err != nil {
		return gitGroup{}, err
	}
	return project, nil
}

type gitProjectResponse struct {
	ID            int64  `json:"id"`
	HTTPURLToRepo string `json:"http_url_to_repo"`
	SSHURLToRepo  string `json:"ssh_url_to_repo"`
	WebURL        string `json:"web_url"`
}

func (p gitProjectResponse) toDomain() sourcerepository.GitProject {
	return sourcerepository.GitProject{ID: fmt.Sprint(p.ID), HTTPURL: firstNonEmpty(p.HTTPURLToRepo, p.WebURL), SSHURL: p.SSHURLToRepo}
}

type gitGroupSpec struct {
	Name     string
	Path     string
	FullPath string
	ParentID int64
}

type gitGroup struct {
	ID       int64  `json:"id"`
	FullPath string `json:"full_path"`
}

func (a *SourceRepositoryAdapter) ensureGroup(ctx context.Context, spec gitGroupSpec) (gitGroup, error) {
	group, err := a.getGroup(ctx, spec.FullPath)
	if err == nil {
		return group, nil
	}
	body := map[string]any{"name": spec.Name, "path": spec.Path}
	if spec.ParentID != 0 {
		body["parent_id"] = spec.ParentID
	}
	req, err := a.client.newRequest(http.MethodPost, "/api/v4/groups", body)
	if err != nil {
		return gitGroup{}, err
	}
	if err := a.do(req.WithContext(ctx), &group); err != nil {
		if group, getErr := a.getGroup(ctx, spec.FullPath); getErr == nil {
			return group, nil
		}
		return gitGroup{}, err
	}
	if group.FullPath == "" {
		group.FullPath = spec.FullPath
	}
	return group, nil
}

func (a *SourceRepositoryAdapter) getGroup(ctx context.Context, fullPath string) (gitGroup, error) {
	var group gitGroup
	req, err := a.client.newRequest(http.MethodGet, "/api/v4/groups/"+url.PathEscape(fullPath), nil)
	if err != nil {
		return gitGroup{}, err
	}
	if err := a.do(req.WithContext(ctx), &group); err != nil {
		return gitGroup{}, err
	}
	if group.FullPath == "" {
		group.FullPath = fullPath
	}
	return group, nil
}

func (a *SourceRepositoryAdapter) getProject(ctx context.Context, fullPath string) (sourcerepository.GitProject, error) {
	var project gitProjectResponse
	req, err := a.client.newRequest(http.MethodGet, "/api/v4/projects/"+url.PathEscape(fullPath), nil)
	if err != nil {
		return sourcerepository.GitProject{}, err
	}
	if err := a.do(req.WithContext(ctx), &project); err != nil {
		return sourcerepository.GitProject{}, err
	}
	return project.toDomain(), nil
}

func (a *SourceRepositoryAdapter) InitializeRepository(context.Context, string, string) error {
	return nil
}

func (a *SourceRepositoryAdapter) DeleteProject(ctx context.Context, gitProjectID string) error {
	req, err := a.client.newRequest(http.MethodDelete, "/api/v4/projects/"+url.PathEscape(gitProjectID), nil)
	if err != nil {
		return err
	}
	return a.do(req.WithContext(ctx), nil)
}

func (a *SourceRepositoryAdapter) ProtectBranch(ctx context.Context, gitProjectID string, branch string) error {
	req, err := a.client.newRequest(http.MethodPost, "/api/v4/projects/"+url.PathEscape(gitProjectID)+"/protected_branches", map[string]any{"name": branch})
	if err != nil {
		return err
	}
	if err := a.do(req.WithContext(ctx), nil); err != nil {
		if isAlreadyExistsError(err) {
			return nil
		}
		return err
	}
	return nil
}

func (a *SourceRepositoryAdapter) ConfigureWebhook(ctx context.Context, gitProjectID string, callbackURL string) error {
	req, err := a.client.newRequest(http.MethodPost, "/api/v4/projects/"+url.PathEscape(gitProjectID)+"/hooks", map[string]any{"url": callbackURL, "push_events": true, "merge_requests_events": true})
	if err != nil {
		return err
	}
	return a.do(req.WithContext(ctx), nil)
}

func (a *SourceRepositoryAdapter) SyncMembers(ctx context.Context, gitProjectID string, members []sourcerepository.GitMemberAccess) error {
	for _, member := range members {
		req, err := a.client.newRequest(http.MethodPost, "/api/v4/projects/"+url.PathEscape(gitProjectID)+"/members", map[string]any{"user_id": member.UserID.String(), "access_level": gitAccessLevel(member.Access)})
		if err != nil {
			return err
		}
		if err := a.do(req.WithContext(ctx), nil); err != nil {
			return err
		}
	}
	return nil
}

func (a *SourceRepositoryAdapter) MirrorRepository(ctx context.Context, spec sourcerepository.GitMirrorSpec) error {
	req, err := a.client.newRequest(http.MethodPost, "/api/v4/projects/"+url.PathEscape(spec.GitProjectID)+"/remote_mirrors", map[string]any{"url": spec.SourceURL, "enabled": true})
	if err != nil {
		return err
	}
	return a.do(req.WithContext(ctx), nil)
}

func (a *SourceRepositoryAdapter) VerifyRepository(ctx context.Context, gitProjectID string) error {
	req, err := a.client.newRequest(http.MethodGet, "/api/v4/projects/"+url.PathEscape(gitProjectID), nil)
	if err != nil {
		return err
	}
	return a.do(req.WithContext(ctx), nil)
}

func (a *SourceRepositoryAdapter) ListFiles(ctx context.Context, gitProjectID string, ref string) ([]sourcerepository.RepositoryFile, error) {
	var out []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}
	req, err := a.client.newRequest(http.MethodGet, "/api/v4/projects/"+url.PathEscape(gitProjectID)+"/repository/tree?recursive=true&ref="+url.QueryEscape(ref), nil)
	if err != nil {
		return nil, err
	}
	if err := a.do(req.WithContext(ctx), &out); err != nil {
		return nil, err
	}
	files := make([]sourcerepository.RepositoryFile, 0, len(out))
	for _, item := range out {
		files = append(files, sourcerepository.RepositoryFile{Path: item.Path, Type: item.Type})
	}
	return files, nil
}

func (a *SourceRepositoryAdapter) ListTree(ctx context.Context, gitProjectID string, ref string, treePath string) ([]sourcerepository.RepositoryTreeItem, error) {
	var out []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	}
	query := "/api/v4/projects/" + url.PathEscape(gitProjectID) + "/repository/tree?recursive=false&per_page=100&ref=" + url.QueryEscape(ref)
	if strings.TrimSpace(treePath) != "" {
		query += "&path=" + url.QueryEscape(treePath)
	}
	req, err := a.client.newRequest(http.MethodGet, query, nil)
	if err != nil {
		return nil, err
	}
	if err := a.do(req.WithContext(ctx), &out); err != nil {
		return nil, err
	}
	items := make([]sourcerepository.RepositoryTreeItem, 0, len(out))
	for _, item := range out {
		items = append(items, sourcerepository.RepositoryTreeItem{Name: item.Name, Path: item.Path, Type: item.Type})
	}
	return items, nil
}

func (a *SourceRepositoryAdapter) ListBranches(ctx context.Context, gitProjectID string) ([]sourcerepository.RepositoryBranch, error) {
	var out []struct {
		Name    string `json:"name"`
		Default bool   `json:"default"`
	}
	req, err := a.client.newRequest(http.MethodGet, "/api/v4/projects/"+url.PathEscape(gitProjectID)+"/repository/branches?per_page=100", nil)
	if err != nil {
		return nil, err
	}
	if err := a.do(req.WithContext(ctx), &out); err != nil {
		return nil, err
	}
	branches := make([]sourcerepository.RepositoryBranch, 0, len(out))
	for _, item := range out {
		branches = append(branches, sourcerepository.RepositoryBranch{Name: item.Name, Default: item.Default})
	}
	return branches, nil
}

func (a *SourceRepositoryAdapter) do(req *http.Request, target any) error {
	return a.client.do(req, target)
}

func gitAccessLevel(level sourcerepository.GitAccessLevel) int {
	switch level {
	case sourcerepository.GitAccessOwner:
		return 50
	case sourcerepository.GitAccessMaintainer:
		return 40
	case sourcerepository.GitAccessDeveloper:
		return 30
	default:
		return 20
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizePath(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if r == '-' && !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
