package web

import (
	"sync"
	"time"
)

// maxTracked bounds the limiter's memory: past it, finished windows are dropped.
const maxTracked = 10000

// limiter allows n requests per client in each fixed window, so the web version
// cannot be used to copy the database quickly (OD-17).
type limiter struct {
	mu      sync.Mutex
	n       int
	window  time.Duration
	clients map[string]*clientWindow
}

type clientWindow struct {
	start time.Time
	count int
}

func newLimiter(n int, window time.Duration) *limiter {
	return &limiter{n: n, window: window, clients: map[string]*clientWindow{}}
}

// allow reports whether client may make another request at now, and counts it.
func (l *limiter) allow(client string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.clients) >= maxTracked {
		for k, c := range l.clients {
			if now.Sub(c.start) >= l.window {
				delete(l.clients, k)
			}
		}
	}
	c, ok := l.clients[client]
	if !ok || now.Sub(c.start) >= l.window {
		l.clients[client] = &clientWindow{start: now, count: 1}
		return true
	}
	if c.count >= l.n {
		return false
	}
	c.count++
	return true
}
