package cache

import (
	"sync"
	"time"

	"github.com/shareinto/paas/internal/modules/clusteragent"
)

// StageRuntimeSnapshot holds cached runtime resources for a stage.
type StageRuntimeSnapshot struct {
	Resources []clusteragent.RuntimeResource
	CachedAt  time.Time
}

// BuildRunSnapshot holds cached build run status.
type BuildRunSnapshot struct {
	Status    string
	Result    string
	UpdatedAt time.Time
}

// Cache defines the interface for deployment page caching.
// Implementations must be safe for concurrent use.
type Cache interface {
	GetStageRuntime(appID, stageKey string) (*StageRuntimeSnapshot, bool)
	SetStageRuntime(appID, stageKey string, snapshot *StageRuntimeSnapshot)
	InvalidateStageRuntime(appID, stageKey string)

	GetBuildRunStatus(buildRunID string) (*BuildRunSnapshot, bool)
	SetBuildRunStatus(buildRunID string, snapshot *BuildRunSnapshot)
}

// MemoryCache is an in-memory Cache with TTL eviction.
type MemoryCache struct {
	mu           sync.RWMutex
	stageRuntime map[string]*ttlEntry[StageRuntimeSnapshot]
	buildRuns    map[string]*ttlEntry[BuildRunSnapshot]
	stageTTL     time.Duration
	buildTTL     time.Duration
}

type ttlEntry[T any] struct {
	value     T
	expiresAt time.Time
}

func NewMemoryCache(stageTTL, buildTTL time.Duration) *MemoryCache {
	return &MemoryCache{
		stageRuntime: make(map[string]*ttlEntry[StageRuntimeSnapshot]),
		buildRuns:    make(map[string]*ttlEntry[BuildRunSnapshot]),
		stageTTL:     stageTTL,
		buildTTL:     buildTTL,
	}
}

func stageKey(appID, stage string) string { return appID + "/" + stage }

func (c *MemoryCache) GetStageRuntime(appID, sk string) (*StageRuntimeSnapshot, bool) {
	c.mu.RLock()
	e := c.stageRuntime[stageKey(appID, sk)]
	c.mu.RUnlock()
	if e == nil || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return &e.value, true
}

func (c *MemoryCache) SetStageRuntime(appID, sk string, snapshot *StageRuntimeSnapshot) {
	c.mu.Lock()
	c.stageRuntime[stageKey(appID, sk)] = &ttlEntry[StageRuntimeSnapshot]{value: *snapshot, expiresAt: time.Now().Add(c.stageTTL)}
	c.mu.Unlock()
}

func (c *MemoryCache) InvalidateStageRuntime(appID, sk string) {
	c.mu.Lock()
	delete(c.stageRuntime, stageKey(appID, sk))
	c.mu.Unlock()
}

func (c *MemoryCache) GetBuildRunStatus(buildRunID string) (*BuildRunSnapshot, bool) {
	c.mu.RLock()
	e := c.buildRuns[buildRunID]
	c.mu.RUnlock()
	if e == nil || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return &e.value, true
}

func (c *MemoryCache) SetBuildRunStatus(buildRunID string, snapshot *BuildRunSnapshot) {
	c.mu.Lock()
	c.buildRuns[buildRunID] = &ttlEntry[BuildRunSnapshot]{value: *snapshot, expiresAt: time.Now().Add(c.buildTTL)}
	c.mu.Unlock()
}
