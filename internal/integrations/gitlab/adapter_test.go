package gitlab

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/shareinto/paas/internal/modules/gitops"
	"github.com/shareinto/paas/internal/modules/sourcerepository"
	"github.com/shareinto/paas/internal/shared"
)

func TestSourceRepositoryAdapterCreateProjectAndListFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12, "http_url_to_repo": "https://gitlab.example/order.git", "ssh_url_to_repo": "git@gitlab.example:order.git"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/12/repository/branches":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"name": "main", "default": true}, {"name": "feature/order", "default": false}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/12/repository/tree":
			if r.URL.Query().Get("recursive") == "false" {
				if r.URL.Query().Get("path") != "services" {
					t.Fatalf("unexpected tree path query: %s", r.URL.RawQuery)
				}
				_ = json.NewEncoder(w).Encode([]map[string]string{{"name": "order-api", "path": "services/order-api", "type": "tree"}})
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]string{{"name": "pom.xml", "path": "pom.xml", "type": "blob"}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	adapter := NewSourceRepositoryAdapter(NewClient(Config{BaseURL: server.URL, Token: "secret"}))
	project, err := adapter.CreateProject(context.Background(), sourcerepository.GitProjectSpec{RepositoryName: "order", DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if project.ID != "12" || project.HTTPURL == "" {
		t.Fatalf("unexpected project: %#v", project)
	}
	files, err := adapter.ListFiles(context.Background(), "12", "main")
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	if len(files) != 1 || files[0].Path != "pom.xml" {
		t.Fatalf("unexpected files: %#v", files)
	}
	tree, err := adapter.ListTree(context.Background(), "12", "main", "services")
	if err != nil {
		t.Fatalf("list tree: %v", err)
	}
	if len(tree) != 1 || tree[0].Path != "services/order-api" || tree[0].Type != "tree" {
		t.Fatalf("unexpected tree: %#v", tree)
	}
	branches, err := adapter.ListBranches(context.Background(), "12")
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(branches) != 2 || branches[0].Name != "main" || !branches[0].Default {
		t.Fatalf("unexpected branches: %#v", branches)
	}
}

func TestSourceRepositoryAdapterCreatesProjectInEnsuredNamespace(t *testing.T) {
	seenNamespaceID := float64(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/groups/paas-root":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 10, "full_path": "paas-root"})
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/groups/paas-root%2Frnd":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/groups":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			switch body["path"] {
			case "rnd":
				if body["parent_id"] != float64(10) {
					t.Fatalf("unexpected tenant group body: %#v", body)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"id": 11, "full_path": "paas-root/rnd"})
			case "order":
				if body["parent_id"] != float64(11) {
					t.Fatalf("unexpected project group body: %#v", body)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"id": 12, "full_path": "paas-root/rnd/order"})
			default:
				t.Fatalf("unexpected group body: %#v", body)
			}
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/groups/paas-root%2Frnd%2Forder":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			seenNamespaceID = body["namespace_id"].(float64)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 30, "http_url_to_repo": "https://gitlab.example/order-api.git"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	adapter := NewSourceRepositoryAdapterWithNamespace(NewClient(Config{BaseURL: server.URL, Token: "secret"}), "paas-root")
	project, err := adapter.CreateProject(context.Background(), sourcerepository.GitProjectSpec{TenantName: "rnd", ProjectName: "order", RepositoryName: "order-api", DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if project.ID != "30" || seenNamespaceID != 12 {
		t.Fatalf("unexpected project=%#v namespace=%v", project, seenNamespaceID)
	}
}

func TestSourceRepositoryAdapterAdoptsExistingProjectInExpectedNamespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/groups/paas-root":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 10, "full_path": "paas-root"})
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/groups/paas-root%2Frnd":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 11, "full_path": "paas-root/rnd"})
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/groups/paas-root%2Frnd%2Forder":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12, "full_path": "paas-root/rnd/order"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]any{"project_namespace.name": []string{"has already been taken"}, "name": []string{"has already been taken"}, "path": []string{"has already been taken"}}})
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/api/v4/projects/paas-root%2Frnd%2Forder%2Forder-api":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 30, "http_url_to_repo": "https://gitlab.example/paas-root/rnd/order/order-api.git", "ssh_url_to_repo": "git@gitlab.example:paas-root/rnd/order/order-api.git"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	adapter := NewSourceRepositoryAdapterWithNamespace(NewClient(Config{BaseURL: server.URL, Token: "secret"}), "paas-root")
	project, err := adapter.CreateProject(context.Background(), sourcerepository.GitProjectSpec{TenantName: "rnd", ProjectName: "order", RepositoryName: "order-api", DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("create project should adopt existing project: %v", err)
	}
	if project.ID != "30" || project.HTTPURL == "" || project.SSHURL == "" {
		t.Fatalf("unexpected adopted project: %#v", project)
	}
}

func TestSourceRepositoryAdapterProtectBranchTreatsAlreadyProtectedAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/12/protected_branches":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "Protected branch 'main' already exists"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	adapter := NewSourceRepositoryAdapter(NewClient(Config{BaseURL: server.URL, Token: "secret"}))
	if err := adapter.ProtectBranch(context.Background(), "12", "main"); err != nil {
		t.Fatalf("already protected branch should be successful: %v", err)
	}
}

func TestGitLabClientRetriesUnavailableStatus(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.Header.Get("PRIVATE-TOKEN") == "" {
			t.Fatalf("missing private token")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 42, "http_url_to_repo": "https://gitlab.example/retry.git"})
	}))
	defer server.Close()
	adapter := NewSourceRepositoryAdapter(NewClient(Config{BaseURL: server.URL, Token: "secret", RetryMax: 1}))
	project, err := adapter.CreateProject(context.Background(), sourcerepository.GitProjectSpec{RepositoryName: "retry", DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("create with retry: %v", err)
	}
	if project.ID != "42" || calls != 2 {
		t.Fatalf("unexpected retry result project=%#v calls=%d", project, calls)
	}
}

func TestManifestRepositoryAdapterCommitAndMR(t *testing.T) {
	commitCalls := 0
	var commitBodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/repository/files/"):
			if strings.Contains(r.URL.Path, "existing.yaml") {
				_ = json.NewEncoder(w).Encode(map[string]any{"content": "b2xk", "encoding": "base64"})
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 File Not Found"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/99/repository/commits":
			commitCalls++
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode commit body: %v", err)
			}
			commitBodies = append(commitBodies, body)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "abc123"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/99/merge_requests":
			_ = json.NewEncoder(w).Encode(map[string]any{"iid": 7, "sha": "def456", "web_url": "https://gitlab.example/mr/7"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	adapter := NewManifestRepositoryAdapter(NewClient(Config{BaseURL: server.URL}), "99")
	commit, err := adapter.CommitFiles(context.Background(), gitops.CommitSpec{Branch: "main", Message: "deploy", Files: []gitops.CommitFile{{Path: "apps/order/dev/values.yaml", Content: "image: v1"}, {Path: "apps/order/dev/existing.yaml", Content: "image: v2"}}})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if commit.CommitSHA != "abc123" {
		t.Fatalf("unexpected commit: %#v", commit)
	}
	mr, err := adapter.CreateMergeRequest(context.Background(), gitops.MergeRequestSpec{SourceBranch: "paas/promotion_1", TargetBranch: "main", Title: "deploy", Files: []gitops.CommitFile{{Path: "apps/order/prod/values.yaml", Content: "image: v1"}}})
	if err != nil {
		t.Fatalf("mr: %v", err)
	}
	deleted, err := adapter.DeleteFiles(context.Background(), gitops.DeleteFilesSpec{Branch: "main", Message: "delete", Paths: []string{"apps/order/dev/existing.yaml", "apps/order/dev/missing.yaml"}})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleted.CommitSHA != "abc123" {
		t.Fatalf("unexpected delete commit: %#v", deleted)
	}
	if mr.ID != "7" || commitCalls != 3 {
		t.Fatalf("unexpected mr result: %#v commitCalls=%d", mr, commitCalls)
	}
	firstActions := commitBodies[0]["actions"].([]any)
	if firstActions[0].(map[string]any)["action"] != "create" || firstActions[1].(map[string]any)["action"] != "update" {
		t.Fatalf("commit should create missing files and update existing files, body=%+v", commitBodies[0])
	}
	if commitBodies[1]["start_branch"] != "main" {
		t.Fatalf("MR commit should create source branch from target branch, body=%+v", commitBodies[1])
	}
	deleteActions := commitBodies[2]["actions"].([]any)
	if len(deleteActions) != 1 || deleteActions[0].(map[string]any)["action"] != "delete" || deleteActions[0].(map[string]any)["file_path"] != "apps/order/dev/existing.yaml" {
		t.Fatalf("delete commit should delete only existing files, body=%+v", commitBodies[2])
	}
}

func TestManifestRepositoryAdapterCreatesTagAndReusesExistingTag(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v4/projects/99/repository/tags" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		calls++
		if calls == 2 {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"message":"Tag already exists"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"name": "paas-app-v0.1.0", "target": "abc123"})
	}))
	defer server.Close()
	adapter := NewManifestRepositoryAdapter(NewClient(Config{BaseURL: server.URL}), "99")
	if tag, err := adapter.CreateTag(context.Background(), "paas-app-v0.1.0", "main"); err != nil || tag.Name != "paas-app-v0.1.0" {
		t.Fatalf("create tag: tag=%+v err=%v", tag, err)
	}
	if tag, err := adapter.CreateTag(context.Background(), "paas-app-v0.1.0", "main"); err != nil || tag.Name != "paas-app-v0.1.0" {
		t.Fatalf("existing tag should be reused: tag=%+v err=%v", tag, err)
	}
}

func TestSourceRepositoryAdapterRepositoryManagementCalls(t *testing.T) {
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.Method+" "+r.URL.Path]++
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/12/protected_branches":
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/12/hooks":
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/12/members":
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/12/remote_mirrors":
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/12":
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	adapter := NewSourceRepositoryAdapter(NewClient(Config{BaseURL: server.URL, Token: "secret"}))
	if err := adapter.InitializeRepository(context.Background(), "12", "main"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := adapter.ProtectBranch(context.Background(), "12", "main"); err != nil {
		t.Fatalf("protect: %v", err)
	}
	if err := adapter.ConfigureWebhook(context.Background(), "12", "https://paas.example/hook"); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	if err := adapter.SyncMembers(context.Background(), "12", []sourcerepository.GitMemberAccess{{UserID: "u1", Access: sourcerepository.GitAccessOwner}, {UserID: "u2", Access: sourcerepository.GitAccessMaintainer}, {UserID: "u3", Access: sourcerepository.GitAccessDeveloper}, {UserID: "u4", Access: sourcerepository.GitAccessReporter}}); err != nil {
		t.Fatalf("members: %v", err)
	}
	if err := adapter.MirrorRepository(context.Background(), sourcerepository.GitMirrorSpec{GitProjectID: "12", SourceURL: "https://git.example/order.git"}); err != nil {
		t.Fatalf("mirror: %v", err)
	}
	if err := adapter.VerifyRepository(context.Background(), "12"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if seen["POST /api/v4/projects/12/members"] != 4 {
		t.Fatalf("expected all member sync calls, got %#v", seen)
	}
}

func TestManifestRepositoryAdapterReadFileAndGetMR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.EscapedPath()
		switch {
		case r.Method == http.MethodGet && path == "/api/v4/projects/99/repository/files/apps%2Forder%2Fdev%2Fvalues.yaml":
			_ = json.NewEncoder(w).Encode(map[string]string{"content": base64.StdEncoding.EncodeToString([]byte("image: v1")), "encoding": "base64"})
		case r.Method == http.MethodGet && path == "/api/v4/projects/99/merge_requests/7":
			_ = json.NewEncoder(w).Encode(map[string]any{"iid": 7, "state": "merged", "web_url": "https://gitlab.example/mr/7"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	adapter := NewManifestRepositoryAdapter(NewClient(Config{BaseURL: server.URL}), "99")
	content, err := adapter.ReadFile(context.Background(), "apps/order/dev/values.yaml", "main")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if content != "image: v1" {
		t.Fatalf("unexpected content %q", content)
	}
	mr, err := adapter.GetMergeRequest(context.Background(), "7")
	if err != nil {
		t.Fatalf("get mr: %v", err)
	}
	if !mr.Merged || mr.ID != "7" {
		t.Fatalf("unexpected mr: %#v", mr)
	}
}

func TestGitLabFakeAdaptersFollowPortContracts(t *testing.T) {
	source := &FakeSourceRepositoryAdapter{Files: []sourcerepository.RepositoryFile{{Path: "pom.xml", Type: "blob"}}}
	project, err := source.CreateProject(context.Background(), sourcerepository.GitProjectSpec{RepositoryID: "repo_1", RepositoryName: "order"})
	if err != nil || project.ID != "git_repo_1" {
		t.Fatalf("fake create project: %#v err=%v", project, err)
	}
	if err := source.InitializeRepository(context.Background(), project.ID, "main"); err != nil {
		t.Fatalf("fake init: %v", err)
	}
	if err := source.DeleteProject(context.Background(), project.ID); err != nil {
		t.Fatalf("fake delete: %v", err)
	}
	if err := source.ProtectBranch(context.Background(), project.ID, "main"); err != nil {
		t.Fatalf("fake protect: %v", err)
	}
	if err := source.ConfigureWebhook(context.Background(), project.ID, "https://paas.example/hook"); err != nil {
		t.Fatalf("fake webhook: %v", err)
	}
	if err := source.SyncMembers(context.Background(), project.ID, nil); err != nil {
		t.Fatalf("fake members: %v", err)
	}
	if err := source.MirrorRepository(context.Background(), sourcerepository.GitMirrorSpec{}); err != nil {
		t.Fatalf("fake mirror: %v", err)
	}
	if err := source.VerifyRepository(context.Background(), project.ID); err != nil {
		t.Fatalf("fake verify: %v", err)
	}
	files, err := source.ListFiles(context.Background(), project.ID, "main")
	if err != nil || len(files) != 1 {
		t.Fatalf("fake files: %#v err=%v", files, err)
	}
	source.Tree = map[string][]sourcerepository.RepositoryTreeItem{"": {{Name: "src", Path: "src", Type: "tree"}}}
	tree, err := source.ListTree(context.Background(), project.ID, "main", "")
	if err != nil || len(tree) != 1 {
		t.Fatalf("fake tree: %#v err=%v", tree, err)
	}
	branches, err := source.ListBranches(context.Background(), project.ID)
	if err != nil || len(branches) != 1 || branches[0].Name != "main" {
		t.Fatalf("fake branches: %#v err=%v", branches, err)
	}

	manifest := NewFakeManifestRepositoryAdapter()
	if _, err := manifest.ReadFile(context.Background(), "missing", "main"); err == nil {
		t.Fatalf("missing fake manifest file should fail")
	}
	if _, err := manifest.CommitFiles(context.Background(), gitops.CommitSpec{Files: []gitops.CommitFile{{Path: "values.yaml", Content: "image: v1"}}}); err != nil {
		t.Fatalf("fake commit: %v", err)
	}
	if _, err := manifest.ReadFile(context.Background(), "values.yaml", "main"); err != nil {
		t.Fatalf("fake read: %v", err)
	}
	if _, err := manifest.CreateMergeRequest(context.Background(), gitops.MergeRequestSpec{Files: []gitops.CommitFile{{Path: "prod.yaml", Content: "image: v1"}}}); err != nil {
		t.Fatalf("fake mr: %v", err)
	}
	if mr, err := manifest.GetMergeRequest(context.Background(), "1"); err != nil || mr.State != "opened" {
		t.Fatalf("fake get mr: %#v err=%v", mr, err)
	}
}

func TestGitLabHTTPHelpersAndErrorMapping(t *testing.T) {
	if reader, contentType, err := encodeBody(nil); err != nil || reader != nil || contentType != "" {
		t.Fatalf("nil body = %#v %q %v", reader, contentType, err)
	}
	if _, _, err := encodeBody(func() {}); err == nil {
		t.Fatalf("unmarshalable body should fail")
	}
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(`{"name":"ok"}`))}
	var decoded map[string]string
	if err := decodeResponse(resp, &decoded); err != nil || decoded["name"] != "ok" {
		t.Fatalf("decode response: %#v err=%v", decoded, err)
	}
	if err := decodeResponse(&http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(bytes.NewBuffer(nil))}, nil); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("forbidden should map permission denied, got %v", err)
	}
	for status, want := range map[int]shared.ErrorCode{
		http.StatusUnauthorized:        shared.CodeUnavailable,
		http.StatusForbidden:           shared.CodePermissionDenied,
		http.StatusNotFound:            shared.CodeNotFound,
		http.StatusConflict:            shared.CodeConflict,
		http.StatusTooManyRequests:     shared.CodeUnavailable,
		http.StatusInternalServerError: shared.CodeInternal,
	} {
		if got := mapStatus(status); got != want {
			t.Fatalf("mapStatus(%d)=%s want %s", status, got, want)
		}
	}
	req, _ := http.NewRequest(http.MethodPost, "https://gitlab.example/api", io.NopCloser(bytes.NewBufferString("body")))
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
}

func TestGitLabAdaptersReturnRequestConstructionErrors(t *testing.T) {
	client := NewClient(Config{BaseURL: "http://[::1"})
	source := NewSourceRepositoryAdapter(client)
	if _, err := source.CreateProject(context.Background(), sourcerepository.GitProjectSpec{}); err == nil {
		t.Fatalf("create project should fail on invalid base URL")
	}
	if err := source.InitializeRepository(context.Background(), "12", "main"); err != nil {
		t.Fatalf("initialize is a no-op and should not fail: %v", err)
	}
	if err := source.DeleteProject(context.Background(), "12"); err == nil {
		t.Fatalf("delete should fail on invalid base URL")
	}
	if err := source.ProtectBranch(context.Background(), "12", "main"); err == nil {
		t.Fatalf("protect should fail on invalid base URL")
	}
	if err := source.ConfigureWebhook(context.Background(), "12", "https://paas.example/hook"); err == nil {
		t.Fatalf("webhook should fail on invalid base URL")
	}
	if err := source.SyncMembers(context.Background(), "12", []sourcerepository.GitMemberAccess{{UserID: "u1"}}); err == nil {
		t.Fatalf("sync members should fail on invalid base URL")
	}
	if err := source.MirrorRepository(context.Background(), sourcerepository.GitMirrorSpec{GitProjectID: "12"}); err == nil {
		t.Fatalf("mirror should fail on invalid base URL")
	}
	if err := source.VerifyRepository(context.Background(), "12"); err == nil {
		t.Fatalf("verify should fail on invalid base URL")
	}
	if _, err := source.ListFiles(context.Background(), "12", "main"); err == nil {
		t.Fatalf("list files should fail on invalid base URL")
	}

	manifest := NewManifestRepositoryAdapter(client, "99")
	if _, err := manifest.ReadFile(context.Background(), "values.yaml", "main"); err == nil {
		t.Fatalf("read file should fail on invalid base URL")
	}
	if _, err := manifest.CommitFiles(context.Background(), gitops.CommitSpec{}); err == nil {
		t.Fatalf("commit should fail on invalid base URL")
	}
	if _, err := manifest.CreateMergeRequest(context.Background(), gitops.MergeRequestSpec{}); err == nil {
		t.Fatalf("mr should fail on invalid base URL")
	}
	if _, err := manifest.GetMergeRequest(context.Background(), "1"); err == nil {
		t.Fatalf("get mr should fail on invalid base URL")
	}
}

func TestManifestRepositoryAdapterRejectsInvalidBase64Content(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"content": "%%%", "encoding": "base64"})
	}))
	defer server.Close()
	adapter := NewManifestRepositoryAdapter(NewClient(Config{BaseURL: server.URL}), "99")
	if _, err := adapter.ReadFile(context.Background(), "values.yaml", "main"); err == nil {
		t.Fatalf("invalid base64 content should fail")
	}
}
