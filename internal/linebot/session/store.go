package session

import (
	"sync"
	"time"

	"github.com/dfgh012316/popofinder/internal/linebot/search"
)

const ttl = 30 * time.Minute

// State holds a user's last search parameters for pagination.
type State struct {
	SearchTerm string
	City       *string
	SearchType search.SearchType
	Timestamp  time.Time
}

// Store is a thread-safe in-memory store of user search states.
type Store struct {
	mu     sync.Mutex
	states map[string]*State
}

func NewStore() *Store {
	return &Store{states: make(map[string]*State)}
}

// Get retrieves the state for the given userID. Expired entries are lazily evicted.
// Returns nil if no valid state exists.
func (s *Store) Get(userID string) *State {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Evict expired entries
	for uid, st := range s.states {
		if now.Sub(st.Timestamp) > ttl {
			delete(s.states, uid)
		}
	}

	return s.states[userID]
}

// Set stores or updates the state for the given userID.
func (s *Store) Set(userID string, st State) {
	st.Timestamp = time.Now()
	s.mu.Lock()
	s.states[userID] = &st
	s.mu.Unlock()
}
