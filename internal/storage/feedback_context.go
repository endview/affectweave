package storage

import (
	"context"
	"encoding/json"
	"github.com/endview/affectweave/internal/affect"
)

func (s *Store) FeedbackContext(ctx context.Context, scope affect.Scope, in Input) (json.RawMessage, error) {
	if in.FeedbackSource == "" {
		return nil, nil
	}
	var raw []byte
	if err := s.DB.QueryRowContext(ctx, `SELECT body FROM aw.facts WHERE scope=$1 AND id=$2 AND kind='delivery' AND body->>'state'='sent'`, scope.Key(), in.FeedbackSource).Scan(&raw); err != nil {
		return nil, err
	}
	var receipt struct {
		Run  string `json:"run_id"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &receipt) != nil || receipt.Run == in.RunID || receipt.Run == "" || receipt.Text == "" || len(receipt.Text) > 8192 {
		return nil, affect.ErrInvalid
	}
	return encode(map[string]any{"source_ref": in.FeedbackSource, "delivered_text": receipt.Text, "feedback_input_id": in.ID, "maximum_evidence_level": "L2", "meaning": "linked user reaction; not independent task success proof"}), nil
}
