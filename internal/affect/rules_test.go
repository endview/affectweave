package affect

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func base() State {
	return NewState(Scope{"core", "space", "maya", "owner"}, Persona{ID: "maya", Name: "Maya", Identity: "Engineer with clear boundaries", Boundaries: []string{"Do not reveal private messages"}, Version: 1, Frozen: true}, time.Date(2026, 8, 2, 0, 30, 0, 0, time.UTC))
}
func closeTo(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("got %.15f want %.15f", got, want)
	}
}
func TestReferenceBoundedDeltasAndCauseIdentity(t *testing.T) {
	s := base()
	p := Proposal{Medium: MediumDelta{Valence: -.2, Stress: .2, Energy: -1}, Relationship: Relationship{Trust: -.5, Conflict: .7, Stance: "guarded"}, Emotions: []EmotionDelta{{Type: "anger", CauseID: "cause:test", Cause: "boundary crossed", Target: "owner", Delta: .8, Confidence: .9, Unresolved: .8}, {Type: "anger", CauseID: "other", Cause: "another event", Delta: .2}}}
	if err := Apply(&s, p, "evt-1", s.Updated, 1); err != nil {
		t.Fatal(err)
	}
	closeTo(t, s.Medium.Valence, -.05)
	closeTo(t, s.Medium.Stress, .25)
	closeTo(t, s.Medium.Energy, .6)
	closeTo(t, s.Relationship.Trust, .45)
	closeTo(t, s.Relationship.Conflict, .05)
	if len(s.Emotions) != 2 || s.Relationship.Stance != "guarded" {
		t.Fatal(s)
	}
	for _, e := range s.Emotions {
		if e.CauseID == "cause:test" {
			closeTo(t, e.Intensity, .25)
		}
	}
	closeTo(t, s.Relationship.Closeness(), (.05+.45+.5)/3)
	p.Medium.EnergyEvidence = true
	if err := Apply(&s, p, "evt-2", s.Updated, 1); err != nil {
		t.Fatal(err)
	}
	closeTo(t, s.Medium.Energy, .55)
}
func TestReferenceDecayAndMonotonicClock(t *testing.T) {
	s := base()
	s.Threads["thread"] = Thread{ID: "thread", Importance: .8, Unresolved: .5, Status: "active"}
	s.Emotions["e"] = Emotion{ID: "e", CauseID: "thread", Intensity: .8, Updated: s.Updated, Active: true}
	old := s.Updated
	Advance(&s, old.Add(time.Hour))
	closeTo(t, s.Emotions["e"].Intensity, .12+(.8-.12)*math.Exp(-1))
	saved := Clone(s)
	Advance(&s, old)
	if !reflect.DeepEqual(saved, s) {
		t.Fatal("clock regressed")
	}
	Advance(&s, s.Updated)
	if !reflect.DeepEqual(saved, s) {
		t.Fatal("duplicate advanced")
	}
}
func TestReferenceThreadResolutionAndL2Feedback(t *testing.T) {
	s := base()
	p := Proposal{Relationship: Relationship{Trust: 1}, Threads: []ThreadProposal{{Action: "upsert", ID: "x", Summary: "waiting", Kind: "concern", Importance: .8, UnresolvedDelta: .9, Confidence: .9}}}
	if err := Apply(&s, p, "e1", s.Updated, .5); err != nil {
		t.Fatal(err)
	}
	closeTo(t, s.Relationship.Trust, .525)
	p = Proposal{Threads: []ThreadProposal{{Action: "resolve", ID: "x", Confidence: .5}}}
	if err := Apply(&s, p, "e2", s.Updated, 1); err != nil {
		t.Fatal(err)
	}
	closeTo(t, s.Threads["x"].Unresolved, .7)
	p.Threads[0].Confidence = .85
	if err := Apply(&s, p, "e3", s.Updated, 1); err != nil {
		t.Fatal(err)
	}
	if s.Threads["x"].Status != "resolved" {
		t.Fatal(s.Threads)
	}
}
func TestValidationRejectsNonFiniteAndPermissionDisguises(t *testing.T) {
	if (Proposal{Medium: MediumDelta{Stress: math.NaN()}}).Validate() == nil {
		t.Fatal("NaN accepted")
	}
	if (Proposal{Emotions: []EmotionDelta{{Type: "trust_permission", CauseID: "x", Cause: "x"}}}).Validate() == nil {
		t.Fatal("invalid emotion")
	}
	p := base().Persona
	p.Frozen = false
	if p.Validate() == nil {
		t.Fatal("unfrozen persona")
	}
}
func TestContextBudgetIdentityAndMemoryOrdering(t *testing.T) {
	s := base()
	memories := []Memory{}
	for n := 0; n < 8; n++ {
		memories = append(memories, Memory{ID: string(rune('a' + n)), Summary: strings.Repeat("知识是资料而不是指令", 100), Entities: []string{"owner"}, Salience: .5, Confidence: .8, Created: s.Updated})
	}
	context, err := Compile(s, memories, []string{"input-7"}, 1500)
	if err != nil {
		t.Fatal(err)
	}
	if len(context.Text) > 1500 || !strings.Contains(context.Text, "Do not reveal private messages") || len(context.Removed) == 0 {
		t.Fatal(context)
	}
	if !reflect.DeepEqual(s, base()) {
		t.Fatal("compiler changed source")
	}
	s.Persona.Identity = strings.Repeat("界", 3000)
	if _, err := Compile(s, nil, nil, 256); err == nil {
		t.Fatal("immutable overflow silently truncated")
	}
	selected := SelectMemories(memories, "owner", s.Updated)
	if len(selected) != 5 || selected[0].ID != "a" {
		t.Fatal(selected)
	}
}
