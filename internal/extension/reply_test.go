package extension

import (
	"encoding/json"
	"testing"
)

func TestFinishedResponseOnlyOffersPartsForCompletedAuthorizedReply(t *testing.T) {
	var p Params
	if err := json.Unmarshal([]byte(`{"output":"第一段\n\n第二段","status":"completed","settings":{"reply_mode":"paragraphs"},"delivery":{"reply":true,"max_text_parts":3}}`), &p); err != nil {
		t.Fatal(err)
	}
	result, err := finishedResponse(p, "done")
	if err != nil || len(result["reply_parts"].([]string)) != 2 {
		t.Fatal(result, err)
	}
	for _, status := range []string{"unknown", "stopped", "failed", "waiting_confirmation"} {
		p.Status = status
		result, err = finishedResponse(p, "done")
		if err != nil || result["reply_parts"] != nil {
			t.Fatal(status, result, err)
		}
	}
	p.Status = "completed"
	p.Delivery = nil
	if result, err = finishedResponse(p, "done"); err != nil || result["reply_parts"] != nil {
		t.Fatal("local-only turn emitted channel parts", result, err)
	}
}
