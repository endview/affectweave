package storage

import (
	"context"
	"encoding/json"
	"github.com/endview/affectweave/internal/affect"
	"math"
	"time"
)

// Late approval applies the remaining impulse at the current state time. It
// does not replay the full old emotion as a newly occurring event.
func prepareProposal(ctx context.Context, db queryRow, state affect.State, c Change) (Change, error) {
	source := c.Source
	if c.Outcome != nil {
		source = c.Outcome.FeedbackID
	}
	if c.Action != "apply_proposal" && c.Action != "record_outcome" {
		return c, nil
	}
	var at time.Time
	if err := db.QueryRowContext(ctx, `SELECT occurred_at FROM aw.inputs WHERE scope=$1 AND input_id=$2`, c.Scope.Key(), source).Scan(&at); err != nil {
		return c, err
	}
	raw := encode(c)
	var prepared Change
	if json.Unmarshal(raw, &prepared) != nil {
		return c, affect.ErrInvalid
	}
	c = prepared
	p := c.Proposal
	if c.Outcome != nil {
		p = &c.Outcome.Proposal
	}
	if p == nil {
		return c, affect.ErrInvalid
	}
	params := state.Persona.EffectiveParameters()
	factor := math.Exp(-math.Max(0, state.Updated.Sub(at).Seconds()) / params.Tau())
	for n, d := range p.Emotions {
		gain := params.Gain(d.Type)
		p.Emotions[n].Delta = affect.Clamp(d.Delta*gain, -.25, .25) * factor / gain
	}
	return c, nil
}
