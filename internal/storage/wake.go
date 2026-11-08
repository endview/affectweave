package storage

import (
	"context"
	"github.com/endview/affectweave/internal/affect"
	"time"
)

// A durable fact claim makes a lost Host response non-replayable. It is not an
// adopted input and never advances the saved relationship or affect state.
func (s *Store) ClaimWake(ctx context.Context, scope affect.Scope, id, run string, at time.Time) (bool, error) {
	if !scope.Valid() || scope.Core != s.CoreID || id == "" || len(id) > 128 || run == "" || at.IsZero() {
		return false, affect.ErrInvalid
	}
	r, err := s.DB.ExecContext(ctx, `INSERT INTO aw.facts(scope,id,kind,body,occurred_at) VALUES($1,$2,'proactive',$3,$4) ON CONFLICT DO NOTHING`, scope.Key(), "wake-started:"+id, string(encode(map[string]any{"run_id": run, "state": "started", "retry": false})), at)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}
