package affect

import (
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
	var parts []string
	start, offset, fenceLength := 0, 0, 0
	var fence byte
	math := ""
	for _, line := range strings.SplitAfter(text, "\n") {
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
		if trimmed == "" && fence == 0 && math == "" && len(parts) < limit-1 &&
			strings.TrimSpace(text[start:offset]) != "" && strings.TrimSpace(text[offset:]) != "" {
			parts = append(parts, text[start:offset])
			start = offset
		}
	}
	if start < len(text) {
		parts = append(parts, text[start:])
	}
	return parts, nil
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
