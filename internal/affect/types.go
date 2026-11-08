// Package affect implements AffectWeave's deterministic character rules.
// It has no host, model, database or permission dependencies.
package affect

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

const Version = "1.6.0-dev.2"

var ErrInvalid = errors.New("affectweave: invalid state or proposal")

type Scope struct {
	Core  string `json:"core_id"`
	Space string `json:"space_id"`
	Role  string `json:"role_id"`
	Actor string `json:"actor_id"`
}

func (s Scope) Valid() bool {
	for _, v := range []string{s.Core, s.Space, s.Role, s.Actor} {
		if v == "" || len(v) > 256 || strings.ContainsAny(v, "\x00\r\n") {
			return false
		}
	}
	return true
}
func (s Scope) Key() string { b, _ := json.Marshal(s); return Hash(b) }
func Hash(b []byte) string  { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

type Persona struct {
	Parameters     *Parameters        `json:"parameters,omitempty"`
	Interactions   *Interactions      `json:"interactions,omitempty"`
	Evolution      *Evolution         `json:"evolution,omitempty"`
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Identity       string             `json:"identity_summary"`
	Values         map[string]float64 `json:"values,omitempty"`
	Traits         map[string]float64 `json:"stable_traits,omitempty"`
	Boundaries     []string           `json:"boundaries,omitempty"`
	Preferences    []string           `json:"preferences,omitempty"`
	Contradictions []string           `json:"contradictions,omitempty"`
	Voice          []string           `json:"voice_notes,omitempty"`
	Version        int64              `json:"version"`
	Frozen         bool               `json:"frozen"`
}
type Medium struct {
	Valence    float64 `json:"valence"`
	Activation float64 `json:"activation"`
	Energy     float64 `json:"energy"`
	Stress     float64 `json:"stress"`
}
type Emotion struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	CauseID    string    `json:"cause_id"`
	Cause      string    `json:"cause_summary"`
	Target     string    `json:"target_id,omitempty"`
	Intensity  float64   `json:"felt_intensity"`
	Confidence float64   `json:"confidence"`
	Unresolved float64   `json:"unresolvedness"`
	Sources    []string  `json:"source_event_ids"`
	Started    time.Time `json:"started_at"`
	Updated    time.Time `json:"updated_at"`
	Active     bool      `json:"active"`
}
type Relationship struct {
	Familiarity float64 `json:"familiarity"`
	Trust       float64 `json:"trust"`
	Warmth      float64 `json:"warmth"`
	Reliability float64 `json:"reliability"`
	Conflict    float64 `json:"conflict"`
	Stance      string  `json:"stance"`
}

func (r Relationship) Closeness() float64 { return Clamp((r.Familiarity+r.Trust+r.Warmth)/3, 0, 1) }

type Thread struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Summary    string    `json:"summary"`
	Importance float64   `json:"importance"`
	Unresolved float64   `json:"unresolvedness"`
	Entities   []string  `json:"related_entities,omitempty"`
	Sources    []string  `json:"source_event_ids"`
	Completion string    `json:"completion_condition,omitempty"`
	Status     string    `json:"status"`
	Created    time.Time `json:"created_at"`
	Updated    time.Time `json:"updated_at"`
}
type State struct {
	Scope         Scope              `json:"scope"`
	Revision      int64              `json:"revision"`
	Persona       Persona            `json:"persona"`
	Medium        Medium             `json:"medium"`
	Relationship  Relationship       `json:"relationship"`
	Emotions      map[string]Emotion `json:"emotions"`
	Threads       map[string]Thread  `json:"threads"`
	Updated       time.Time          `json:"updated_at"`
	AppliedEvents []string           `json:"applied_events,omitempty"`
}

func NewState(scope Scope, p Persona, now time.Time) State {
	return State{Scope: scope, Revision: 1, Persona: p, Medium: Medium{Activation: .3, Energy: .6, Stress: .2}, Relationship: Relationship{Familiarity: .05, Trust: .5, Warmth: .5, Reliability: .5, Stance: "open"}, Emotions: map[string]Emotion{}, Threads: map[string]Thread{}, Updated: now}
}

type MediumDelta struct {
	Valence        float64 `json:"valence"`
	Activation     float64 `json:"activation"`
	Energy         float64 `json:"energy"`
	Stress         float64 `json:"stress"`
	EnergyEvidence bool    `json:"energy_evidence"`
}
type EmotionDelta struct {
	Type       string  `json:"emotion"`
	CauseID    string  `json:"cause_id"`
	Cause      string  `json:"cause_summary"`
	Target     string  `json:"target_id,omitempty"`
	Delta      float64 `json:"delta"`
	Confidence float64 `json:"confidence"`
	Unresolved float64 `json:"unresolvedness"`
}
type ThreadProposal struct {
	Result          string   `json:"result,omitempty"`
	Action          string   `json:"action"`
	ID              string   `json:"thread_id"`
	Kind            string   `json:"kind"`
	Summary         string   `json:"summary"`
	Importance      float64  `json:"importance"`
	UnresolvedDelta float64  `json:"unresolvedness_delta"`
	Completion      string   `json:"completion_condition,omitempty"`
	Entities        []string `json:"related_entities,omitempty"`
	Confidence      float64  `json:"confidence"`
}
type Memory struct {
	ID         string    `json:"id"`
	Summary    string    `json:"summary"`
	Sources    []string  `json:"event_ids"`
	Entities   []string  `json:"related_entities"`
	Salience   float64   `json:"salience"`
	Confidence float64   `json:"confidence"`
	Evidence   string    `json:"evidence_level"`
	Created    time.Time `json:"created_at"`
}
type Intent struct {
	Label      string   `json:"label"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence,omitempty"`
}
type Proposal struct {
	Intents      []Intent         `json:"intent_hypotheses,omitempty"`
	Focus        []string         `json:"focus,omitempty"`
	Medium       MediumDelta      `json:"medium_delta"`
	Emotions     []EmotionDelta   `json:"emotion_deltas,omitempty"`
	Relationship Relationship     `json:"relationship_delta"`
	Threads      []ThreadProposal `json:"thread_proposals,omitempty"`
	Memories     []Memory         `json:"memory_candidates,omitempty"`
	Reasons      []string         `json:"reason_codes,omitempty"`
}
type Speech struct {
	Sentences  int     `json:"max_sentences"`
	Tokens     int     `json:"max_tokens"`
	Question   bool    `json:"question_allowed"`
	Disclosure float64 `json:"self_disclosure"`
}
type Decision struct {
	Goal       string   `json:"goal"`
	Act        string   `json:"act"`
	Strength   float64  `json:"strength"`
	Stance     string   `json:"relationship_stance"`
	Speech     Speech   `json:"speech_budget"`
	Disclosure float64  `json:"disclosure_level"`
	LeaveSpace bool     `json:"leave_space"`
	Reasons    []string `json:"reason_codes,omitempty"`
}
type Output struct {
	Proposal Proposal `json:"state_proposal"`
	Decision Decision `json:"decision"`
	Reply    string   `json:"reply"`
}

func Clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func oneOf(v, choices string) bool    { return strings.Contains("|"+choices+"|", "|"+v+"|") }
func (p Persona) Validate() error {
	if p.Parameters != nil {
		if err := p.Parameters.Validate(); err != nil {
			return err
		}
	}
	if e := p.Evolution; e != nil {
		if e.BaselineVersion < 1 || len(e.Overlay) > 20 || len(e.Evidence) > 16 {
			return ErrInvalid
		}
		if err := e.Baseline.Validate(); err != nil {
			return err
		}
		base := e.Baseline.Values()
		for key, delta := range e.Overlay {
			value, ok := base[key]
			if !ok || math.IsNaN(delta) || math.IsInf(delta, 0) || math.Abs(delta) > .200000001 || value+delta < 0 || value+delta > 1 {
				return ErrInvalid
			}
		}
		if err := p.EffectiveParameters().Validate(); err != nil {
			return err
		}
	}

	if p.Interactions != nil && (p.Interactions.Strength < 0 || p.Interactions.Strength > 1) {
		return ErrInvalid
	}
	if p.ID == "" || p.Name == "" || p.Identity == "" || !p.Frozen || len(p.Identity) > 8192 || len(p.Boundaries) > 32 || len(p.Values) > 32 || len(p.Traits) > 32 {
		return ErrInvalid
	}
	if len(p.ID) > 128 || len(p.Name) > 256 || len(p.Preferences) > 32 || len(p.Contradictions) > 32 || len(p.Voice) > 32 {
		return ErrInvalid
	}
	for _, list := range [][]string{p.Boundaries, p.Preferences, p.Contradictions, p.Voice} {
		for _, text := range list {
			if len(text) > 4096 {
				return ErrInvalid
			}
		}
	}
	for _, values := range []map[string]float64{p.Values, p.Traits} {
		for _, v := range values {
			if v < 0 || v > 1 {
				return ErrInvalid
			}
		}
	}
	return finiteJSON(p)
}
func (o *Output) Validate() error {
	if strings.TrimSpace(o.Reply) == "" || len(o.Reply) > 65536 || !oneOf(o.Decision.Goal, "connect|understand|share|solve|protect_self|repair|create_distance|end_interaction") || !oneOf(o.Decision.Act, "answer|acknowledge|self_disclose|ask_or_clarify|support|repair_or_boundary|assert|disengage|topic_shift|leave_space") || !oneOf(o.Decision.Stance, "open|friendly|cautious|guarded|defensive|distant|disengaged") {
		return ErrInvalid
	}
	if err := o.Proposal.Validate(); err != nil {
		return err
	}
	o.Decision.Strength = Clamp(o.Decision.Strength, 0, 1)
	o.Decision.Disclosure = Clamp(o.Decision.Disclosure, 0, 1)
	o.Decision.Speech.Sentences = max(1, min(12, o.Decision.Speech.Sentences))
	o.Decision.Speech.Tokens = max(8, min(800, o.Decision.Speech.Tokens))
	o.Decision.Speech.Disclosure = Clamp(o.Decision.Speech.Disclosure, 0, 1)
	return finiteJSON(o)
}
func finiteJSON(v any) error {
	_, err := json.Marshal(v)
	if err != nil {
		return ErrInvalid
	}
	return nil
}
func (p Proposal) Validate() error {
	if len(p.Emotions) > 32 || len(p.Threads) > 32 || len(p.Memories) > 16 || len(p.Intents) > 16 {
		return ErrInvalid
	}
	if p.Relationship.Stance != "" && !oneOf(p.Relationship.Stance, "open|friendly|cautious|guarded|defensive|distant|disengaged") {
		return ErrInvalid
	}
	for _, e := range p.Emotions {
		if !oneOf(e.Type, "joy|interest|hope|relief|gratitude|anger|fear|sadness|disappointment|guilt|shame|disgust") || e.CauseID == "" || e.Cause == "" || len(e.Cause) > 4096 {
			return ErrInvalid
		}
	}
	for _, t := range p.Threads {
		if t.Result != "" && !oneOf(t.Result, "resolved|failed") {
			return ErrInvalid
		}
		if t.ID == "" || !oneOf(t.Action, "upsert|resolve|abandon") || len(t.Summary) > 4096 || len(t.Entities) > 32 {
			return ErrInvalid
		}
	}
	for _, m := range p.Memories {
		if strings.TrimSpace(m.Summary) == "" || len(m.Summary) > 4096 || len(m.Entities) > 32 {
			return ErrInvalid
		}
	}
	return finiteJSON(p)
}
