package storage

import (
	"context"
	"github.com/endview/affectweave/internal/affect"
)

type PersonaEvidence struct {
	ID       string `json:"id"`
	Summary  string `json:"summary"`
	Source   string `json:"source_id"`
	Feedback string `json:"feedback_input_id"`
}

func (s *Store) PersonaEvidence(ctx context.Context, scope affect.Scope) ([]PersonaEvidence, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,request#>>'{outcome,summary}',request->>'source_id',request#>>'{outcome,feedback_input_id}' FROM aw.operations WHERE request->>'action'='record_outcome' AND request#>>'{scope,core_id}'=$1 AND request#>>'{scope,space_id}'=$2 AND request#>>'{scope,role_id}'=$3 AND request#>>'{scope,actor_id}'=$4 ORDER BY applied_at DESC,id DESC LIMIT 32`, scope.Core, scope.Space, scope.Role, scope.Actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PersonaEvidence{}
	for rows.Next() {
		var e PersonaEvidence
		if err = rows.Scan(&e.ID, &e.Summary, &e.Source, &e.Feedback); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
