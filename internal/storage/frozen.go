package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/endview/affectweave/internal/affect"
)

type InputOptions struct {
	MaxBytes   int  `json:"max_bytes"`
	Analysis   bool `json:"analysis"`
	DeadlineMS int  `json:"deadline_ms"`
}

func (o InputOptions) defaults() InputOptions {
	if o.MaxBytes == 0 {
		o.MaxBytes = 6000
	}
	if o.DeadlineMS == 0 {
		o.DeadlineMS = 8000
	}
	return o
}

// FrozenInput owns the complete adopted fragment, including selected memory
// contents. Repeated hooks never consult newer memories or settings.
type FrozenInput struct {
	Expression *affect.Expression `json:"expression,omitempty"`
	Feedback   *Outcome           `json:"feedback_draft,omitempty"`
	Input      Input              `json:"input"`
	State      affect.State       `json:"state"`
	Memories   []affect.Memory    `json:"memories"`
	Context    affect.Context     `json:"context"`
	Options    InputOptions       `json:"options"`
	Analysis   string             `json:"analysis"`
	Detail     string             `json:"detail,omitempty"`
	Draft      *affect.Proposal   `json:"draft,omitempty"`
}
type queryRows interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readMemories(ctx context.Context, db queryRows, scope affect.Scope) ([]affect.Memory, error) {
	rows, err := db.QueryContext(ctx, `SELECT body FROM aw.memories WHERE scope=$1 ORDER BY created_at DESC,id LIMIT 256`, scope.Key())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []affect.Memory{}
	for rows.Next() {
		var raw []byte
		var m affect.Memory
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
