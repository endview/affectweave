package affect

import (
	"encoding/json"
	"math"
	"sort"
)

const InteractionVersion = "bounded-1"

type Interactions struct {
	Enabled  bool    `json:"enabled"`
	Strength float64 `json:"strength"`
}
type interactionEdge struct {
	From, To string
	Weight   float64
}

var interactionRules = []interactionEdge{{"hope", "fear", -.08}, {"gratitude", "anger", -.08}, {"fear", "sadness", .06}}

// Every edge reads one immutable snapshot; cause and target must match. This
// stage runs only for a new source event, never on a read or the decay timer.
func couple(s *State) {
	setting := s.Persona.Interactions
	if setting == nil || !setting.Enabled || setting.Strength == 0 {
		return
	}
	before := Clone(*s)
	ids := make([]string, 0, len(before.Emotions))
	for id := range before.Emotions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	changes := map[string]float64{}
	for _, id := range ids {
		from := before.Emotions[id]
		if !from.Active {
			continue
		}
		for _, edge := range interactionRules {
			if edge.From != from.Type {
				continue
			}
			for _, toID := range ids {
				to := before.Emotions[toID]
				if to.Type == edge.To && to.CauseID == from.CauseID && to.Target == from.Target && to.Active {
					changes[toID] += edge.Weight * from.Intensity
				}
			}
		}
	}
	for _, id := range ids {
		e := s.Emotions[id]
		e.Intensity = Clamp(e.Intensity+Clamp(changes[id], -.10, .10)*setting.Strength, 0, 1)
		e.Active = e.Intensity >= .01
		s.Emotions[id] = e
	}
}

// Confirmed result transformation has a fresh source. It never infers an
// outcome from comforting text, silence, model generation or UNKNOWN.
func Transform(s *State, cause, event, result string) error {
	if cause == "" || event == "" || !oneOf(result, "resolved|failed") {
		return ErrInvalid
	}
	setting := s.Persona.Interactions
	if setting == nil || !setting.Enabled || setting.Strength == 0 {
		return nil
	}
	from, to := "fear", "relief"
	if result == "failed" {
		from, to = "hope", "disappointment"
	}
	ids := []string{}
	for id := range s.Emotions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		e := s.Emotions[id]
		if e.Type != from || e.CauseID != cause || !e.Active {
			continue
		}
		derived := "transform:" + event + ":" + id
		identity, _ := json.Marshal([]string{to, e.CauseID, e.Target})
		targetID := Hash(identity)
		target := s.Emotions[targetID]
		for _, source := range target.Sources {
			if source == derived {
				return nil
			}
		}
		amount := math.Min(.10, e.Intensity*.25) * setting.Strength
		e.Intensity -= amount
		e.Active = e.Intensity >= .01
		s.Emotions[id] = e
		if target.ID == "" {
			target = Emotion{ID: targetID, Type: to, CauseID: e.CauseID, Cause: e.Cause, Target: e.Target, Started: s.Updated, Updated: s.Updated, Confidence: e.Confidence, Sources: append([]string(nil), e.Sources...)}
		}
		target.Intensity = Clamp(target.Intensity+amount, 0, 1)
		target.Active = target.Intensity >= .01
		target.Sources = appendUnique(target.Sources, derived)
		s.Emotions[targetID] = target
	}
	return nil
}
