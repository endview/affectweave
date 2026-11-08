package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type queryRow interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// A human approval authorizes an interpretation; it is not independent proof
// that a task succeeded. Feedback must be a later, explicitly linked input.
func validateFeedback(ctx context.Context, db queryRow, c Change) error {
	var kind string
	var raw []byte
	var delivered time.Time
	if err := db.QueryRowContext(ctx, `SELECT kind,body,occurred_at FROM aw.facts WHERE scope=$1 AND id=$2`, c.Scope.Key(), c.Source).Scan(&kind, &raw, &delivered); err != nil {
		return errors.New("affectweave: delivered source required")
	}
	var receipt struct {
		State string `json:"state"`
		Run   string `json:"run_id"`
	}
	if json.Unmarshal(raw, &receipt) != nil || kind != "delivery" || receipt.State != "sent" || receipt.Run == "" {
		return errors.New("affectweave: complete verified delivery required")
	}
	var run string
	var at time.Time
	var frozenRaw []byte
	if err := db.QueryRowContext(ctx, `SELECT run_id,occurred_at,context_snapshot FROM aw.inputs WHERE scope=$1 AND input_id=$2`, c.Scope.Key(), c.Outcome.FeedbackID).Scan(&run, &at, &frozenRaw); err != nil {
		return errors.New("affectweave: adopted feedback required")
	}
	var frozen FrozenInput
	if json.Unmarshal(frozenRaw, &frozen) != nil || !at.After(delivered) || run == receipt.Run || frozen.Input.FeedbackSource != c.Source {
		return errors.New("affectweave: feedback must explicitly reference a preceding delivered result")
	}
	if c.Outcome.Evidence != "L2" {
		return errors.New("affectweave: linked user reaction supports L2, not independent L3 proof")
	}
	return nil
}
