package storage_test

import (
	"encoding/json"
	"fmt"
	"github.com/endview/affectweave/internal/affect"
	"github.com/endview/affectweave/internal/storage"
	"github.com/endview/affectweave/internal/testpg"
	"testing"
	"time"
)

func TestRealPGReviewedEvolutionVersionsFreezeAndRollback(t *testing.T) {
	s, _ := testpg.Open(t)
	scope, in := seed(t, s)
	for n := 1; n <= 2; n++ {
		receipt := fmt.Sprint("receipt-", n)
		body, _ := json.Marshal(map[string]string{"state": "sent", "run_id": fmt.Sprint("delivered-", n)})
		at := in.At.Add(time.Duration(n) * time.Hour)
		if err := s.RecordFact(t.Context(), scope, storage.Fact{ID: receipt, Kind: "delivery", Body: body, At: at}); err != nil {
			t.Fatal(err)
		}
		feedback := storage.Input{ID: fmt.Sprint("feedback-input-", n), RunID: fmt.Sprint("feedback-run-", n), Text: "This concrete result helped", At: at.Add(time.Minute), FeedbackSource: receipt}
		state, err := s.Adopt(t.Context(), scope, feedback)
		if err != nil {
			t.Fatal(err)
		}
		c := storage.Change{Scope: scope, Expected: state.Revision, Action: "record_outcome", Source: receipt, Outcome: &storage.Outcome{FeedbackID: feedback.ID, Evidence: "L2", Summary: "Explicit linked reaction"}}
		if _, err = s.Apply(t.Context(), fmt.Sprint("outcome-", n), "digest", c); err != nil {
			t.Fatal(err)
		}
		state, _ = s.Read(t.Context(), scope)
		c.Expected = state.Revision
		if _, err = s.Apply(t.Context(), fmt.Sprint("duplicate-outcome-", n), "digest", c); err == nil {
			t.Fatal("same feedback applied twice")
		}
	}
	yes := true
	if _, err := s.Apply(t.Context(), "enable-evolution", "digest", storage.Change{Scope: scope, Expected: 1, Action: "set_evolution", Flag: &yes}); err != nil {
		t.Fatal(err)
	}
	c := storage.Change{Scope: scope, Expected: 2, Action: "adapt_persona", Adaptation: &affect.Adaptation{BaselineVersion: 1, Changes: map[string]float64{"directness": .55}, Evidence: []string{"outcome-1", "outcome-2"}, Reason: "Two separately confirmed responses"}}
	if _, err := s.Preview(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	roles, _ := s.Roles(t.Context(), scope.Space)
	if roles[0].Persona.EffectiveParameters().Directness != .5 {
		t.Fatal("preview activated adaptation")
	}
	if _, err := s.Apply(t.Context(), "adapt", "digest", c); err != nil {
		t.Fatal(err)
	}
	versions, err := s.Versions(t.Context(), scope, 0)
	if err != nil || len(versions) != 3 || versions[0].Persona.EffectiveParameters().Directness != .55 {
		t.Fatal(versions, err)
	}
	if _, err = s.Apply(t.Context(), "rollback", "digest", storage.Change{Scope: scope, Expected: 3, Action: "rollback_persona", ToVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(t.Context(), "freeze", "digest", storage.Change{Scope: scope, Expected: 4, Action: "freeze_persona", Flag: &yes}); err != nil {
		t.Fatal(err)
	}
	c.Expected = 5
	if _, err = s.Preview(t.Context(), c); err == nil {
		t.Fatal("frozen evolution accepted")
	}
}
