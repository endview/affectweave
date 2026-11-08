package affect

import "unicode/utf8"

type ReplyResource struct {
	ID    string `json:"id"`
	Emoji string `json:"emoji"`
}
type FollowupExpression struct {
	Text         string `json:"text"`
	Source       string `json:"source_ref"`
	DelaySeconds int    `json:"after_seconds"`
}
type Expression struct {
	Sticker  string              `json:"sticker_ref,omitempty"`
	Reaction string              `json:"reaction,omitempty"`
	Followup *FollowupExpression `json:"followup,omitempty"`
}

func (e Expression) Validate(resources []ReplyResource, reactions []string) error {
	if e.Sticker != "" {
		found := false
		for _, r := range resources {
			found = found || r.ID == e.Sticker
		}
		if !found {
			return ErrInvalid
		}
	}
	if e.Reaction != "" {
		found := false
		for _, r := range reactions {
			found = found || r == e.Reaction
		}
		if !found {
			return ErrInvalid
		}
	}
	if f := e.Followup; f != nil {
		if f.Source == "" || len(f.Source) > 256 || f.DelaySeconds < 60 || f.DelaySeconds > 7*86400 || f.Text == "" || utf8.RuneCountInString(f.Text) > 600 {
			return ErrInvalid
		}
	}
	return nil
}
