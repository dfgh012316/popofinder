package session

import (
	"testing"
	"time"

	"github.com/dfgh012316/popofinder/internal/linebot/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_SetAndGet(t *testing.T) {
	s := NewStore()
	s.Set("user1", State{SearchTerm: "陳", SearchType: search.TypeName})

	got := s.Get("user1")
	require.NotNil(t, got)
	assert.Equal(t, "陳", got.SearchTerm)
	assert.Equal(t, search.TypeName, got.SearchType)
}

func TestStore_GetMissing(t *testing.T) {
	s := NewStore()
	assert.Nil(t, s.Get("no_such_user"))
}

func TestStore_Expiry(t *testing.T) {
	s := NewStore()
	s.Set("user1", State{SearchTerm: "test", SearchType: search.TypeName})

	// Manually expire the entry
	s.mu.Lock()
	s.states["user1"].Timestamp = time.Now().Add(-31 * time.Minute)
	s.mu.Unlock()

	assert.Nil(t, s.Get("user1"))
}
