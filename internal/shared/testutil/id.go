package testutil

import (
	"fmt"
	"sync"

	"github.com/shareinto/paas/internal/shared"
)

type FakeIDGenerator struct {
	mu   sync.Mutex
	next int
}

func NewFakeIDGenerator(start int) *FakeIDGenerator {
	return &FakeIDGenerator{next: start}
}

func (g *FakeIDGenerator) NewID(prefix string) (shared.ID, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	id := shared.ID(fmt.Sprintf("%s_%d", prefix, g.next))
	g.next++
	return id, nil
}
