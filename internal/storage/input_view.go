package storage

import (
	"context"
	"github.com/endview/affectweave/internal/affect"
	"time"
)

type InputReference struct {
	ID    string    `json:"id"`
	RunID string    `json:"run_id"`
	At    time.Time `json:"occurred_at"`
}

func (s *Store) InputReferences(ctx context.Context, scope affect.Scope) ([]InputReference, error) {
	if _, err := s.Read(ctx, scope); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT input_id,run_id,occurred_at FROM aw.inputs WHERE scope=$1 ORDER BY occurred_at DESC,input_id DESC LIMIT 32`, scope.Key())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []InputReference{}
	for rows.Next() {
		var i InputReference
		if err = rows.Scan(&i.ID, &i.RunID, &i.At); err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, rows.Err()
}
