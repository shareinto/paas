package jenkins

import (
	"context"

	"github.com/shareinto/paas/internal/modules/build"
)

type FakeAdapter struct {
	Jobs        []build.BuildJobSpec
	DeletedJobs []string
	Triggers    []map[string]string
	Logs        map[int64]build.ProgressiveText
	Statuses    map[int64]build.BuildStatus
	CancelCalls int
}

func (f *FakeAdapter) EnsureJob(_ context.Context, spec build.BuildJobSpec) error {
	f.Jobs = append(f.Jobs, spec)
	return nil
}
func (f *FakeAdapter) DeleteJob(_ context.Context, jobName string) error {
	f.DeletedJobs = append(f.DeletedJobs, jobName)
	return nil
}
func (f *FakeAdapter) TriggerBuild(_ context.Context, _ string, params map[string]string) (build.BuildQueueItem, error) {
	f.Triggers = append(f.Triggers, params)
	return build.BuildQueueItem{QueueID: "queue_fake"}, nil
}
func (f *FakeAdapter) GetQueueItem(_ context.Context, queueID string) (build.BuildQueueItem, error) {
	return build.BuildQueueItem{QueueID: queueID, Started: true, BuildNumber: 1}, nil
}
func (f *FakeAdapter) GetBuildStatus(_ context.Context, _ string, buildNumber int64) (build.BuildStatus, error) {
	if f.Statuses == nil {
		return build.BuildStatus{BuildNumber: buildNumber, Building: true, Status: build.BuildRunRunning}, nil
	}
	return f.Statuses[buildNumber], nil
}
func (f *FakeAdapter) ProgressiveText(_ context.Context, _ string, buildNumber int64, _ int64) (build.ProgressiveText, error) {
	if f.Logs == nil {
		return build.ProgressiveText{}, nil
	}
	return f.Logs[buildNumber], nil
}
func (f *FakeAdapter) CancelBuild(context.Context, string, int64) error {
	f.CancelCalls++
	return nil
}
