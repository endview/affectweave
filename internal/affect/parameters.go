package affect

import (
	"math"
	"strings"
)

// These are engineering controls for an AI character, not psychological scores.
type Parameters struct {
	Sensitivity        float64            `json:"sensitivity"`
	EmotionSensitivity map[string]float64 `json:"emotion_sensitivity,omitempty"`
	RecoverySpeed      float64            `json:"recovery_speed"`
	Expression         float64            `json:"expression"`
	Regulation         float64            `json:"regulation"`
	Directness         float64            `json:"directness"`
	Disclosure         float64            `json:"disclosure"`
	RepairPreference   float64            `json:"repair_preference"`
}

func NeutralParameters() Parameters {
	return Parameters{Sensitivity: .5, RecoverySpeed: .5, Expression: .5, Regulation: .5, Directness: .5, Disclosure: .5, RepairPreference: .5}
}
func (p Parameters) Values() map[string]float64 {
	m := map[string]float64{"sensitivity": p.Sensitivity, "recovery_speed": p.RecoverySpeed, "expression": p.Expression, "regulation": p.Regulation, "directness": p.Directness, "disclosure": p.Disclosure, "repair_preference": p.RepairPreference}
	for k, v := range p.EmotionSensitivity {
		m["sensitivity."+k] = v
	}
	return m
}
func (p *Parameters) Set(k string, v float64) error {
	if p == nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
		return ErrInvalid
	}
	next := *p
	next.EmotionSensitivity = cloneMap(p.EmotionSensitivity)
	if err := next.set(k, v); err != nil {
		return err
	}
	*p = next
	return nil
}

func (p *Parameters) set(k string, v float64) error {
	switch k {
	case "sensitivity":
		p.Sensitivity = v
	case "recovery_speed":
		p.RecoverySpeed = v
	case "expression":
		p.Expression = v
	case "regulation":
		p.Regulation = v
	case "directness":
		p.Directness = v
	case "disclosure":
		p.Disclosure = v
	case "repair_preference":
		p.RepairPreference = v
	default:
		if !strings.HasPrefix(k, "sensitivity.") || !oneOf(strings.TrimPrefix(k, "sensitivity."), emotionTypes) {
			return ErrInvalid
		}
		if p.EmotionSensitivity == nil {
			p.EmotionSensitivity = map[string]float64{}
		}
		p.EmotionSensitivity[strings.TrimPrefix(k, "sensitivity.")] = v
	}
	return p.Validate()
}

const emotionTypes = "joy|interest|hope|relief|gratitude|anger|fear|sadness|disappointment|guilt|shame|disgust"

func (p Parameters) Validate() error {
	if len(p.EmotionSensitivity) > 12 {
		return ErrInvalid
	}
	for k, v := range p.Values() {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return ErrInvalid
		}
		if strings.HasPrefix(k, "sensitivity.") && !oneOf(strings.TrimPrefix(k, "sensitivity."), emotionTypes) {
			return ErrInvalid
		}
	}
	return nil
}
func (p Persona) EffectiveParameters() Parameters {
	result := NeutralParameters()
	if p.Parameters != nil {
		result = *p.Parameters
	}
	if p.Evolution != nil {
		result = p.Evolution.Baseline
		result.EmotionSensitivity = cloneMap(result.EmotionSensitivity)
		for k, delta := range p.Evolution.Overlay {
			_ = result.Set(k, Clamp(result.Values()[k]+delta, 0, 1))
		}
	}
	return result
}
func cloneMap(m map[string]float64) map[string]float64 {
	if m == nil {
		return nil
	}
	n := map[string]float64{}
	for k, v := range m {
		n[k] = v
	}
	return n
}
func (p Parameters) Gain(emotion string) float64 {
	v := p.Sensitivity
	if x, ok := p.EmotionSensitivity[emotion]; ok {
		v = x
	}
	return .5 + v
}
func (p Parameters) Tau() float64 { return 3600 * math.Pow(2, 1-2*p.RecoverySpeed) }

type ExpressionHints struct {
	Clarify    float64 `json:"clarify"`
	Support    float64 `json:"support"`
	Explore    float64 `json:"explore"`
	Repair     float64 `json:"repair"`
	Boundary   float64 `json:"boundary"`
	LeaveSpace float64 `json:"leave_space"`
	Directness float64 `json:"directness"`
	Disclosure float64 `json:"disclosure"`
}

func ExpressionFor(s State) ExpressionHints {
	p := s.Persona.EffectiveParameters()
	levels := map[string]float64{}
	for _, e := range s.Emotions {
		if e.Active {
			levels[e.Type] = max(levels[e.Type], e.Intensity)
		}
	}
	strength := p.Expression * (1 - .5*p.Regulation)
	return ExpressionHints{Clarify: Clamp(levels["fear"]*strength, 0, 1), Support: Clamp((levels["gratitude"]+levels["sadness"])*strength, 0, 1), Explore: Clamp(levels["interest"]*strength, 0, 1), Repair: Clamp((levels["anger"]+levels["guilt"])*p.RepairPreference*strength, 0, 1), Boundary: Clamp(levels["anger"]*p.Directness*strength, 0, 1), LeaveSpace: Clamp(levels["sadness"]*p.Regulation*strength, 0, 1), Directness: p.Directness, Disclosure: p.Disclosure}
}

type Evolution struct {
	Enabled         bool               `json:"enabled"`
	Frozen          bool               `json:"frozen"`
	BaselineVersion int64              `json:"baseline_version"`
	Baseline        Parameters         `json:"baseline"`
	Overlay         map[string]float64 `json:"overlay"`
	Evidence        []string           `json:"evidence_refs,omitempty"`
}
type Adaptation struct {
	BaselineVersion int64              `json:"baseline_version"`
	Changes         map[string]float64 `json:"changes"`
	Evidence        []string           `json:"evidence_refs"`
	Reason          string             `json:"reason"`
}

// ReviewAdaptation is called only by the confirmed storage transaction. It
// changes numeric controls, never identity, boundaries or descriptive facts.
func ReviewAdaptation(p Persona, a Adaptation) (Persona, error) {
	e := p.Evolution
	if e == nil || !e.Enabled || e.Frozen || a.BaselineVersion != e.BaselineVersion || len(a.Changes) == 0 || len(a.Changes) > 8 || len(a.Evidence) < 2 || len(a.Evidence) > 16 || len(a.Reason) == 0 || len(a.Reason) > 1024 {
		return p, ErrInvalid
	}
	copy := *e
	copy.Overlay = cloneMap(e.Overlay)
	if copy.Overlay == nil {
		copy.Overlay = map[string]float64{}
	}
	current := p.EffectiveParameters()
	base := e.Baseline.Values()
	for k, v := range a.Changes {
		before, ok := current.Values()[k]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 || math.Abs(v-before) > .050000001 || math.Abs(v-base[k]) > .200000001 {
			return p, ErrInvalid
		}
		copy.Overlay[k] = v - base[k]
	}
	copy.Evidence = append([]string(nil), a.Evidence...)
	p.Evolution = &copy
	p.Version++
	return p, p.EffectiveParameters().Validate()
}
