package paasagent

import (
	"strings"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type Config struct {
	ClusterID         shared.ID
	ControlPlaneURL   string
	AgentToken        string
	HeartbeatInterval time.Duration
	SnapshotInterval  time.Duration
	Namespaces        []string
}

func (c Config) Normalize() Config {
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = 10 * time.Second
	}
	if c.SnapshotInterval == 0 {
		c.SnapshotInterval = 30 * time.Second
	}
	return c
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.ClusterID.String()) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "cluster id is required")
	}
	if strings.TrimSpace(c.ControlPlaneURL) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "control plane url is required")
	}
	if strings.TrimSpace(c.AgentToken) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "agent token is required")
	}
	return nil
}
