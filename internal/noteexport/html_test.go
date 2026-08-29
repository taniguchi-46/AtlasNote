package noteexport

import (
	"strings"
	"testing"
)

func TestRenderHTMLDocumentSanitizesActiveContentAndURLs(t *testing.T) {
	validID := "0123456789abcdef0123456789abcdef"
	fragment := `<div class="wrapper">
<script>alert('script')</script><style>.evil{display:block}</style>
<noscript>noscript body</noscript><iframe>frame body</iframe>
<form><label>secret<input value="value"></label></form>
<h1 onclick="run()" style="color:red">見出し</h1>
<p class="ignored">本文 <strong data-x="1">太字</strong> <em>斜体</em> <del>削除</del> <s>取消</s> <strike>旧式取消</strike></p>
<p><a href="https://example.test/path">https</a> <a href="http://example.test">http</a>
<a href="mailto:person@example.test">mail</a> <a href="tel:+819012345678">tel</a>
<a href="atlasnote://note/` + validID + `">note</a> <a href="#part">anchor</a>
<a href="javascript:alert(1)">javascript</a> <a href="data:text/plain,no">data</a>
<a href="file:///secret">file</a> <a href="https://user:pass@example.test">credentials</a></p>
<img src="https://tracker.test/pixel" alt="画像の説明" onerror="run()">
<ul data-type="taskList"><li data-checked="true"><input type="checkbox" checked>完了</li><li data-checked="false"><input type="checkbox">未完了</li></ul>
<p hidden>hidden body</p>
<svg><text>svg body</text></svg><object>object body</object>
</div>`

	document, err := renderHTMLDocument(`正本 <タイトル>`, fragment)
	if err != nil {
		t.Fatalf("renderHTMLDocument() error = %v", err)
	}
	output := string(document)

	wants := []string{
		`<!doctype html>`, `<html lang="ja">`, `<meta charset="utf-8">`,
		`default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'`,
		`<title>正本 &lt;タイトル&gt;</title>`, `<h1>見出し</h1>`,
		`<strong>太字</strong>`, `<em>斜体</em>`, `<del>削除</del>`,
		`<del>取消</del>`, `<del>旧式取消</del>`, `画像の説明`,
		`href="https://example.test/path" rel="noopener noreferrer"`,
		`href="http://example.test" rel="noopener noreferrer"`,
		`href="mailto:person@example.test" rel="noopener noreferrer"`,
		`href="tel:+819012345678" rel="noopener noreferrer"`,
		`href="atlasnote://note/` + validID + `" rel="noopener noreferrer"`,
		`href="#part" rel="noopener noreferrer"`,
		`<a>javascript</a>`, `<a>data</a>`, `<a>file</a>`, `<a>credentials</a>`,
		`<li>[x] 完了</li>`, `<li>[ ] 未完了</li>`,
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Errorf("rendered HTML missing %q\n%s", want, output)
		}
	}
	for _, forbidden := range []string{
		"alert('script')", ".evil", "noscript body", "frame body", "secret", "value",
		"svg body", "object body", "hidden body", "onclick", "onerror", "class=", "src=", "data-x", "javascript:", "file:///", "user:pass",
	} {
		if strings.Contains(output, forbidden) {
			t.Errorf("rendered HTML contains forbidden value %q\n%s", forbidden, output)
		}
	}
}

func TestRenderHTMLDocumentPreservesSupportedStructureAndEmptyBody(t *testing.T) {
	fragment := `<h2>章</h2><blockquote><p>引用</p></blockquote>
<ul><li>一</li><li><ol start="3"><li>二</li></ol></li></ul>
<pre><code>if a &lt; b {\n  ok()\n}</code></pre><hr>
<table><thead><tr><th>列</th></tr></thead><tbody><tr><td>値</td></tr></tbody></table>`
	document, err := renderHTMLDocument("構造", fragment)
	if err != nil {
		t.Fatalf("renderHTMLDocument() error = %v", err)
	}
	output := string(document)
	for _, want := range []string{
		"<h2>章</h2>", "<blockquote><p>引用</p></blockquote>",
		"<ul><li>一</li><li><ol start=\"3\"><li>二</li></ol></li></ul>",
		"<pre><code>if a &lt; b {\\n  ok()\\n}</code></pre>", "<hr/>",
		"<table><thead><tr><th>列</th></tr></thead><tbody><tr><td>値</td></tr></tbody></table>",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered structure missing %q\n%s", want, output)
		}
	}

	empty, err := renderHTMLDocument("空", "")
	if err != nil {
		t.Fatalf("empty render error = %v", err)
	}
	if !strings.Contains(string(empty), "<body></body>") {
		t.Fatalf("empty document does not have an empty body:\n%s", empty)
	}
}

func TestSafeHTMLHref(t *testing.T) {
	validID := "abcdefabcdefabcdefabcdefabcdefab"
	tests := []struct {
		href string
		want bool
	}{
		{href: "https://example.test", want: true},
		{href: "http://example.test/path?q=1#part", want: true},
		{href: "mailto:user@example.test", want: true},
		{href: "tel:+819012345678", want: true},
		{href: "atlasnote://note/" + validID, want: true},
		{href: "atlasnote://note/" + validID + "#part", want: true},
		{href: "#part", want: true},
		{href: "#", want: false},
		{href: "javascript:alert(1)", want: false},
		{href: "data:text/html,unsafe", want: false},
		{href: "file:///tmp/a", want: false},
		{href: "https://user:password@example.test", want: false},
		{href: "https://", want: false},
		{href: " atlasnote://note/" + validID, want: false},
		{href: "atlasnote://note/ABC", want: false},
	}
	for _, test := range tests {
		if got := safeHTMLHref(test.href); got != test.want {
			t.Errorf("safeHTMLHref(%q) = %v, want %v", test.href, got, test.want)
		}
	}
}

func TestSanitizeHTMLFragmentRejectsExcessiveNesting(t *testing.T) {
	fragment := strings.Repeat("<div>", maxHTMLDepth+1) + "body" + strings.Repeat("</div>", maxHTMLDepth+1)
	if _, err := sanitizeHTMLFragment(fragment); err == nil {
		t.Fatal("sanitizeHTMLFragment() accepted excessively nested HTML")
	}
}
