package affect

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// ReplyParts is an optional expression policy. FreeAgent receives the exact
// resulting parts, never the policy name or a paragraph-splitting algorithm.
func ReplyParts(text, mode string, limit int) ([]string, error) {
	if mode == "" {
		mode = "single"
	}
	if limit == 0 {
		limit = 3
	}
	if mode != "single" && mode != "paragraphs" || limit < 1 || limit > 3 || !utf8.ValidString(text) {
		return nil, ErrInvalid
	}
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	if mode == "single" || limit == 1 {
		return []string{text}, nil
	}
	// Reference links can couple distant paragraphs. Conservatively keep the
	// whole answer when splitting could remove their definition or formatting.
	if replyReference.MatchString(text) || replyHTML.MatchString(text) {
		return []string{text}, nil
	}
	var boundaries []int
	start, offset, fenceLength := 0, 0, 0
	var fence byte
	math := ""
	lines := strings.SplitAfter(text, "\n")
	previous := ""
	listBlock := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		marker, length := fenceMarker(trimmed)
		if fence != 0 {
			if marker == fence && length >= fenceLength && strings.TrimSpace(trimmed[length:]) == "" {
				fence, fenceLength = 0, 0
			}
		} else if math != "" {
			if trimmed == math {
				math = ""
			}
		} else if marker != 0 {
			fence, fenceLength = marker, length
		} else if trimmed == "$$" {
			math = "$$"
		} else if trimmed == `\[` {
			math = `\]`
		}
		offset += len(line)
		if trimmed == "" && fence == 0 && math == "" &&
			strings.TrimSpace(text[start:offset]) != "" && strings.TrimSpace(text[offset:]) != "" {
			next := ""
			for _, l := range lines[index+1:] {
				if strings.TrimSpace(l) != "" {
					next = l
					break
				}
			}
			// Attach all separator blank lines to the preceding source slice.
			if index+1 < len(lines) && strings.TrimSpace(lines[index+1]) == "" {
				continue
			}
			if replyInlineOpen(text[start:offset]) {
				continue
			}
			if replyHeading.MatchString(previous) || listBlock && (replyList.MatchString(next) || strings.HasPrefix(next, "  ") || strings.HasPrefix(next, "\t")) || strings.HasPrefix(strings.TrimSpace(previous), ">") && strings.HasPrefix(strings.TrimSpace(next), ">") || replyIndented(previous) && replyIndented(next) {
				continue
			}
			boundaries = append(boundaries, offset)
			start = offset
			listBlock = false
		}
		if trimmed != "" {
			previous = strings.TrimRight(line, "\r\n")
			if replyList.MatchString(line) {
				listBlock = true
			}
		}
	}
	boundaries = append(boundaries, len(text))
	parts := []string{}
	start = 0
	for len(boundaries) > 0 {
		slots := limit - len(parts)
		chosen := 0
		if len(boundaries) > slots && slots > 1 {
			target := utf8.RuneCountInString(text[start:]) / slots
			distance := len(text) + 1
			for i := 0; i <= len(boundaries)-slots; i++ {
				d := absReply(utf8.RuneCountInString(text[start:boundaries[i]]) - target)
				if d < distance {
					chosen, distance = i, d
				}
			}
		} else if slots == 1 {
			chosen = len(boundaries) - 1
		}
		end := boundaries[chosen]
		parts = append(parts, text[start:end])
		start = end
		boundaries = boundaries[chosen+1:]
	}
	return parts, nil
}

var replyReference = regexp.MustCompile(`(?m)^\s{0,3}\[[^\]\n]+\]:|\]\s*\[`)
var replyHeading = regexp.MustCompile(`^\s{0,3}(#{1,6}\s|={3,}\s*$|-{3,}\s*$)`)
var replyList = regexp.MustCompile(`^\s*(?:[-+*]\s|[0-9]{1,9}[.)]\s)`)
var replyHTML = regexp.MustCompile(`(?m)^\s{0,3}<[A-Za-z!/]`)

// Cross-paragraph inline formatting is uncommon but legal. Keep its source
// together instead of asking the channel renderer to repair separate slices.
func replyInlineOpen(text string) bool {
	var plain strings.Builder
	var fence byte
	size := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimSpace(line)
		marker, length := fenceMarker(trimmed)
		if fence != 0 {
			if marker == fence && length >= size && strings.TrimSpace(trimmed[length:]) == "" {
				fence = 0
			}
			continue
		}
		if marker != 0 {
			fence, size = marker, length
			continue
		}
		plain.WriteString(line)
	}
	text = plain.String()
	open := map[string]bool{}
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		c := text[i]
		if c != '*' && c != '_' && c != '`' && c != '~' {
			continue
		}
		j := i + 1
		for j < len(text) && text[j] == c {
			j++
		}
		key := text[i:j]
		open[key] = !open[key]
		i = j - 1
	}
	for _, active := range open {
		if active {
			return true
		}
	}
	return false
}

func replyIndented(s string) bool { return strings.HasPrefix(s, "    ") || strings.HasPrefix(s, "\t") }
func absReply(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// EffectiveReplyMode is the one rule used for both prompting and delivery.
func EffectiveReplyMode(mode string, state *State) string {
	if mode == "" {
		return "single"
	}
	if mode != "character" {
		return mode
	}
	if state != nil {
		h := ExpressionFor(*state)
		if h.Directness < .7 && state.Persona.EffectiveParameters().Expression >= .4 {
			return "paragraphs"
		}
	}
	return "single"
}

// Expression preference is data under Core/user instructions, not authority.
func ReplyGuidance(mode string, limit int) string {
	if mode != "paragraphs" || limit <= 1 {
		return ""
	}
	if limit > 3 {
		limit = 3
	}
	return "Optional reply expression preference (subordinate to Core rules and the current user's format): when separate chat messages are appropriate, separate them with a blank line, at most " + string(rune('0'+limit)) + " parts, usually one to three sentences each. Single newlines stay within a message. Do not pad the answer to reach the limit. Keep code, lists, tables, quotations and math intact. Generated text is not proof of delivery."
}

func fenceMarker(line string) (byte, int) {
	if len(line) < 3 || line[0] != '`' && line[0] != '~' {
		return 0, 0
	}
	length := 1
	for length < len(line) && line[length] == line[0] {
		length++
	}
	if length < 3 {
		return 0, 0
	}
	return line[0], length
}
