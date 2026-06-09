package testutil

import (
	"testing"
	"time"
)

func TestFakeClock(t *testing.T) {
	start := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	clock := NewFakeClock(start)
	clock.Advance(2 * time.Hour)

	if got := clock.Now(); !got.Equal(start.Add(2 * time.Hour)) {
		t.Fatalf("Now() = %s", got)
	}
}

func TestFakeIDGenerator(t *testing.T) {
	generator := NewFakeIDGenerator(10)
	id, err := generator.NewID("app")
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	if id != "app_10" {
		t.Fatalf("id = %q", id)
	}
}
