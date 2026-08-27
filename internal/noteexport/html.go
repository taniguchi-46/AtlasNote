package noteexport

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"atlasnote/internal/note"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const maxHTMLDepth = 100

var standaloneHTMLTemplate = template.Must(template.New("note-export").Parse(`<!doctype html>
<html lang="ja">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
  <title>{{.Title}}</title>
  <style>
    :root { color-scheme: light; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "Noto Sans JP", sans-serif; line-height: 1.65; }
    body { box-sizing: border-box; max-width: 52rem; margin: 0 auto; padding: 2.5rem 1.5rem; color: #202124; background: #fff; overflow-wrap: anywhere; }
    h1, h2, h3, h4, h5, h6 { line-height: 1.3; margin: 1.5em 0 .6em; }
    p, blockquote, ul, ol, pre, table { margin: 1em 0; }
    blockquote { margin-left: 0; padding-left: 1rem; border-left: .25rem solid #c7c9cc; color: #555; }
    pre, code { font-family: ui-monospace, SFMono-Regular, Consolas, "Liberation Mono", monospace; }
    pre { padding: 1rem; overflow-x: auto; background: #f5f6f7; border-radius: .4rem; white-space: pre-wrap; }
    :not(pre) > code { padding: .12em .3em; background: #f5f6f7; border-radius: .25rem; }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: .45rem .6rem; border: 1px solid #cfd2d6; text-align: left; vertical-align: top; }
    th { background: #f5f6f7; }
    a { color: #1558b0; text-decoration: underline; }
    hr { border: 0; border-top: 1px solid #cfd2d6; margin: 2rem 0; }
  </style>
</head>
<body>{{.Body}}</body>
</html>
`))

var allowedHTMLTags = map[string]struct{}{
	"h1": {}, "h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {},
	"p": {}, "br": {}, "strong": {}, "em": {}, "del": {},
	"blockquote": {}, "ul": {}, "ol": {}, "li": {}, "pre": {}, "code": {},
	"hr": {}, "table": {}, "thead": {}, "tbody": {}, "tfoot": {},
	"tr": {}, "th": {}, "td": {}, "a": {},
}

var discardedHTMLTags = map[string]struct{}{
	"script": {}, "style": {}, "noscript": {}, "template": {},
	"iframe": {}, "object": {}, "embed": {}, "svg": {}, "canvas": {},
	"video": {}, "audio": {}, "source": {}, "track": {}, "base": {},
	"form": {}, "input": {}, "button": {}, "select": {}, "option": {},
	"optgroup": {}, "textarea": {}, "label": {}, "fieldset": {}, "legend": {},
	"datalist": {}, "output": {}, "progress": {}, "meter": {},
}

func renderHTMLDocument(title string, fragment string) ([]byte, error) {
	sanitized, err := sanitizeHTMLFragment(fragment)
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer
	err = standaloneHTMLTemplate.Execute(&output, struct {
		Title string
		Body  template.HTML
	}{Title: title, Body: template.HTML(sanitized)})
	if err != nil {
		return nil, fmt.Errorf("render standalone HTML: %w", err)
	}
	return output.Bytes(), nil
}

func sanitizeHTMLFragment(fragment string) (string, error) {
	contextNode := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(fragment), contextNode)
	if err != nil {
		return "", fmt.Errorf("parse HTML fragment: %w", err)
	}
	if err := validateHTMLDepth(nodes); err != nil {
		return "", err
	}

	var output bytes.Buffer
	for _, current := range nodes {
		for _, sanitized := range sanitizeHTMLNode(current) {
			if err := html.Render(&output, sanitized); err != nil {
				return "", fmt.Errorf("render sanitized HTML: %w", err)
			}
		}
	}
	return output.String(), nil
}

func validateHTMLDepth(nodes []*html.Node) error {
	type nodeDepth struct {
		node  *html.Node
		depth int
	}
	stack := make([]nodeDepth, 0, len(nodes))
	for _, node := range nodes {
		stack = append(stack, nodeDepth{node: node, depth: 1})
	}
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current.depth > maxHTMLDepth {
			return fmt.Errorf("HTML fragment nesting exceeds %d nodes", maxHTMLDepth)
		}
		for child := current.node.FirstChild; child != nil; child = child.NextSibling {
			stack = append(stack, nodeDepth{node: child, depth: current.depth + 1})
		}
	}
	return nil
}

func sanitizeHTMLNode(current *html.Node) []*html.Node {
	switch current.Type {
	case html.TextNode:
		return []*html.Node{{Type: html.TextNode, Data: current.Data}}
	case html.ElementNode:
		tag := strings.ToLower(current.Data)
		if hasHTMLAttribute(current, "hidden") {
			return nil
		}
		if _, discarded := discardedHTMLTags[tag]; discarded {
			return nil
		}
		if tag == "img" {
			if alt := attributeValue(current, "alt"); alt != "" {
				return []*html.Node{{Type: html.TextNode, Data: alt}}
			}
			return nil
		}
		if tag == "s" || tag == "strike" {
			tag = "del"
		}
		if _, allowed := allowedHTMLTags[tag]; !allowed {
			return sanitizeHTMLChildren(current)
		}

		clean := &html.Node{Type: html.ElementNode, Data: tag}
		if tag == "a" {
			if href := attributeValue(current, "href"); safeHTMLHref(href) {
				clean.Attr = []html.Attribute{
					{Key: "href", Val: href},
					{Key: "rel", Val: "noopener noreferrer"},
				}
			}
		}
		if tag == "ol" {
			if start, err := strconv.ParseInt(attributeValue(current, "start"), 10, 32); err == nil && start > 0 {
				clean.Attr = []html.Attribute{{Key: "start", Val: strconv.FormatInt(start, 10)}}
			}
		}
		if tag == "li" {
			switch strings.ToLower(attributeValue(current, "data-checked")) {
			case "true":
				clean.AppendChild(&html.Node{Type: html.TextNode, Data: "[x] "})
			case "false":
				clean.AppendChild(&html.Node{Type: html.TextNode, Data: "[ ] "})
			}
		}
		for _, child := range sanitizeHTMLChildren(current) {
			clean.AppendChild(child)
		}
		return []*html.Node{clean}
	default:
		return nil
	}
}

func sanitizeHTMLChildren(parent *html.Node) []*html.Node {
	children := make([]*html.Node, 0)
	for child := parent.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, sanitizeHTMLNode(child)...)
	}
	return children
}

func hasHTMLAttribute(current *html.Node, key string) bool {
	for _, attribute := range current.Attr {
		if strings.EqualFold(attribute.Key, key) {
			return true
		}
	}
	return false
}

func attributeValue(current *html.Node, key string) string {
	for _, attribute := range current.Attr {
		if strings.EqualFold(attribute.Key, key) {
			return attribute.Val
		}
	}
	return ""
}

func safeHTMLHref(href string) bool {
	if href == "" || strings.TrimSpace(href) != href || strings.IndexFunc(href, unicode.IsControl) >= 0 {
		return false
	}
	if strings.HasPrefix(href, "#") {
		return len(href) > 1 && !strings.ContainsAny(href, " \t\r\n")
	}
	if strings.HasPrefix(href, "atlasnote://note/") {
		base := href
		if index := strings.IndexByte(base, '#'); index >= 0 {
			if index == len(base)-1 || strings.ContainsAny(base[index+1:], " \t\r\n#") {
				return false
			}
			base = base[:index]
		}
		_, valid := note.ParseNoteLinkHref(base)
		return valid
	}

	parsed, err := url.Parse(href)
	if err != nil || parsed.Scheme == "" {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.Host != "" && parsed.Opaque == "" && parsed.User == nil
	case "mailto", "tel":
		return parsed.Opaque != "" && parsed.Host == "" && parsed.User == nil
	default:
		return false
	}
}
