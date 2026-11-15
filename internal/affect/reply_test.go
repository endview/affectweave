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
		{"single-newlines-stay", "第一句。\n第二句。", 1},
		{"loose-list", "- 第一项\n\n- 第二项\n\n  继续解释\n\n结束。", 2},
		{"quote", "> 第一行\n\n> 第二行\n\n结束。", 2},
		{"heading", "## 标题\n\n正文。\n\n结束。", 2},
		{"reference-link", "[资料][a]\n\n结束\n\n[a]: https://example.com", 1},
		{"indented-code", "    code\n\n    more\n\n结束。", 2},
		{"table", "| 项目 | 值 |\n| --- | --- |\n| 一 | 二 |\n\n结束。", 2},
		{"cross-paragraph-emphasis", "**第一段\n\n第二段**\n\n结束。", 2},
		{"html-block", "<details>\n\n正文\n\n</details>", 1},
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

func TestCharacterReplyModeUsesFrozenParameters(t *testing.T) {
	p := NeutralParameters()
	state := State{Persona: Persona{Parameters: &p}}
	if got := EffectiveReplyMode("character", &state); got != "paragraphs" {
		t.Fatal(got)
	}
	p.Expression = .39
	if got := EffectiveReplyMode("character", &state); got != "single" {
		t.Fatal(got)
	}
	p.Expression = .5
	p.Directness = 1
	if got := EffectiveReplyMode("character", &state); got != "single" {
		t.Fatal(got)
	}
	if EffectiveReplyMode("character", nil) != "single" || ReplyGuidance("single", 3) != "" || ReplyGuidance("paragraphs", 1) != "" {
		t.Fatal("mode/host bounds ignored")
	}
}

func TestReplyPartsBalanceWholeParagraphs(t *testing.T) {
	text := "一。\n\n二。\n\n三。\n\n四。\n\n五。\n\n六。"
	parts, err := ReplyParts(text, "paragraphs", 3)
	if err != nil || len(parts) != 3 || strings.Join(parts, "") != text {
		t.Fatal(parts, err)
	}
	if parts[0] != "一。\n\n二。\n\n" || parts[1] != "三。\n\n四。\n\n" {
		t.Fatalf("unbalanced: %q", parts)
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
