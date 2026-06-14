package gitops

import (
	"context"
	"sync"

	"github.com/shareinto/paas/internal/shared"
)

type FakeManifestRepository struct {
	mu         sync.Mutex
	Files      map[string]string
	Commits    []CommitSpec
	MRs        []MergeRequestSpec
	Tags       map[string]string
	nextCommit int
}

func NewFakeManifestRepository() *FakeManifestRepository {
	return &FakeManifestRepository{Files: map[string]string{}, Tags: map[string]string{}}
}

func (r *FakeManifestRepository) ReadFile(_ context.Context, path string, _ string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	content, ok := r.Files[path]
	if !ok {
		return "", shared.NewError(shared.CodeNotFound, "manifest file not found")
	}
	return content, nil
}

func (r *FakeManifestRepository) CommitFiles(_ context.Context, spec CommitSpec) (CommitResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextCommit++
	for _, file := range spec.Files {
		r.Files[file.Path] = file.Content
	}
	r.Commits = append(r.Commits, spec)
	return CommitResult{CommitSHA: "commit_" + string(rune('0'+r.nextCommit))}, nil
}

func (r *FakeManifestRepository) CreateMergeRequest(_ context.Context, spec MergeRequestSpec) (MergeRequestResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextCommit++
	for _, file := range spec.Files {
		r.Files[file.Path] = file.Content
	}
	r.MRs = append(r.MRs, spec)
	id := "mr_" + string(rune('0'+len(r.MRs)))
	return MergeRequestResult{ID: id, CommitSHA: "commit_" + string(rune('0'+r.nextCommit)), WebURL: "https://gitlab.example/" + id}, nil
}

func (r *FakeManifestRepository) GetMergeRequest(_ context.Context, mrID string) (MergeRequest, error) {
	return MergeRequest{ID: mrID, State: "opened", WebURL: "https://gitlab.example/" + mrID}, nil
}

func (r *FakeManifestRepository) CreateTag(_ context.Context, name string, ref string) (TagResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Tags == nil {
		r.Tags = map[string]string{}
	}
	if existing := r.Tags[name]; existing != "" {
		return TagResult{Name: name, Ref: existing}, nil
	}
	r.Tags[name] = ref
	return TagResult{Name: name, Ref: ref}, nil
}
