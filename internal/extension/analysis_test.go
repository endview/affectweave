package extension

import "testing"

func TestAnalysisRequiresCompleteTentativeContract(t *testing.T) {
	valid := `{"appraisal":{"kind":"interest","confidence":0.7,"reason":"A concrete question"},"emotion_candidates":[],"behavior_hint":"","persistent_candidates":{}}`
	var a Analysis
	if err := decodeAnalysis([]byte(valid), &a); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{}`, `null`, `{"appraisal":{"kind":"interest","reason":"No confidence"},"emotion_candidates":[],"behavior_hint":"","persistent_candidates":{}}`, `{"appraisal":{"kind":"interest","confidence":0.7,"reason":"A question"},"emotion_candidates":null,"behavior_hint":"","persistent_candidates":{}}`, valid + ` {}`} {
		if decodeAnalysis([]byte(raw), &a) == nil {
			t.Fatal("incomplete/ambiguous analysis accepted", raw)
		}
	}
}
