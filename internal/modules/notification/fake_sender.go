package notification

import (
	"context"
	"sync"
)

type FakeSender struct {
	mu       sync.Mutex
	Messages []Message
	Err      error
}

func (s *FakeSender) Send(_ context.Context, _ NotificationChannel, message Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	s.Messages = append(s.Messages, message)
	return nil
}
