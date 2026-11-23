package store

import (
	"sync"
	"testing"
	"time"
)

func TestMissingFileIsEmptyState(t *testing.T) {
	st, err := New(t.TempDir()).Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Opened == nil || st.Prefs == nil || st.Sessions == nil {
		t.Error("maps must be initialised so callers can write to them")
	}
}

func TestUpdatePersists(t *testing.T) {
	s := New(t.TempDir())
	now := time.Now().UTC().Truncate(time.Second)
	if err := s.Update(func(st *State) {
		st.Opened["/p"] = now
		st.Prefs["/p"] = Prefs{Saved: true, Run: "go run .", Agents: []string{"claude"}}
	}); err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Opened["/p"].Equal(now) || st.Prefs["/p"].Run != "go run ." || st.Prefs["/p"].Agents[0] != "claude" {
		t.Errorf("round trip lost data: %+v", st)
	}
}

func TestConcurrentUpdatesDoNotLoseWrites(t *testing.T) {
	s := New(t.TempDir())
	var wg sync.WaitGroup
	const n = 20
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Update(func(st *State) { st.Sessions[string(rune('a'+i))] = Session{} })
		}()
	}
	wg.Wait()
	st, _ := s.Load()
	if len(st.Sessions) != n {
		t.Errorf("sessions = %d, want %d", len(st.Sessions), n)
	}
}
