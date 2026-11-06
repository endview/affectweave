package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/endview/affectweave/internal/affect"
)

type Source struct {
	ID      string `json:"id"`
	RunID   string `json:"run_id"`
	SpaceID string `json:"space_id"`
	ActorID string `json:"actor_id"`
	Hash    string `json:"source_hash"`
}
type SourcePage struct {
	CoreID  string   `json:"core_id"`
	Sources []Source `json:"sources"`
	Next    string   `json:"next,omitempty"`
}

func (s *Store) Sources(ctx context.Context, cursor string) (SourcePage, error) {
	var key [2]string
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || len(raw) > 1024 || json.Unmarshal(raw, &key) != nil {
			return SourcePage{}, affect.ErrInvalid
		}
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT i.scope,i.input_id,i.run_id,s.space_id,s.actor_id,i.source_hash FROM aw.inputs i JOIN aw.states s ON s.scope=i.scope WHERE (i.scope,i.input_id)>($1,$2) ORDER BY i.scope,i.input_id LIMIT 129`, key[0], key[1])
	if err != nil {
		return SourcePage{}, err
	}
	defer rows.Close()
	result := SourcePage{CoreID: s.CoreID, Sources: []Source{}}
	last := key
	for rows.Next() {
		var scope string
		var source Source
		if err = rows.Scan(&scope, &source.ID, &source.RunID, &source.SpaceID, &source.ActorID, &source.Hash); err != nil {
			return result, err
		}
		if len(result.Sources) == 128 {
			b, _ := json.Marshal(last)
			result.Next = base64.RawURLEncoding.EncodeToString(b)
			break
		}
		result.Sources = append(result.Sources, source)
		last = [2]string{scope, source.ID}
	}
	return result, rows.Err()
}
