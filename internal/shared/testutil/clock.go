package testutil

import "time"

type FakeClock struct {
	now time.Time
}

func NewFakeClock(now time.Time) *FakeClock {
	return &FakeClock{now: now.UTC()}
}

func (c *FakeClock) Now() time.Time {
	return c.now
}

func (c *FakeClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}
