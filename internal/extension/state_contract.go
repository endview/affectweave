package extension

import (
	"encoding/json"
	"github.com/endview/affectweave/internal/affect"
	"github.com/endview/affectweave/internal/storage"
)

// The host envelope contains no character vocabulary. Only this adapter maps
// the extension-owned resource/payload to AffectWeave's existing data model.
type stateScope struct {
	Core     string `json:"core_id"`
	Space    string `json:"space_id"`
	Resource string `json:"resource,omitempty"`
	Actor    string `json:"actor_id"`
}

func (s stateScope) internal() affect.Scope {
	return affect.Scope{Core: s.Core, Space: s.Space, Role: s.Resource, Actor: s.Actor}
}

type stateChange struct {
	ID              string          `json:"id"`
	Scope           stateScope      `json:"scope"`
	Action          string          `json:"action"`
	Expected        int64           `json:"expected_revision"`
	Payload         json.RawMessage `json:"payload"`
	Contract        int             `json:"contract"`
	DeclarationHash string          `json:"declaration_hash"`
	Origin          *struct {
		RunID   string `json:"run_id"`
		InputID string `json:"input_id"`
	} `json:"origin,omitempty"`
}

func (wire stateChange) internal() (storage.Change, error) {
	if wire.ID != ID || wire.Contract != 2 || wire.DeclarationHash == "" {
		return storage.Change{}, affect.ErrInvalid
	}
	var payload struct {
		Persona    *affect.Persona    `json:"persona,omitempty"`
		Spaces     []string           `json:"spaces,omitempty"`
		Source     string             `json:"source_id,omitempty"`
		Proposal   *affect.Proposal   `json:"proposal,omitempty"`
		Decision   *affect.Decision   `json:"decision,omitempty"`
		Outcome    *storage.Outcome   `json:"outcome,omitempty"`
		MemoryID   string             `json:"memory_id,omitempty"`
		Adaptation *affect.Adaptation `json:"adaptation,omitempty"`
		Flag       *bool              `json:"flag,omitempty"`
		ToVersion  int64              `json:"to_version,omitempty"`
	}
	if strictJSON(wire.Payload, &payload) != nil {
		return storage.Change{}, affect.ErrInvalid
	}
	return storage.Change{Scope: wire.Scope.internal(), Action: wire.Action, Expected: wire.Expected, Persona: payload.Persona, Spaces: payload.Spaces, Source: payload.Source, Proposal: payload.Proposal, Decision: payload.Decision, Outcome: payload.Outcome, MemoryID: payload.MemoryID, Adaptation: payload.Adaptation, Flag: payload.Flag, ToVersion: payload.ToVersion}, nil
}
