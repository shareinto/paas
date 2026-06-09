package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/paasagent"
	"github.com/shareinto/paas/internal/shared"
)

func main() {
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
	config = config.Normalize()
	if err := config.Validate(); err != nil {
		log.Fatalf("paas-agent 配置校验失败: %v", err)
	}
	fmt.Printf("paas-agent cluster=%s heartbeat=%s snapshot=%s\n", config.ClusterID, config.HeartbeatInterval, config.SnapshotInterval)
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
