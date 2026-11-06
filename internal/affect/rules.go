package affect

import (
	"encoding/json"
	"math"
	"slices"
	"sort"
	"time"
)

// Clone makes proposals and context compilation independent of saved state.
func Clone(s State) State {
	b, _ := json.Marshal(s)
	var copy State
	_ = json.Unmarshal(b, &copy)
	return copy
}
func appendUnique(values []string, v string) []string {
	if !slices.Contains(values, v) {
		values = append(values, v)
	}
	return values
}

// Advance advances logical time monotonically. A repeated context read never
// calls this function: the repository freezes a snapshot per adopted input.
func Advance(s *State, now time.Time) {
	if !now.After(s.Updated) {
		return
	}
	for id, e := range s.Emotions {
		elapsed := max(0, now.Sub(e.Updated).Seconds())
		residual := 0.0
		if t, ok := s.Threads[e.CauseID]; ok && t.Status == "active" {
			residual = Clamp(t.Importance*t.Unresolved*.30, 0, 1)
			e.Unresolved = t.Unresolved
		} else {
			e.Unresolved = 0
		}
		e.Intensity = Clamp(residual+(e.Intensity-residual)*math.Exp(-elapsed/3600), 0, 1)
		e.Active = e.Intensity >= .01
		e.Updated = now
		s.Emotions[id] = e
	}
	s.Updated = now
}

// Apply runs only after a host-confirmed state operation. Limits match the
// fixed Python reference; no preference is inferred from free user text.
func Apply(s *State, p Proposal, event string, now time.Time, multiplier float64) error {
	next := Clone(*s)
	if err := applyInPlace(&next, p, event, now, multiplier); err != nil {
		return err
	}
	*s = next
	return nil
}
func applyInPlace(s *State, p Proposal, event string, now time.Time, multiplier float64) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if event == "" || now.Before(s.Updated) || multiplier <= 0 || multiplier > 1 {
		return ErrInvalid
	}
	m := p.Medium
	s.Medium.Valence = Clamp(s.Medium.Valence+Clamp(m.Valence, -.05, .05), -1, 1)
	s.Medium.Activation = Clamp(s.Medium.Activation+Clamp(m.Activation, -.05, .05), 0, 1)
	s.Medium.Stress = Clamp(s.Medium.Stress+Clamp(m.Stress, -.05, .05), 0, 1)
	if m.EnergyEvidence {
		s.Medium.Energy = Clamp(s.Medium.Energy+Clamp(m.Energy, -.05, .05), 0, 1)
	}
	for _, d := range p.Emotions {
		identity, _ := json.Marshal([]string{d.Type, d.CauseID, d.Target})
		id := Hash(identity)
		e, exists := s.Emotions[id]
		delta := Clamp(d.Delta*multiplier, -.25, .25)
		if !exists {
			if delta <= 0 {
				continue
			}
			e = Emotion{ID: id, Type: d.Type, CauseID: d.CauseID, Target: d.Target, Started: now}
		}
		e.Intensity = Clamp(e.Intensity+delta, 0, 1)
		e.Confidence = max(e.Confidence, Clamp(d.Confidence, 0, 1))
		e.Unresolved = Clamp(d.Unresolved, 0, 1)
		e.Cause = d.Cause
		e.Sources = appendUnique(e.Sources, event)
		e.Active = e.Intensity >= .01
		e.Updated = now
		// Keep current provenance bounded; the immutable event ledger retains all facts.
		if len(e.Sources) > 32 {
			e.Sources = e.Sources[len(e.Sources)-32:]
		}
		s.Emotions[id] = e
	}
	r, d := &s.Relationship, p.Relationship
	fields := []*float64{&r.Familiarity, &r.Trust, &r.Warmth, &r.Reliability, &r.Conflict}
	incoming := []float64{d.Familiarity, d.Trust, d.Warmth, d.Reliability, d.Conflict}
	for n, f := range fields {
		*f = Clamp(*f+Clamp(incoming[n]*multiplier, -.05*multiplier, .05*multiplier), 0, 1)
	}
	if d.Stance != "" {
		r.Stance = d.Stance
	}
	for _, d := range p.Threads {
		t, exists := s.Threads[d.ID]
		switch d.Action {
		case "upsert":
			if !exists {
				if d.Confidence < .55 || d.Summary == "" {
					continue
				}
				t = Thread{ID: d.ID, Kind: d.Kind, Summary: d.Summary, Importance: Clamp(d.Importance, 0, 1), Unresolved: Clamp(max(.10, d.UnresolvedDelta), 0, 1), Status: "active", Created: now, Completion: d.Completion}
			} else {
				t.Unresolved = Clamp(t.Unresolved+Clamp(d.UnresolvedDelta, -.20, .20), 0, 1)
				if d.Summary != "" {
					t.Summary = d.Summary
				}
				t.Importance = max(t.Importance, Clamp(d.Importance, 0, 1))
			}
			for _, v := range append(d.Entities, s.Scope.Actor) {
				t.Entities = appendUnique(t.Entities, v)
			}
		case "resolve":
			if !exists {
				continue
			}
			if d.Confidence >= .85 {
				t.Status = "resolved"
				t.Unresolved = 0
			} else {
				t.Unresolved = Clamp(t.Unresolved-.20, 0, 1)
			}
		case "abandon":
			if !exists {
				continue
			}
			if d.Confidence >= .85 {
				t.Status = "abandoned"
				t.Unresolved = 0
			}
		}
		t.Sources = appendUnique(t.Sources, event)
		if len(t.Sources) > 32 {
			t.Sources = t.Sources[len(t.Sources)-32:]
		}
		t.Updated = now
		s.Threads[d.ID] = t
	}
	if len(s.Emotions) > 128 || len(s.Threads) > 128 {
		return ErrInvalid
	}
	s.Updated = now
	return nil
}

func SelectMemories(all []Memory, actor string, now time.Time) []Memory {
	score := func(m Memory) float64 {
		match := 0.0
		if slices.Contains(m.Entities, actor) {
			match = 1
		}
		age := max(0, now.Sub(m.Created).Hours()/24)
		return 1.5*match + .6*m.Salience + .4*m.Confidence + .3*math.Exp(-age/14)
	}
	result := slices.Clone(all)
	sort.Slice(result, func(i, j int) bool {
		a, b := score(result[i]), score(result[j])
		if a != b {
			return a > b
		}
		if !result[i].Created.Equal(result[j].Created) {
			return result[i].Created.After(result[j].Created)
		}
		return result[i].ID < result[j].ID
	})
	return result[:min(5, len(result))]
}
