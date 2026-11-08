package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/endview/affectweave/internal/affect"
)

// EnsureFactSource verifies an existing source without preparing its context
// again. Fact synchronization is not a turn hook: it cannot replace a missing
// frozen fragment, advance state again or initiate analysis.
func (s *Store) EnsureFactSource(ctx context.Context, scope affect.Scope, in Input) error {
	if !scope.Valid() || scope.Core != s.CoreID || in.ID == "" || in.RunID == "" || in.At.IsZero() || len(in.Text) > 65536 {
		return affect.ErrInvalid
	}
	var hash string
	err := s.DB.QueryRowContext(ctx, `SELECT i.source_hash FROM aw.inputs i JOIN aw.states s ON s.scope=i.scope JOIN aw.roles r ON r.id=s.role_id WHERE i.scope=$1 AND i.input_id=$2 AND r.spaces ? $3`, scope.Key(), in.ID, scope.Space).Scan(&hash)
	if err == nil {
		if hash != affect.Hash(encode(in)) {
			return ErrOperation
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = s.Adopt(ctx, scope, in)
	return err
}
