package extension

import (
	"context"
	"encoding/json"
	"github.com/endview/affectweave/internal/affect"
	"github.com/endview/affectweave/internal/storage"
	"strings"
	"time"
)

func (s *Server) finishExpressions(ctx context.Context, p Params, f storage.FrozenInput, scope affect.Scope) (map[string]any, error) {
	if p.Settings.ReplyMode == "character" {
		p.Settings.ReplyMode = "single"
		hints := affect.ExpressionFor(f.State)
		if hints.Directness < .7 && f.State.Persona.EffectiveParameters().Expression >= .4 {
			p.Settings.ReplyMode = "paragraphs"
		}
	}
	result, err := finishedResponse(p, "Character facts recorded; persistent proposals require confirmation")
	if err != nil || p.Status != "completed" || p.Delivery == nil || !p.Delivery.Reply || !p.Settings.Expression || f.Expression == nil {
		return result, err
	}
	e := f.Expression
	for _, action := range []map[string]string{{"action": "sticker", "resource": e.Sticker}, {"action": "reaction", "emoji": e.Reaction}} {
		if action["resource"] == "" && action["emoji"] == "" {
			continue
		}
		var receipt json.RawMessage
		// Unavailable/revoked resources and policy requiring a separate approval
		// simply retain the ordinary reply. Never claim the expression was sent.
		_ = s.hostCall(ctx, "delivery.reply", map[string]any{"invocation": p.Invocation, "action": action}, &receipt)
	}
	if e.Followup == nil || !p.Settings.Followup {
		return result, nil
	}
	t, ok := f.State.Threads[strings.TrimPrefix(e.Followup.Source, "thread:")]
	if !strings.HasPrefix(e.Followup.Source, "thread:") || !ok || t.Status != "active" {
		return result, nil
	}
	after := f.Input.At.Add(time.Duration(e.Followup.DelaySeconds) * time.Second)
	expires := after.Add(time.Hour)
	if !expires.After(time.Now()) {
		return result, nil
	}
	if len(p.Delivery.RuleIDs) > 0 {
		key := "followup:" + affect.Hash([]byte(f.Input.ID+"/"+e.Followup.Source))
		candidate := map[string]any{"key": key, "rule_id": p.Delivery.RuleIDs[0], "text": e.Followup.Text, "after": after.UTC().Format(time.RFC3339Nano), "expires": expires.UTC().Format(time.RFC3339Nano), "reason": e.Followup.Source}
		body, _ := json.Marshal(map[string]string{"source_ref": e.Followup.Source})
		if err := s.Store.RecordFact(ctx, scope, storage.Fact{ID: key, Kind: "expression", Body: body, At: f.Input.At}); err != nil {
			return result, err
		}
		var receipt json.RawMessage
		_ = s.hostCall(ctx, "delivery.propose", map[string]any{"invocation": p.Invocation, "candidate": candidate}, &receipt)
	} else if len(p.Delivery.WakeRuleIDs) > 0 {
		result["wake_hint"] = map[string]any{"rule_id": p.Delivery.WakeRuleIDs[0], "source_refs": []string{e.Followup.Source}, "not_before": after.UTC().Format(time.RFC3339Nano), "not_after": expires.UTC().Format(time.RFC3339Nano)}
	}
	return result, nil
}
func (s *Server) checkDelivery(ctx context.Context, raw []byte) (any, error) {
	var p struct {
		Identity *Identity `json:"identity"`
		Variant  string    `json:"variant"`
		Settings struct {
			Role string `json:"role_id"`
		} `json:"settings"`
		Candidate struct {
			Key string `json:"key"`
		} `json:"candidate"`
	}
	if json.Unmarshal(raw, &p) != nil || p.Identity == nil || len(p.Identity.Inputs) == 0 {
		return nil, affect.ErrInvalid
	}
	role := p.Variant
	if role == "" {
		role = p.Settings.Role
	}
	scope := affect.Scope{Core: p.Identity.Core, Space: p.Identity.Space, Role: role, Actor: p.Identity.Inputs[len(p.Identity.Inputs)-1].Actor}
	state, err := s.Store.Read(ctx, scope)
	if err != nil {
		return map[string]bool{"allow": false}, nil
	}
	var rawFact []byte
	if err = s.Store.DB.QueryRowContext(ctx, `SELECT body FROM aw.facts WHERE scope=$1 AND id=$2 AND kind='expression'`, scope.Key(), p.Candidate.Key).Scan(&rawFact); err != nil {
		return map[string]bool{"allow": false}, nil
	}
	var fact struct {
		Source string `json:"source_ref"`
	}
	_ = json.Unmarshal(rawFact, &fact)
	thread, ok := state.Threads[strings.TrimPrefix(fact.Source, "thread:")]
	return map[string]bool{"allow": ok && thread.Status == "active"}, nil
}
