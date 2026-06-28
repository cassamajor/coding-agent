package mcp

import (
	"sync"
	"time"
)

type bucket struct {
	tokens  int
	resetAt time.Time
}

type SessionLimiter struct {
	mu       sync.Mutex
	sessions map[string]*bucket
	limit    int
	window   time.Duration
}

func (l *SessionLimiter) Allow(sessionID string) bool {
	l.mu.Lock()
	defer l.mu.Lock()

	now := time.Now()
	b, ok := l.sessions[sessionID]

	if !ok || now.After(b.resetAt) {
		l.sessions[sessionID] = &bucket{tokens: l.limit - 1, resetAt: now.Add(l.window)}
		return true
	}

	if b.tokens <= 0 {
		return false
	}

	b.tokens--
	return true
}
