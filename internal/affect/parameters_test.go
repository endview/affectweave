package affect

import (
	"math"
	"reflect"
	"testing"
	"time"
)

func TestParametersNeutralSensitivityAndRecovery(t *testing.T) {
	p := NeutralParameters()
	closeTo(t, p.Gain("fear"), 1)
	closeTo(t, p.Tau(), 3600)
	low, high := base(), base()
	a, b := p, p
	a.Sensitivity = 0
	b.Sensitivity = 1
	a.RecoverySpeed = 0
	b.RecoverySpeed = 1
	low.Persona.Parameters = &a
	high.Persona.Parameters = &b
	event := Proposal{Emotions: []EmotionDelta{{Type: "fear", CauseID: "cause", Cause: "specific event", Delta: .1}}}
	if err := Apply(&low, event, "one", low.Updated, 1); err != nil {
		t.Fatal(err)
	}
	if err := Apply(&high, event, "one", high.Updated, 1); err != nil {
		t.Fatal(err)
	}
	for id, e := range low.Emotions {
		if high.Emotions[id].Intensity <= e.Intensity {
			t.Fatal("sensitivity has no effect")
		}
	}
	initial := Clone(high)
	if err := Apply(&high, event, "one", high.Updated, 1); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(initial, high) {
		t.Fatal("duplicate event changed state")
	}
	Advance(&low, low.Updated.Add(time.Hour))
	Advance(&high, high.Updated.Add(time.Hour))
	for id, e := range low.Emotions {
		closeTo(t, high.Emotions[id].Intensity, .15*math.Exp(-2))
		closeTo(t, e.Intensity, .05*math.Exp(-.5))
	}
}
func TestInteractionsCauseIsolationNoReadAmplificationAndTransformation(t *testing.T) {
	a := base()
	a.Persona.Interactions = &Interactions{Enabled: true, Strength: 1}
	p := Proposal{Emotions: []EmotionDelta{{Type: "hope", CauseID: "a", Cause: "a", Target: "x", Delta: .2}, {Type: "fear", CauseID: "a", Cause: "a", Target: "x", Delta: .2}, {Type: "fear", CauseID: "b", Cause: "b", Target: "x", Delta: .2}}}
	if err := Apply(&a, p, "one", a.Updated, 1); err != nil {
		t.Fatal(err)
	}
	for _, e := range a.Emotions {
		if e.Type == "fear" && e.CauseID == "a" {
			closeTo(t, e.Intensity, .184)
		}
		if e.Type == "fear" && e.CauseID == "b" {
			closeTo(t, e.Intensity, .2)
		}
	}
	before := Clone(a)
	Advance(&a, a.Updated)
	if !reflect.DeepEqual(before, a) {
		t.Fatal("read repeated interaction")
	}
	if err := Transform(&a, "a", "real-result", "resolved"); err != nil {
		t.Fatal(err)
	}
	before = Clone(a)
	_ = Transform(&a, "a", "real-result", "resolved")
	if !reflect.DeepEqual(before, a) {
		t.Fatal("transformation repeated")
	}
}
func TestReviewedPersonaLimitsAndFreeze(t *testing.T) {
	p := base().Persona
	p.Evolution = &Evolution{Enabled: true, BaselineVersion: 1, Baseline: NeutralParameters(), Overlay: map[string]float64{}}
	a := Adaptation{BaselineVersion: 1, Changes: map[string]float64{"directness": .55}, Evidence: []string{"real-1", "real-2"}, Reason: "Two independent outcomes"}
	q, err := ReviewAdaptation(p, a)
	if err != nil || q.EffectiveParameters().Directness != .55 || p.EffectiveParameters().Directness != .5 {
		t.Fatal(q, err)
	}
	a.Changes["directness"] = .7
	if _, err = ReviewAdaptation(q, a); err == nil {
		t.Fatal("unbounded adaptation")
	}
	q.Evolution.Frozen = true
	a.Changes["directness"] = .6
	if _, err = ReviewAdaptation(q, a); err == nil {
		t.Fatal("frozen adaptation")
	}
}

func TestInvalidParameterUpdateLeavesOriginalUntouched(t *testing.T) {
	p := NeutralParameters()
	p.EmotionSensitivity = map[string]float64{"fear": .6}
	before := p
	before.EmotionSensitivity = cloneMap(p.EmotionSensitivity)
	for _, change := range []struct {
		key   string
		value float64
	}{{"directness", 2}, {"sensitivity.fear", math.NaN()}, {"unknown", .4}} {
		if p.Set(change.key, change.value) == nil || !reflect.DeepEqual(before, p) {
			t.Fatal("invalid update mutated parameters", change.key, p)
		}
	}
}
