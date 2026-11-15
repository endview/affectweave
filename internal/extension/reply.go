package extension

import "github.com/endview/affectweave/internal/affect"

func replyLimit(p Params) int {
	limit := p.Settings.ReplyParts
	if limit == 0 {
		limit = 3
	}
	if p.Delivery != nil {
		limit = min(limit, p.Delivery.MaxTextParts)
	}
	return limit
}

func replyContext(p Params, text string, state *affect.State) string {
	if p.Delivery == nil || !p.Delivery.Reply {
		return text
	}
	hint := affect.ReplyGuidance(affect.EffectiveReplyMode(p.Settings.ReplyMode, state), replyLimit(p))
	maxBytes := p.Settings.MaxBytes
	if maxBytes == 0 {
		maxBytes = 6000
	}
	if hint != "" && len(hint) <= 512 && len(text)+len(hint)+1 <= maxBytes {
		if text != "" {
			text += "\n"
		}
		text += hint
	}
	return text
}

// No scheduling, target selection or side effects take place here. The Host
// still validates output identity, authorization and every delivery receipt.
func finishedResponse(p Params, summary string) (map[string]any, error) {
	p.Settings.ReplyMode = affect.EffectiveReplyMode(p.Settings.ReplyMode, nil)
	result := map[string]any{"activity": map[string]any{"summary": summary}}
	if p.Status != "completed" || p.Delivery == nil || !p.Delivery.Reply || p.Delivery.MaxTextParts < 1 {
		return result, nil
	}
	limit := p.Settings.ReplyParts
	if limit == 0 {
		limit = 3
	}
	if limit < 1 || limit > 3 {
		return nil, affect.ErrInvalid
	}
	limit = min(limit, p.Delivery.MaxTextParts)
	parts, err := affect.ReplyParts(p.Output, p.Settings.ReplyMode, limit)
	if err != nil {
		return nil, err
	}
	if len(parts) != 0 {
		result["reply_parts"] = parts
	}
	return result, nil
}
