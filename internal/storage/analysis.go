package storage

import (
	"context"
	"encoding/json"
	"github.com/endview/affectweave/internal/affect"
)

func (s *Store) ClaimAnalysis(ctx context.Context, scope affect.Scope, id string) (FrozenInput, bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return FrozenInput{}, false, err
	}
	defer tx.Rollback()
	var raw []byte
	var frozen FrozenInput
	if err = tx.QueryRowContext(ctx, `SELECT context_snapshot FROM aw.inputs WHERE scope=$1 AND input_id=$2 FOR UPDATE`, scope.Key(), id).Scan(&raw); err != nil {
		return frozen, false, err
	}
	if err = json.Unmarshal(raw, &frozen); err != nil {
		return frozen, false, err
	}
	claimed := frozen.Analysis == "pending"
	if claimed {
		frozen.Analysis = "started"
	} else if frozen.Analysis == "started" {
		frozen.Analysis = "unknown"
		frozen.Detail = "previous analysis did not settle; frozen confirmed-only fallback"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE aw.inputs SET context_snapshot=$1 WHERE scope=$2 AND input_id=$3`, string(encode(frozen)), scope.Key(), id); err != nil {
		return frozen, false, err
	}
	return frozen, claimed, tx.Commit()
}
func (s *Store) FinishAnalysis(ctx context.Context, scope affect.Scope, id string, f FrozenInput) (FrozenInput, error) {
	if f.Analysis != "complete" && f.Analysis != "failed" && f.Analysis != "unknown" {
		return FrozenInput{}, affect.ErrInvalid
	}
	r, err := s.DB.ExecContext(ctx, `UPDATE aw.inputs SET context_snapshot=$1 WHERE scope=$2 AND input_id=$3 AND context_snapshot->>'analysis'='started'`, string(encode(f)), scope.Key(), id)
	if err != nil {
		return FrozenInput{}, err
	}
	n, _ := r.RowsAffected()
	if n == 1 {
		return f, nil
	}
	var raw []byte
	if err = s.DB.QueryRowContext(ctx, `SELECT context_snapshot FROM aw.inputs WHERE scope=$1 AND input_id=$2`, scope.Key(), id).Scan(&raw); err != nil {
		return FrozenInput{}, err
	}
	err = json.Unmarshal(raw, &f)
	return f, err
}
func (s *Store) Analyses(ctx context.Context, scope affect.Scope) ([]FrozenInput, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT jsonb_build_object('input',jsonb_build_object('id',input_id,'run_id',run_id,'occurred_at',occurred_at,'feedback_source',context_snapshot#>>'{input,feedback_source}'),'analysis',context_snapshot->'analysis','detail',context_snapshot->'detail','draft',context_snapshot->'draft','feedback_draft',context_snapshot->'feedback_draft') FROM aw.inputs WHERE scope=$1 ORDER BY occurred_at DESC,input_id DESC LIMIT 20`, scope.Key())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []FrozenInput{}
	for rows.Next() {
		var raw []byte
		var f FrozenInput
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		if f.Analysis != "disabled" {
			result = append(result, f)
		}
	}
	return result, rows.Err()
}
