package storage_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/endview/affectweave/internal/affect"
	"github.com/endview/affectweave/internal/storage"
	"github.com/endview/affectweave/internal/testpg"
)

func seed(t *testing.T, s *storage.Store) (affect.Scope, storage.Input) {
	t.Helper()
	scope := affect.Scope{Core: s.CoreID, Space: "local", Role: "maya", Actor: "owner"}
	p := affect.Persona{ID: "maya", Name: "Maya", Identity: "Engineer", Frozen: true, Version: 1}
	c := storage.Change{Scope: scope, Action: "create_role", Expected: 0, Persona: &p, Spaces: []string{"local"}}
	if _, err := s.Apply(t.Context(), "create", "create-digest", c); err != nil {
		t.Fatal(err)
	}
	in := storage.Input{ID: "input-1", RunID: "run-1", Text: "hello", At: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)}
	if _, err := s.Adopt(t.Context(), scope, in); err != nil {
		t.Fatal(err)
	}
	return scope, in
}
func TestRealPGFrozenInputsCASAndLostACK(t *testing.T) {
	s, path := testpg.Open(t)
	scope, in := seed(t, s)
	frozen, err := s.Adopt(t.Context(), scope, in)
	if err != nil {
		t.Fatal(err)
	}
	c := storage.Change{Scope: scope, Action: "apply_proposal", Expected: frozen.Revision, Source: in.ID, Proposal: &affect.Proposal{Relationship: affect.Relationship{Trust: 1}}}
	preview, err := s.Preview(t.Context(), c)
	if err != nil || preview.Revision != 2 {
		t.Fatal(preview, err)
	}
	var group sync.WaitGroup
	failures := make(chan error, 8)
	for n := 0; n < 8; n++ {
		group.Add(1)
		go func() {
			defer group.Done()
			r, err := s.Apply(t.Context(), "one-operation", "content-digest", c)
			if err != nil {
				failures <- err
			} else if !r.Found || r.Revision != 3 {
				failures <- fmt.Errorf("bad result: %+v", r)
			}
		}()
	}
	group.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	state, err := s.Read(t.Context(), scope)
	if err != nil || state.Relationship.Trust != .55 || state.Revision != 3 {
		t.Fatal(state, err)
	}
	again, err := s.Adopt(t.Context(), scope, in)
	if err != nil || again.Revision != frozen.Revision || again.Relationship.Trust != .5 {
		t.Fatal("hook did not return frozen version", again, err)
	}
	c.Proposal.Relationship.Trust = -1
	if _, err = s.Apply(t.Context(), "one-operation", "content-digest", c); !errors.Is(err, storage.ErrOperation) {
		t.Fatal("changed payload reused", err)
	}
	if _, err = s.Apply(t.Context(), "new-operation", "new-digest", c); !errors.Is(err, storage.ErrConflict) {
		t.Fatal("stale CAS accepted", err)
	}
	reopened, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	result, err := reopened.Operation(t.Context(), "one-operation", "content-digest")
	if err != nil || !result.Found || result.Revision != 3 {
		t.Fatal("lost ACK result missing after reopen", result, err)
	}
	if _, err = reopened.Operation(t.Context(), "one-operation", "other"); !errors.Is(err, storage.ErrOperation) {
		t.Fatal(err)
	}
	bad := in
	bad.Text = "changed"
	if _, err = s.Adopt(t.Context(), scope, bad); !errors.Is(err, storage.ErrOperation) {
		t.Fatal(err)
	}
	other := scope
	other.Space = "group"
	if _, err = s.Adopt(t.Context(), other, in); !errors.Is(err, storage.ErrScope) {
		t.Fatal("cross-space leak", err)
	}
}
func TestRealPGSourceValidationFactsAndConfirmedMemory(t *testing.T) {
	s, _ := testpg.Open(t)
	scope, in := seed(t, s)
	state, _ := s.Read(t.Context(), scope)
	c := storage.Change{Scope: scope, Action: "apply_proposal", Expected: state.Revision, Source: "nonexistent", Proposal: &affect.Proposal{Medium: affect.MediumDelta{Stress: .05}}}
	if _, err := s.Apply(t.Context(), "missing", "digest", c); err == nil {
		t.Fatal("missing source accepted")
	}
	after, _ := s.Read(t.Context(), scope)
	if after.Revision != state.Revision {
		t.Fatal("failed proposal mutated state")
	}
	for _, status := range []string{"sent", "partial", "unknown", "failed"} {
		body, _ := json.Marshal(map[string]string{"state": status, "run_id": in.RunID})
		fact := storage.Fact{ID: "delivery:" + status, Kind: "delivery", Body: body, At: in.At}
		if err := s.RecordFact(t.Context(), scope, fact); err != nil {
			t.Fatal(err)
		}
		if err := s.RecordFact(t.Context(), scope, fact); err != nil {
			t.Fatal(err)
		}
	}
	facts, err := s.Facts(t.Context(), scope, 50)
	if err != nil || len(facts) != 4 {
		t.Fatal(facts, err)
	}
	feedback := storage.Input{ID: "input-2", RunID: "run-2", Text: "the delivered result helped", At: in.At.Add(time.Minute), FeedbackSource: "delivery:sent"}
	if _, err = s.Adopt(t.Context(), scope, feedback); err != nil {
		t.Fatal(err)
	}
	state, _ = s.Read(t.Context(), scope)
	c = storage.Change{Scope: scope, Action: "record_outcome", Expected: state.Revision, Source: "delivery:sent", Outcome: &storage.Outcome{FeedbackID: feedback.ID, Evidence: "L2", Summary: "user feedback", AcceptMemories: true, Proposal: affect.Proposal{Relationship: affect.Relationship{Trust: 1}, Memories: []affect.Memory{{Summary: "Keep the confirmed boundary", Salience: .8, Confidence: .9}}}}}
	if _, err = s.Apply(t.Context(), "feedback", "digest", c); err != nil {
		t.Fatal(err)
	}
	memories, err := s.Memories(t.Context(), scope)
	if err != nil || len(memories) != 1 || memories[0].Evidence != "L2" {
		t.Fatal(memories, err)
	}
	after, _ = s.Read(t.Context(), scope)
	if after.Relationship.Trust != .525 {
		t.Fatal(after.Relationship)
	}
	var super, createRole, createDB bool
	if err = s.DB.QueryRowContext(t.Context(), `SELECT rolsuper,rolcreaterole,rolcreatedb FROM pg_roles WHERE rolname=current_user`).Scan(&super, &createRole, &createDB); err != nil || super || createRole || createDB {
		t.Fatal("overprivileged role", err)
	}
}
