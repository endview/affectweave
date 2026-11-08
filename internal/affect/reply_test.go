package affect

import (
	"strings"
	"testing"
)

func TestReplyPartsPreserveCodeMathAndSource(t *testing.T) {
	for _, test := range []struct {
		name, text string
		want       int
	}{
		{"paragraphs", "第一段。\n\n第二段。\n\n第三段。\n\n第四段。", 3},
		{"code", "第一段。\n\n```go\ncode\n\nstill code\n```\n\n第三段。", 3},
		{"long-fence", "a\n\n````text\n```\n\ninside\n````\n\nz", 3},
		{"unclosed-code", "a\n\n~~~\ncode\n\ninside", 2},
		{"dollar-math", "a\n\n$$\nx\n\ny\n$$\n\nz", 3},
		{"bracket-math", "a\n\n\\[\nx\n\ny\n\\]\n\nz", 3},
		{"crlf", "第一段\r\n\r\n第二段\r\n\r\n第三段", 3},
		{"unicode", "你好🙂\n\n**还好**。", 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			parts, err := ReplyParts(test.text, "paragraphs", 3)
			if err != nil || len(parts) != test.want || strings.Join(parts, "") != test.text {
				t.Fatal(parts, err)
			}
			for _, mode := range []string{"", "single"} {
				parts, err := ReplyParts(test.text, mode, 3)
				if err != nil || len(parts) != 1 || parts[0] != test.text {
					t.Fatal(parts, err)
				}
			}
		})
	}
}

func TestReplyPartsRejectInvalidPolicyAndKeepBound(t *testing.T) {
	for _, test := range []struct {
		mode  string
		limit int
	}{{"other", 3}, {"paragraphs", -1}, {"paragraphs", 4}} {
		if _, err := ReplyParts("answer", test.mode, test.limit); err == nil {
			t.Fatal(test)
		}
	}
	if _, err := ReplyParts(string([]byte{255}), "paragraphs", 3); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	for _, text := range []string{"", " \n \n"} {
		if parts, err := ReplyParts(text, "paragraphs", 3); err != nil || len(parts) != 0 {
			t.Fatal(parts, err)
		}
	}
	text := "a\n\nb\n\nc\n\nd"
	for _, limit := range []int{1, 2, 3} {
		parts, err := ReplyParts(text, "paragraphs", limit)
		if err != nil || len(parts) != limit || strings.Join(parts, "") != text {
			t.Fatal(parts, err)
		}
	}
}
