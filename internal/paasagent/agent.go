package paasagent

import (
	"context"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/shared"
)

const watchSnapshotDebounce = 5 * time.Second

type Agent struct {
	config        Config
	client        ControlPlaneClient
	reader        KubernetesReader
	clock         shared.Clock
	watchDebounce time.Duration
	termMu        sync.Mutex
	terms         map[string]chan []byte
	runtimeMu     sync.Mutex
	runtimeSender RuntimeMessageSender
}

func New(config Config, client ControlPlaneClient, reader KubernetesReader, clock shared.Clock) *Agent {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Agent{config: config.Normalize(), client: client, reader: reader, clock: clock, watchDebounce: watchSnapshotDebounce, terms: map[string]chan []byte{}}
}

func (a *Agent) SendHeartbeat(ctx context.Context) error {
	return a.client.Heartbeat(ctx, clusteragent.ClusterHeartbeat{ClusterID: a.config.ClusterID, ObservedAt: a.clock.Now(), ControlPlaneURL: a.config.ControlPlaneURL})
}

func (a *Agent) ReportSnapshot(ctx context.Context) (clusteragent.StatusReport, error) {
	snapshot, err := a.reader.Snapshot(ctx, a.config.Namespaces)
	if err != nil {
		return clusteragent.StatusReport{}, err
	}
	report := ToStatusReport(a.config.ClusterID, snapshot, a.clock.Now())
	if err := a.client.ReportStatus(ctx, report); err != nil {
		return clusteragent.StatusReport{}, err
	}
	if len(report.Events) > 0 {
		if err := a.client.ReportEvents(ctx, report); err != nil {
			return clusteragent.StatusReport{}, err
		}
	}
	return report, nil
}

func (a *Agent) RunTaskOnce(ctx context.Context) error {
	tasks, err := a.client.PullTasks(ctx)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		status, message := "succeeded", "任务执行成功"
		if err := a.executeTask(ctx, task); err != nil {
			status = "failed"
			message = err.Error()
		}
		if err := a.client.ReportTaskResult(ctx, task.ID.String(), status, message); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) WatchChanges(ctx context.Context) error {
	changes := make(chan RuntimeInvalidation, 64)
	watchErr := make(chan error, 1)
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		watchErr <- a.reader.RunRuntimeCache(watchCtx, a.config.Namespaces, func(invalidation RuntimeInvalidation) {
			select {
			case changes <- invalidation:
			default:
			}
		})
	}()
	pending := map[RuntimeInvalidation]struct{}{}
	var snapshotTimer *time.Timer
	var snapshotC <-chan time.Time
	var invalidationTimer *time.Timer
	var invalidationC <-chan time.Time
	stopTimer := func(timer *time.Timer) {
		if timer != nil {
			timer.Stop()
		}
	}
	defer func() {
		stopTimer(snapshotTimer)
		stopTimer(invalidationTimer)
	}()
	for {
		select {
		case <-ctx.Done():
			cancel()
			return ctx.Err()
		case err := <-watchErr:
			cancel()
			return err
		case invalidation := <-changes:
			if invalidation.ApplicationID != "" && strings.TrimSpace(invalidation.StageKey) != "" {
				pending[invalidation] = struct{}{}
				if invalidationTimer == nil {
					invalidationTimer = time.NewTimer(500 * time.Millisecond)
					invalidationC = invalidationTimer.C
				}
			}
			if snapshotTimer == nil {
				snapshotTimer = time.NewTimer(a.watchDebounce)
				snapshotC = snapshotTimer.C
			}
		case <-invalidationC:
			for invalidation := range pending {
				_ = a.sendRuntimeInvalidation(ctx, invalidation)
				delete(pending, invalidation)
			}
			invalidationTimer = nil
			invalidationC = nil
		case <-snapshotC:
			_, _ = a.ReportSnapshot(ctx)
			snapshotTimer = nil
			snapshotC = nil
		}
	}
}

func (a *Agent) ConnectRuntime(ctx context.Context) error {
	return a.client.ConnectRuntime(ctx, a)
}

func (a *Agent) SetRuntimeSender(sender RuntimeMessageSender) {
	a.runtimeMu.Lock()
	defer a.runtimeMu.Unlock()
	a.runtimeSender = sender
}

func (a *Agent) sendRuntimeInvalidation(ctx context.Context, invalidation RuntimeInvalidation) error {
	a.runtimeMu.Lock()
	sender := a.runtimeSender
	a.runtimeMu.Unlock()
	if sender == nil {
		return nil
	}
	return sender.SendRuntimeMessage(ctx, clusteragent.RuntimeWireMessage{Type: "stage_changed", ApplicationID: invalidation.ApplicationID, StageKey: invalidation.StageKey})
}

func (a *Agent) HandleRuntimeRequest(ctx context.Context, msg clusteragent.RuntimeWireMessage, sender RuntimeMessageSender) error {
	switch msg.Type {
	case "list_resources":
		resources, err := a.reader.ListRuntimeResources(ctx, a.config.Namespaces, msg.ApplicationID, msg.StageKey)
		return sender.SendRuntimeMessage(ctx, runtimeResponse(msg.ID, resources, err, true))
	case "watch_resources":
		return a.reader.WatchRuntimeResources(ctx, a.config.Namespaces, msg.ApplicationID, msg.StageKey, func(resources []RuntimeResource) {
			_ = sender.SendRuntimeMessage(ctx, clusteragent.RuntimeWireMessage{ID: msg.ID, Type: "snapshot", Resources: resources})
		})
	case "restart_resource":
		err := a.reader.RestartRuntimeResource(ctx, msg.Target.Kind, msg.Target.Namespace, msg.Target.Name)
		return sender.SendRuntimeMessage(ctx, runtimeResponse(msg.ID, nil, err, true))
	case "pod_logs":
		writer := runtimeLogSender{id: msg.ID, sender: sender, ctx: ctx}
		err := a.reader.StreamPodLogs(ctx, msg.Target.Namespace, msg.Target.Name, msg.LogOptions.Container, msg.LogOptions.TailLines, writer)
		return sender.SendRuntimeMessage(ctx, runtimeResponse(msg.ID, nil, err, true))
	case "terminal_open":
		in := make(chan []byte, 16)
		out := make(chan []byte, 16)
		a.addTerminal(msg.ID, in)
		go func() {
			defer a.removeTerminal(msg.ID)
			err := a.reader.Terminal(ctx, msg.Target.Namespace, msg.Target.Name, msg.TermOptions.Container, firstNonEmpty(msg.TermOptions.Command, "/bin/sh"), in, out)
			_ = sender.SendRuntimeMessage(ctx, runtimeResponse(msg.ID, nil, err, true))
		}()
		go func() {
			for data := range out {
				_ = sender.SendRuntimeMessage(ctx, clusteragent.RuntimeWireMessage{ID: msg.ID, Type: "terminal_output", Data: base64.StdEncoding.EncodeToString(data)})
			}
		}()
		return nil
	case "terminal_input":
		data, err := base64.StdEncoding.DecodeString(msg.Data)
		if err != nil {
			return sender.SendRuntimeMessage(ctx, clusteragent.RuntimeWireMessage{ID: msg.ID, Error: err.Error(), Done: true})
		}
		if ch := a.terminal(msg.ID); ch != nil {
			ch <- data
		}
		return nil
	case "terminal_close":
		a.removeTerminal(msg.ID)
		return nil
	default:
		return sender.SendRuntimeMessage(ctx, clusteragent.RuntimeWireMessage{ID: msg.ID, Error: "unsupported runtime request", Done: true})
	}
}

func (a *Agent) addTerminal(id string, ch chan []byte) {
	a.termMu.Lock()
	defer a.termMu.Unlock()
	a.terms[id] = ch
}

func (a *Agent) terminal(id string) chan []byte {
	a.termMu.Lock()
	defer a.termMu.Unlock()
	return a.terms[id]
}

func (a *Agent) removeTerminal(id string) {
	a.termMu.Lock()
	ch := a.terms[id]
	delete(a.terms, id)
	a.termMu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (a *Agent) executeTask(ctx context.Context, task Task) error {
	target := strings.TrimSpace(task.TargetRef)
	if target == "" {
		target = strings.TrimSpace(task.Payload["argo_application"])
	}
	switch task.Type {
	case "argocd_refresh":
		if target == "" {
			return shared.NewError(shared.CodeInvalidArgument, "argo application target is required")
		}
		return a.reader.RefreshArgoApplication(ctx, target)
	case "argocd_sync":
		if target == "" {
			return shared.NewError(shared.CodeInvalidArgument, "argo application target is required")
		}
		return a.reader.SyncArgoApplication(ctx, target)
	case "runtime_restart":
		kind := strings.TrimSpace(task.Payload["kind"])
		namespace := strings.TrimSpace(task.Payload["namespace"])
		name := strings.TrimSpace(task.Payload["name"])
		if kind == "" || namespace == "" || name == "" {
			return shared.NewError(shared.CodeInvalidArgument, "runtime restart target is required")
		}
		return a.reader.RestartRuntimeResource(ctx, kind, namespace, name)
	default:
		return shared.NewError(shared.CodeInvalidArgument, "unsupported agent task")
	}
}

func runtimeResponse(id string, resources []RuntimeResource, err error, done bool) clusteragent.RuntimeWireMessage {
	resp := clusteragent.RuntimeWireMessage{ID: id, Resources: resources, Done: done}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp
}

type runtimeLogSender struct {
	id     string
	sender RuntimeMessageSender
	ctx    context.Context
}

func (w runtimeLogSender) Write(data []byte) (int, error) {
	err := w.sender.SendRuntimeMessage(w.ctx, clusteragent.RuntimeWireMessage{ID: w.id, Type: "log", Data: base64.StdEncoding.EncodeToString(data)})
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
