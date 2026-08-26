package noteimport

import (
	"errors"
	"strings"
	"testing"
)

func TestConvertHTMLRendersSupportedDocumentStructure(t *testing.T) {
	converted, err := convertHTML(`
<!doctype html>
<html>
  <head><title>Metadata title</title><meta name="title" content="Fallback metadata"></head>
  <body>
    <h1>Document title</h1>
    <p>First<br>second <strong>bold</strong> <em>emphasis</em> <del>deleted</del></p>
    <blockquote><p>quoted</p><p>second quote</p></blockquote>
    <ul><li>outer<ul><li>nested</li></ul></li><li><input type="checkbox" checked>done</li></ul>
    <ol start="3"><li>third</li></ol>
    <p><code>inline</code></p>
    <pre><code class="language-go">func main() {
  println("ok")
}</code></pre>
    <hr>
    <table><thead><tr><th>Head</th><th>Other</th></tr></thead><tbody><tr><td>A|B</td><td>one<br>two</td></tr></tbody></table>
    <p><a href="https://example.test/path">safe link</a> <img src="https://example.test/image.png" alt="image label"></p>
  </body>
</html>`)
	if err != nil {
		t.Fatalf("convert HTML: %v", err)
	}

	for _, expected := range []string{
		"# Document title",
		"First  \nsecond **bold** *emphasis* ~~deleted~~",
		"> quoted",
		"> second quote",
		"- outer\n  - nested",
		"- [x] done",
		"3. third",
		"`inline`",
		"```go\nfunc main() {\n  println(\"ok\")\n}\n```",
		"---",
		"| Head | Other |\n| --- | --- |\n| A\\|B | one  \\ntwo |",
		"[safe link](https://example.test/path) image label",
	} {
		if !strings.Contains(converted.Content, expected) {
			t.Fatalf("converted content does not contain %q:\n%s", expected, converted.Content)
		}
	}
	if converted.HeadingTitle != "Document title" {
		t.Fatalf("heading title = %q", converted.HeadingTitle)
	}
	if converted.MetadataTitle != "Metadata title" {
		t.Fatalf("metadata title = %q", converted.MetadataTitle)
	}
}

func TestConvertHTMLDropsUnsafeContentAndKeepsOnlySafeLinks(t *testing.T) {
	converted, err := convertHTML(`<html><head><style>hidden</style></head><body>
	<p class="ignored" onclick="run()"><a href="javascript:alert(1)">unsafe link</a> <a href="data:text/plain,unsafe">data link</a> <a href="http://example.test">http</a> <a href="mailto:person@example.test">mail</a> <a href="tel:+819012345678">phone</a> <a href="atlasnote://note/example">internal</a> <a href="#part">fragment</a></p>
	<script>steal()</script><iframe>frame text</iframe><form><input>form text</form><svg><text>vector text</text></svg>
	<p><unknown data-value="x">visible <b>child</b></unknown><img src="remote" alt="diagram"></p>
	</body></html>`)
	if err != nil {
		t.Fatalf("convert HTML: %v", err)
	}
	for _, forbidden := range []string{"javascript:", "data:text", "steal", "frame text", "form text", "vector text", "onclick", "class=", "<script", "<img"} {
		if strings.Contains(strings.ToLower(converted.Content), strings.ToLower(forbidden)) {
			t.Fatalf("unsafe value %q remained in %q", forbidden, converted.Content)
		}
	}
	for _, expected := range []string{
		"unsafe link",
		"data link",
		"[http](http://example.test)",
		"[mail](mailto:person@example.test)",
		"[phone](tel:+819012345678)",
		"[internal](atlasnote://note/example)",
		"[fragment](#part)",
		"visible **child**diagram",
	} {
		if !strings.Contains(converted.Content, expected) {
			t.Fatalf("converted content does not contain %q: %q", expected, converted.Content)
		}
	}
}

func TestConvertHTMLUsesPlainTextFallbackForComplexTables(t *testing.T) {
	converted, err := convertHTML(`<table>
<tr><th colspan="2">Header</th></tr>
<tr><td>left</td><td>right</td></tr>
</table>`)
	if err != nil {
		t.Fatalf("convert HTML: %v", err)
	}
	if strings.Contains(converted.Content, "| ---") {
		t.Fatalf("complex table unexpectedly became a GFM table: %q", converted.Content)
	}
	if !strings.Contains(converted.Content, "Header") || !strings.Contains(converted.Content, "left | right") {
		t.Fatalf("complex table fallback = %q", converted.Content)
	}
}

func TestConvertHTMLPreservesCodeWhitespaceAndDynamicInlineFence(t *testing.T) {
	converted, err := convertHTML("<p><code>a`b</code></p><pre><code class=\"language-c++\">line 1\n\n  line 3\n</code></pre>")
	if err != nil {
		t.Fatalf("convert HTML: %v", err)
	}
	if !strings.Contains(converted.Content, "``a`b``") {
		t.Fatalf("inline code fence = %q", converted.Content)
	}
	if !strings.Contains(converted.Content, "```c++\nline 1\n\n  line 3\n```") {
		t.Fatalf("pre/code conversion did not preserve whitespace: %q", converted.Content)
	}
}

func TestConvertHTMLTitleFallbacksAndEmptyDocument(t *testing.T) {
	metadataOnly, err := convertHTML("<head><title>Page title</title><meta name=title content=Meta></head><p>body</p>")
	if err != nil {
		t.Fatalf("convert metadata HTML: %v", err)
	}
	if metadataOnly.HeadingTitle != "" || metadataOnly.MetadataTitle != "Page title" {
		t.Fatalf("metadata title = %#v", metadataOnly)
	}

	metaOnly, err := convertHTML("<head><meta name=title content=\"Meta title\"></head><p>body</p>")
	if err != nil {
		t.Fatalf("convert meta HTML: %v", err)
	}
	if metaOnly.MetadataTitle != "Meta title" {
		t.Fatalf("meta fallback = %#v", metaOnly)
	}

	if _, err := convertHTML("<html><head><title>Only metadata</title></head><body><script>hidden</script></body></html>"); !errors.Is(err, errHTMLWithoutVisibleContent) {
		t.Fatalf("empty converted HTML error = %v", err)
	}

	if _, err := convertHTML("<p>unclosed <strong>markup"); err != nil {
		t.Fatalf("malformed HTML must be tolerated: %v", err)
	}
}

func TestConvertHTMLDropsHiddenSubtreesAndIgnoresHiddenHeading(t *testing.T) {
	converted, err := convertHTML(`<html>
<head><title>Metadata title</title></head>
<body>
  <h1 hidden>Hidden heading</h1>
  <h1>Visible heading</h1>
  <p>Before <span hidden>hidden paragraph text</span> after</p>
  <p hidden="false">hidden boolean attribute text</p>
  <ul><li hidden>hidden list item</li><li>visible list item <span hidden>hidden list text</span></li></ul>
  <pre><code>visible code <span hidden>hidden code text</span></code></pre>
  <table><tr hidden><td>hidden table row</td></tr><tr><td>visible cell <span hidden>hidden cell text</span></td></tr></table>
</body>
</html>`)
	if err != nil {
		t.Fatalf("convert HTML: %v", err)
	}
	if converted.HeadingTitle != "Visible heading" {
		t.Fatalf("heading title = %q", converted.HeadingTitle)
	}
	for _, forbidden := range []string{
		"Hidden heading",
		"hidden paragraph text",
		"hidden boolean attribute text",
		"hidden list item",
		"hidden list text",
		"hidden code text",
		"hidden table row",
		"hidden cell text",
	} {
		if strings.Contains(converted.Content, forbidden) {
			t.Fatalf("hidden content %q remained in %q", forbidden, converted.Content)
		}
	}
	for _, expected := range []string{"# Visible heading", "- visible list item", "visible code", "visible cell"} {
		if !strings.Contains(converted.Content, expected) {
			t.Fatalf("visible content %q is missing from %q", expected, converted.Content)
		}
	}
	if !strings.Contains(strings.Join(strings.Fields(converted.Content), " "), "Before after") {
		t.Fatalf("visible paragraph text is missing from %q", converted.Content)
	}
}

func TestConvertHTMLRejectsDocumentWithOnlyHiddenBodyContent(t *testing.T) {
	for _, source := range []string{
		`<html><body><p hidden>hidden</p><p hidden="false">also hidden</p></body></html>`,
		`<html hidden><body><p>hidden ancestor</p></body></html>`,
	} {
		if _, err := convertHTML(source); !errors.Is(err, errHTMLWithoutVisibleContent) {
			t.Fatalf("hidden-only HTML error = %v", err)
		}
	}
}
