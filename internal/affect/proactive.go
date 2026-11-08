package affect

import "strings"

type Topic struct {
	Ref     string `json:"ref"`
	Summary string `json:"summary"`
}

// Only explicitly granted, confirmed material is eligible. No retrieval,
// hidden memory scan, news search or interpretation of silence happens here.
func Topics(s State, memories []Memory, refs []string, free bool) []Topic {
	result := []Topic{}
	for _, ref := range refs {
		text := ""
		switch {
		case ref == "persona:interests" && free:
			text = strings.Join(s.Persona.Preferences, "; ")
		case strings.HasPrefix(ref, "thread:"):
			if t, ok := s.Threads[strings.TrimPrefix(ref, "thread:")]; ok && t.Status == "active" {
				text = t.Summary
			}
		case strings.HasPrefix(ref, "memory:"):
			for _, m := range memories {
				if "memory:"+m.ID == ref {
					text = m.Summary
				}
			}
		}
		if text != "" && len(text) <= 4096 {
			result = append(result, Topic{ref, text})
		}
	}
	return result[:min(3, len(result))]
}
