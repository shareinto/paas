package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/paasagent"
	"github.com/shareinto/paas/internal/shared"
)

func main() {
	config, argoNamespace := runtimeConfigFromEnv()
	config = config.Normalize()
	if err := config.Validate(); err != nil {
		log.Fatalf("paas-agent 配置校验失败: %v", err)
	}
	reader, err := paasagent.NewInClusterKubernetesReader(argoNamespace)
	if err != nil {
		log.Fatalf("paas-agent Kubernetes client 初始化失败: %v", err)
	}
	agent := paasagent.New(config, paasagent.NewHTTPControlPlaneClient(config), reader, shared.SystemClock{})
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Printf("paas-agent cluster=%s heartbeat=%s snapshot=%s argocd_namespace=%s", config.ClusterID, config.HeartbeatInterval, config.SnapshotInterval, argoNamespace)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runAgent(ctx, agent, config.HeartbeatInterval, config.SnapshotInterval, logger); err != nil {
		log.Fatalf("paas-agent 运行失败: %v", err)
	}
}

type runtimeAgent interface {
	SendHeartbeat(ctx context.Context) error
	ReportSnapshot(ctx context.Context) (clusteragent.StatusReport, error)
	RunTaskOnce(ctx context.Context) error
	WatchChanges(ctx context.Context) error
	ConnectRuntime(ctx context.Context) error
}

func runtimeConfigFromEnv() (paasagent.Config, string) {
	config := paasagent.Config{
		ClusterID:       shared.ID(os.Getenv("PAAS_CLUSTER_ID")),
		ControlPlaneURL: os.Getenv("PAAS_CONTROL_PLANE_URL"),
		AgentToken:      os.Getenv("PAAS_AGENT_TOKEN"),
		Namespaces:      splitCSV(os.Getenv("PAAS_AGENT_NAMESPACES")),
	}
	if value := os.Getenv("PAAS_HEARTBEAT_INTERVAL"); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			config.HeartbeatInterval = duration
		}
	}
	if value := os.Getenv("PAAS_SNAPSHOT_INTERVAL"); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			config.SnapshotInterval = duration
		}
	}
	argoNamespace := strings.TrimSpace(os.Getenv("PAAS_ARGOCD_NAMESPACE"))
	if argoNamespace == "" {
		argoNamespace = "argocd"
	}
	return config.Normalize(), argoNamespace
}

func runAgent(ctx context.Context, agent runtimeAgent, heartbeatInterval time.Duration, snapshotInterval time.Duration, logger *log.Logger) error {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	if err := agent.SendHeartbeat(ctx); err != nil {
		return fmt.Errorf("发送心跳失败: %w", err)
	}
	if _, err := agent.ReportSnapshot(ctx); err != nil {
		return fmt.Errorf("上报状态快照失败: %w", err)
	}
	if err := agent.RunTaskOnce(ctx); err != nil {
		return fmt.Errorf("执行受控任务失败: %w", err)
	}
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := agent.WatchChanges(childCtx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for childCtx.Err() == nil {
			if err := agent.ConnectRuntime(childCtx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Printf("paas-agent 实时通道断开: %v", err)
				select {
				case <-childCtx.Done():
					return
				case <-time.After(3 * time.Second):
				}
			}
		}
	}()
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()
	snapshotTicker := time.NewTicker(snapshotInterval)
	defer snapshotTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			cancel()
			wg.Wait()
			return nil
		case err := <-errCh:
			cancel()
			wg.Wait()
			return fmt.Errorf("监听集群资源变化失败: %w", err)
		case <-heartbeatTicker.C:
			if err := agent.SendHeartbeat(ctx); err != nil {
				logger.Printf("paas-agent 心跳失败: %v", err)
			}
		case <-snapshotTicker.C:
			if _, err := agent.ReportSnapshot(ctx); err != nil {
				logger.Printf("paas-agent 状态快照上报失败: %v", err)
			}
			if err := agent.RunTaskOnce(ctx); err != nil {
				logger.Printf("paas-agent 受控任务执行失败: %v", err)
			}
		}
	}
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
