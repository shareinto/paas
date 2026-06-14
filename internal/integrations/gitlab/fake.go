package gitlab

import (
	"context"

	"github.com/shareinto/paas/internal/modules/gitops"
	"github.com/shareinto/paas/internal/modules/sourcerepository"
	"github.com/shareinto/paas/internal/shared"
)

type FakeSourceRepositoryAdapter struct {
	Projects []sourcerepository.GitProjectSpec
	Files    []sourcerepository.RepositoryFile
	Tree     map[string][]sourcerepository.RepositoryTreeItem
	Branches []sourcerepository.RepositoryBranch
}

func (f *FakeSourceRepositoryAdapter) CreateProject(_ context.Context, spec sourcerepository.GitProjectSpec) (sourcerepository.GitProject, error) {
	f.Projects = append(f.Projects, spec)
	return sourcerepository.GitProject{ID: "git_" + spec.RepositoryID.String(), HTTPURL: "https://gitlab.example/" + spec.RepositoryName + ".git", SSHURL: "git@gitlab.example:" + spec.RepositoryName + ".git"}, nil
}
func (f *FakeSourceRepositoryAdapter) InitializeRepository(context.Context, string, string) error {
	return nil
}
func (f *FakeSourceRepositoryAdapter) DeleteProject(context.Context, string) error {
	return nil
}
func (f *FakeSourceRepositoryAdapter) ProtectBranch(context.Context, string, string) error {
	return nil
}
func (f *FakeSourceRepositoryAdapter) ConfigureWebhook(context.Context, string, string) error {
	return nil
}
func (f *FakeSourceRepositoryAdapter) SyncMembers(context.Context, string, []sourcerepository.GitMemberAccess) error {
	return nil
}
func (f *FakeSourceRepositoryAdapter) MirrorRepository(context.Context, sourcerepository.GitMirrorSpec) error {
	return nil
}
func (f *FakeSourceRepositoryAdapter) VerifyRepository(context.Context, string) error { return nil }
func (f *FakeSourceRepositoryAdapter) ListFiles(context.Context, string, string) ([]sourcerepository.RepositoryFile, error) {
	return f.Files, nil
}
func (f *FakeSourceRepositoryAdapter) ListTree(_ context.Context, _ string, _ string, path string) ([]sourcerepository.RepositoryTreeItem, error) {
	if f.Tree == nil {
		return nil, nil
	}
	return append([]sourcerepository.RepositoryTreeItem(nil), f.Tree[path]...), nil
}
func (f *FakeSourceRepositoryAdapter) ListBranches(context.Context, string) ([]sourcerepository.RepositoryBranch, error) {
	if len(f.Branches) == 0 {
		return []sourcerepository.RepositoryBranch{{Name: "main", Default: true}}, nil
	}
	return append([]sourcerepository.RepositoryBranch(nil), f.Branches...), nil
}

type FakeManifestRepositoryAdapter struct {
	Files   map[string]string
	Commits []gitops.CommitSpec
	MRs     []gitops.MergeRequestSpec
	Tags    map[string]string
}

func NewFakeManifestRepositoryAdapter() *FakeManifestRepositoryAdapter {
	return &FakeManifestRepositoryAdapter{Files: map[string]string{}, Tags: map[string]string{}}
}

func (f *FakeManifestRepositoryAdapter) ReadFile(_ context.Context, path string, _ string) (string, error) {
	content, ok := f.Files[path]
	if !ok {
		return "", shared.NewError(shared.CodeNotFound, "manifest file not found")
	}
	return content, nil
}
func (f *FakeManifestRepositoryAdapter) CommitFiles(_ context.Context, spec gitops.CommitSpec) (gitops.CommitResult, error) {
	for _, file := range spec.Files {
		f.Files[file.Path] = file.Content
	}
	f.Commits = append(f.Commits, spec)
	return gitops.CommitResult{CommitSHA: "commit_fake"}, nil
}
func (f *FakeManifestRepositoryAdapter) CreateMergeRequest(_ context.Context, spec gitops.MergeRequestSpec) (gitops.MergeRequestResult, error) {
	for _, file := range spec.Files {
		f.Files[file.Path] = file.Content
	}
	f.MRs = append(f.MRs, spec)
	return gitops.MergeRequestResult{ID: "1", CommitSHA: "commit_mr_fake", WebURL: "https://gitlab.example/mr/1"}, nil
}
func (f *FakeManifestRepositoryAdapter) GetMergeRequest(_ context.Context, mrID string) (gitops.MergeRequest, error) {
	return gitops.MergeRequest{ID: mrID, State: "opened", WebURL: "https://gitlab.example/mr/" + mrID}, nil
}

func (f *FakeManifestRepositoryAdapter) CreateTag(_ context.Context, name string, ref string) (gitops.TagResult, error) {
	if f.Tags == nil {
		f.Tags = map[string]string{}
	}
	if existing := f.Tags[name]; existing != "" {
		return gitops.TagResult{Name: name, Ref: existing}, nil
	}
	f.Tags[name] = ref
	return gitops.TagResult{Name: name, Ref: ref}, nil
}
