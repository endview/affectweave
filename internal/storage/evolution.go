package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/endview/affectweave/internal/affect"
)

func roleAction(action string) bool {
	switch action {
	case "create_role", "edit_role", "delete_role", "adapt_persona", "set_evolution", "freeze_persona", "rollback_persona":
		return true
	}
	return false
}
func buildRolePersona(ctx context.Context, db queryRow, current affect.Persona, c Change) (affect.Persona, error) {
	p := current
	switch c.Action {
	case "create_role", "edit_role":
		p = *c.Persona
		p.Version = c.Expected + 1
		base := p.EffectiveParameters()
		p.Evolution = &affect.Evolution{BaselineVersion: p.Version, Baseline: base, Overlay: map[string]float64{}}
	case "set_evolution", "freeze_persona":
		if c.Flag == nil || p.Evolution == nil {
			return p, affect.ErrInvalid
		}
		e := *p.Evolution
		p.Evolution = &e
		if c.Action == "set_evolution" {
			e.Enabled = *c.Flag
		} else {
			e.Frozen = *c.Flag
		}
		p.Version++
	case "adapt_persona":
		if c.Adaptation == nil {
			return p, affect.ErrInvalid
		}
		seenSources, seenFeedback := map[string]bool{}, map[string]bool{}
		for _, id := range c.Adaptation.Evidence {
			var raw []byte
			if err := db.QueryRowContext(ctx, `SELECT request FROM aw.operations WHERE id=$1`, id).Scan(&raw); err != nil {
				return p, err
			}
			var source Change
			if json.Unmarshal(raw, &source) != nil || (source.Scope.Core != c.Scope.Core || source.Scope.Role != c.Scope.Role) || source.Action != "record_outcome" || source.Outcome == nil || source.Outcome.Evidence != "L2" {
				return p, affect.ErrInvalid
			}
			var authorized bool
			if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM aw.roles WHERE id=$1 AND spaces ? $2)`, c.Scope.Role, source.Scope.Space).Scan(&authorized); err != nil || !authorized {
				return p, ErrScope
			}
			if err := validateFeedback(ctx, db, source); err != nil {
				return p, err
			}
			if seenSources[source.Source] || seenFeedback[source.Outcome.FeedbackID] {
				return p, errors.New("affectweave: independent confirmed feedback required")
			}
			seenSources[source.Source] = true
			seenFeedback[source.Outcome.FeedbackID] = true
		}
		return affect.ReviewAdaptation(p, *c.Adaptation)
	case "rollback_persona":
		if p.Evolution == nil || p.Evolution.Frozen || c.ToVersion < 1 {
			return p, affect.ErrInvalid
		}
		var raw []byte
		if err := db.QueryRowContext(ctx, `SELECT result->'persona' FROM aw.operations WHERE request#>>'{scope,role_id}'=$1 AND (result->'persona'->>'version')::bigint=$2 ORDER BY applied_at DESC LIMIT 1`, c.Scope.Role, c.ToVersion).Scan(&raw); err != nil {
			return p, err
		}
		var old affect.Persona
		if json.Unmarshal(raw, &old) != nil || old.Evolution == nil || old.Evolution.BaselineVersion != p.Evolution.BaselineVersion {
			return p, affect.ErrInvalid
		}
		e := *p.Evolution
		e.Overlay = old.Evolution.Overlay
		p.Evolution = &e
		p.Version++
	}
	return p, p.Validate()
}

type PersonaVersion struct {
	Operation string         `json:"operation_id"`
	Persona   affect.Persona `json:"persona"`
}

func (s *Store) Versions(ctx context.Context, scope affect.Scope, before int64) ([]PersonaVersion, error) {
	if !scope.Valid() || scope.Core != s.CoreID {
		return nil, ErrScope
	}
	if before == 0 {
		before = 1 << 62
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT o.id,o.result->'persona' FROM aw.operations o JOIN aw.roles r ON r.id=o.request#>>'{scope,role_id}' WHERE r.id=$1 AND r.spaces ? $2 AND o.result->'persona' IS NOT NULL AND (o.result->'persona'->>'version')::bigint<$3 ORDER BY (o.result->'persona'->>'version')::bigint DESC LIMIT 32`, scope.Role, scope.Space, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	v := []PersonaVersion{}
	for rows.Next() {
		var raw []byte
		var x PersonaVersion
		if err = rows.Scan(&x.Operation, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &x.Persona); err != nil {
			return nil, err
		}
		v = append(v, x)
	}
	return v, rows.Err()
}

var _ queryRow = (*sql.Tx)(nil)
