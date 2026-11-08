package storage_test

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/endview/affectweave/internal/affect"
	"github.com/endview/affectweave/internal/storage"
	"github.com/endview/affectweave/internal/testpg"
)

func TestRealPGDelayedApprovalDoesNotMutateOrCompoundPreview(t *testing.T) {
	s, _ := testpg.Open(t)
	scope, in := seed(t, s)
	state, err := s.Adopt(t.Context(), scope, storage.Input{ID: "later", RunID: "later-run", Text: "later input", At: in.At.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	c := storage.Change{Scope: scope, Expected: state.Revision, Action: "apply_proposal", Source: in.ID,
		Proposal: &affect.Proposal{Emotions: []affect.EmotionDelta{{Type: "interest", CauseID: "original-event", Cause: "original event", Delta: .2}}}}
	original, _ := json.Marshal(c)
	first, err := s.Preview(t.Context(), c)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Preview(t.Context(), c)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first.After)
	b, _ := json.Marshal(second.After)
	after, _ := json.Marshal(c)
	if string(original) != string(after) || string(a) != string(b) {
		t.Fatal("preview mutated the confirmed request or compounded decay")
	}
	if _, err = s.Apply(t.Context(), "delayed", "digest", c); err != nil {
		t.Fatal(err)
	}
	after, _ = json.Marshal(c)
	if string(original) != string(after) {
		t.Fatal("apply mutated caller request")
	}
	if _, err = s.Apply(t.Context(), "delayed", "digest", c); err != nil {
		t.Fatal("original idempotent request no longer matches", err)
	}
	state, err = s.Read(t.Context(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Emotions) != 1 {
		t.Fatal(state.Emotions)
	}
	for _, e := range state.Emotions {
		if math.Abs(e.Intensity-.2*math.Exp(-1)) > 1e-9 {
			t.Fatal("old impulse replayed at full strength", e.Intensity)
		}
	}
}
