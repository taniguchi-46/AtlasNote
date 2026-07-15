package note

import (
	"fmt"
	"strings"
	"testing"
)

func TestExtractNoteLinkTargets(t *testing.T) {
	t.Parallel()

	first := "0123456789abcdef0123456789abcdef"
	second := "fedcba9876543210fedcba9876543210"
	content := "" +
		"[First](atlasnote://note/" + first + ")\n" +
		"[Duplicate](atlasnote://note/" + first + ")\n" +
		"![Image](atlasnote://note/" + second + ")\n" +
		"`[Inline](atlasnote://note/" + second + ")`\n" +
		"\\[Escaped](atlasnote://note/" + second + ")\n" +
		"[External](https://example.com)\n" +
		"```\n[Code](atlasnote://note/" + second + ")\n```\n" +
		"~~~markdown\n[Code](atlasnote://note/" + second + ")\n~~~\n" +
		"[Invalid](atlasnote://note/ABC)\n"

	targets := ExtractNoteLinkTargets(content)
	if len(targets) != 1 || targets[0] != first {
		t.Fatalf("targets = %#v, want [%q]", targets, first)
	}
}

func TestParseNoteLinkHref(t *testing.T) {
	t.Parallel()

	valid := "0123456789abcdef0123456789abcdef"
	for _, test := range []struct {
		href  string
		want  string
		valid bool
	}{
		{href: "atlasnote://note/" + valid, want: valid, valid: true},
		{href: "atlasnote://note/ABC", valid: false},
		{href: "atlasnote://note/" + valid + "x", valid: false},
		{href: "https://example.com", valid: false},
	} {
		got, ok := ParseNoteLinkHref(test.href)
		if got != test.want || ok != test.valid {
			t.Fatalf("ParseNoteLinkHref(%q) = (%q, %t), want (%q, %t)", test.href, got, ok, test.want, test.valid)
		}
	}
}

func TestExtractNoteLinkTargetsHandlesManyLinks(t *testing.T) {
	t.Parallel()

	const linkCount = 128
	var content strings.Builder
	for index := 0; index < linkCount; index++ {
		id := fmt.Sprintf("%032x", index)
		fmt.Fprintf(&content, "[%d](atlasnote://note/%s)\n", index, id)
	}
	content.WriteString("[duplicate](atlasnote://note/00000000000000000000000000000000)\n")

	targets := ExtractNoteLinkTargets(content.String())
	if len(targets) != linkCount {
		t.Fatalf("target count = %d, want %d", len(targets), linkCount)
	}
	if targets[0] != "00000000000000000000000000000000" || targets[linkCount-1] != "0000000000000000000000000000007f" {
		t.Fatalf("target order = (%q, %q), want first/last IDs", targets[0], targets[linkCount-1])
	}
}
