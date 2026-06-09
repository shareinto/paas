package shared_test

import (
	"strings"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/shared"
	"github.com/shareinto/paas/internal/shared/testutil"
)

func TestRandomIDGenerator(t *testing.T) {
	id, err := (shared.RandomIDGenerator{}).NewID("app")
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	if !strings.HasPrefix(id.String(), "app_") {
		t.Fatalf("id = %q, want app_ prefix", id)
	}
	if _, err := (shared.RandomIDGenerator{}).NewID(" "); err == nil {
		t.Fatalf("empty prefix should fail")
	}
}

func TestValidateStatus(t *testing.T) {
	allowed := []string{"ready", "disabled"}
	if err := shared.ValidateStatus("ready", allowed); err != nil {
		t.Fatalf("ready should be allowed: %v", err)
	}
	if err := shared.ValidateStatus("", allowed); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("empty status should be invalid_argument, got %v", err)
	}
	if err := shared.ValidateStatus("running", allowed); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("unknown status should be invalid_argument, got %v", err)
	}
}

func TestFakeClockAndIDGenerator(t *testing.T) {
	start := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	clock := testutil.NewFakeClock(start)
	clock.Advance(time.Minute)
	if got := clock.Now(); !got.Equal(start.Add(time.Minute)) {
		t.Fatalf("clock.Now() = %s", got)
	}

	ids := testutil.NewFakeIDGenerator(7)
	first, err := ids.NewID("evt")
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	second, err := ids.NewID("evt")
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	if first != "evt_7" || second != "evt_8" {
		t.Fatalf("ids = %q, %q", first, second)
	}
}

func TestSystemClockNowIsUTC(t *testing.T) {
	now := (shared.SystemClock{}).Now()
	if now.Location() != time.UTC {
		t.Fatalf("SystemClock should return UTC time, got %s", now.Location())
	}
}
