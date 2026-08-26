package noteimport

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
)

var (
	errHTMLWithoutVisibleContent = errors.New("html has no visible content")
	safeLanguageName             = regexp.MustCompile(`^[A-Za-z0-9_+-]{1,32}$`)
)

type htmlConversion struct {
	Content       string
	HeadingTitle  string
	MetadataTitle string
}

func convertHTML(source string) (htmlConversion, error) {
	document, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		return htmlConversion{}, err
	}

	body := findElement(document, "body")
	if body == nil {
		body = document
	}
	if hasHiddenAncestor(body) {
		return htmlConversion{}, errHTMLWithoutVisibleContent
	}

	renderer := htmlRenderer{}
	content := normalizeMarkdown(renderer.renderChildren(body, htmlRenderContext{}))
	if strings.TrimSpace(content) == "" {
		return htmlConversion{}, errHTMLWithoutVisibleContent
	}

	return htmlConversion{
		Content:       content,
		HeadingTitle:  firstHeadingTitle(body),
		MetadataTitle: documentMetadataTitle(document),
	}, nil
}

func findElement(root *xhtml.Node, name string) *xhtml.Node {
	if root == nil {
		return nil
	}
	if root.Type == xhtml.ElementNode && elementName(root) == name {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := findElement(child, name); found != nil {
			return found
		}
	}
	return nil
}

func documentMetadataTitle(document *xhtml.Node) string {
	head := findElement(document, "head")
	if head == nil {
		return ""
	}

	var title string
	var metaTitle string
	var visit func(*xhtml.Node)
	visit = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode {
			switch elementName(node) {
			case "title":
				if title == "" {
					title = normalizeTitle(rawText(node))
				}
			case "meta":
				name, _ := attribute(node, "name")
				content, _ := attribute(node, "content")
				if metaTitle == "" && strings.EqualFold(strings.TrimSpace(name), "title") {
					metaTitle = normalizeTitle(content)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(head)
	if title != "" {
		return title
	}
	return metaTitle
}

func firstHeadingTitle(root *xhtml.Node) string {
	var found string
	var visit func(*xhtml.Node)
	visit = func(node *xhtml.Node) {
		if found != "" || node == nil {
			return
		}
		if node.Type == xhtml.ElementNode {
			name := elementName(node)
			if isHiddenElement(node) || isDroppedElement(name) || name == "head" {
				return
			}
			if name == "h1" {
				if value := normalizeTitle(visibleText(node)); value != "" {
					found = value
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return found
}

type htmlRenderer struct{}

type htmlRenderContext struct {
	listDepth int
}

func (r htmlRenderer) renderChildren(node *xhtml.Node, context htmlRenderContext) string {
	if node == nil || isHiddenElement(node) {
		return ""
	}
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		builder.WriteString(r.renderNode(child, context))
	}
	return builder.String()
}

func (r htmlRenderer) renderNode(node *xhtml.Node, context htmlRenderContext) string {
	if node == nil {
		return ""
	}
	switch node.Type {
	case xhtml.TextNode:
		return escapeMarkdownText(collapseHTMLText(node.Data))
	case xhtml.ElementNode:
		name := elementName(node)
		if isHiddenElement(node) || isDroppedElement(name) || name == "head" {
			return ""
		}

		switch name {
		case "html", "body", "thead", "tbody", "tfoot", "tr", "th", "td":
			return r.renderChildren(node, context)
		case "h1", "h2", "h3", "h4", "h5", "h6":
			return r.renderHeading(node, int(name[1]-'0'), context)
		case "p", "div", "section", "article", "main", "header", "footer", "figure", "figcaption":
			return asBlock(r.renderChildren(node, context))
		case "br":
			return "  \n"
		case "strong", "b":
			return wrapInline("**", r.renderChildren(node, context))
		case "em", "i":
			return wrapInline("*", r.renderChildren(node, context))
		case "del", "s", "strike":
			return wrapInline("~~", r.renderChildren(node, context))
		case "blockquote":
			return r.renderBlockquote(node, context)
		case "ul":
			return r.renderList(node, false, context.listDepth)
		case "ol":
			return r.renderList(node, true, context.listDepth)
		case "code":
			return r.renderInlineCode(node)
		case "pre":
			return r.renderPre(node)
		case "hr":
			return "---\n\n"
		case "table":
			return r.renderTable(node)
		case "a":
			return r.renderLink(node, context)
		case "img":
			return renderImageAlt(node)
		case "input":
			return ""
		default:
			return r.renderChildren(node, context)
		}
	}
	return ""
}

func (r htmlRenderer) renderHeading(node *xhtml.Node, level int, context htmlRenderContext) string {
	content := strings.TrimSpace(r.renderChildren(node, context))
	if content == "" {
		return ""
	}
	return strings.Repeat("#", level) + " " + content + "\n\n"
}

func (r htmlRenderer) renderBlockquote(node *xhtml.Node, context htmlRenderContext) string {
	content := normalizeMarkdown(r.renderChildren(node, context))
	if strings.TrimSpace(content) == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	for index, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[index] = ">"
		} else {
			lines[index] = "> " + line
		}
	}
	return strings.Join(lines, "\n") + "\n\n"
}

func (r htmlRenderer) renderList(node *xhtml.Node, ordered bool, depth int) string {
	start := 1
	if ordered {
		if rawStart, ok := attribute(node, "start"); ok {
			if parsed, err := strconv.Atoi(strings.TrimSpace(rawStart)); err == nil && parsed > 0 {
				start = parsed
			}
		}
	}

	items := make([]string, 0)
	itemIndex := 0
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != xhtml.ElementNode || elementName(child) != "li" || isHiddenElement(child) {
			continue
		}
		items = append(items, r.renderListItem(child, ordered, start+itemIndex, depth))
		itemIndex++
	}
	if len(items) == 0 {
		return ""
	}
	return strings.Join(items, "\n") + "\n\n"
}

func (r htmlRenderer) renderListItem(node *xhtml.Node, ordered bool, number int, depth int) string {
	checkbox, checked := checkboxInListItem(node)
	var content strings.Builder
	nested := make([]string, 0)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode {
			name := elementName(child)
			if name == "ul" || name == "ol" {
				nested = append(nested, r.renderNode(child, htmlRenderContext{listDepth: depth + 1}))
				continue
			}
		}
		content.WriteString(r.renderNode(child, htmlRenderContext{listDepth: depth}))
	}

	text := normalizeMarkdown(content.String())
	text = strings.Trim(text, "\n")
	marker := "-"
	if ordered {
		marker = strconv.Itoa(number) + "."
	}
	if checkbox {
		if checked {
			marker = "- [x]"
		} else {
			marker = "- [ ]"
		}
	}

	indent := strings.Repeat("  ", depth)
	var builder strings.Builder
	if text == "" {
		builder.WriteString(indent + marker)
	} else {
		lines := strings.Split(text, "\n")
		builder.WriteString(indent + marker + " " + lines[0])
		continuationIndent := indent + "  "
		for _, line := range lines[1:] {
			builder.WriteByte('\n')
			if line != "" {
				builder.WriteString(continuationIndent + line)
			}
		}
	}

	for _, childList := range nested {
		childList = strings.Trim(childList, "\n")
		if childList == "" {
			continue
		}
		builder.WriteByte('\n')
		builder.WriteString(childList)
	}
	return builder.String()
}

func checkboxInListItem(node *xhtml.Node) (bool, bool) {
	var checked bool
	var found bool
	var visit func(*xhtml.Node)
	visit = func(current *xhtml.Node) {
		if found || current == nil {
			return
		}
		if current.Type == xhtml.ElementNode {
			name := elementName(current)
			if isHiddenElement(current) || name == "form" || isDroppedElement(name) && name != "input" {
				return
			}
			if name == "input" {
				inputType, _ := attribute(current, "type")
				if strings.EqualFold(strings.TrimSpace(inputType), "checkbox") {
					_, checked = attribute(current, "checked")
					found = true
					return
				}
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return found, checked
}

func (r htmlRenderer) renderInlineCode(node *xhtml.Node) string {
	content := strings.Join(strings.Fields(visibleRawText(node)), " ")
	if content == "" {
		return ""
	}
	fence := strings.Repeat("`", longestBacktickRun(content)+1)
	return fence + content + fence
}

func (r htmlRenderer) renderPre(node *xhtml.Node) string {
	code := node
	language := ""
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode && elementName(child) == "code" && !isHiddenElement(child) {
			code = child
			language = codeLanguage(child)
			break
		}
	}
	content := strings.ReplaceAll(visibleRawText(code), "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	if content == "" {
		return ""
	}
	fence := strings.Repeat("`", max(3, longestBacktickRun(content)+1))
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return fence + language + "\n" + content + fence + "\n\n"
}

func codeLanguage(node *xhtml.Node) string {
	classes, ok := attribute(node, "class")
	if !ok {
		return ""
	}
	for _, className := range strings.Fields(classes) {
		language, matched := strings.CutPrefix(className, "language-")
		if matched && safeLanguageName.MatchString(language) {
			return language
		}
	}
	return ""
}

func (r htmlRenderer) renderTable(node *xhtml.Node) string {
	rows := collectTableRows(node)
	if len(rows) == 0 {
		return ""
	}

	expectedColumns := len(rows[0].cells)
	complex := expectedColumns == 0
	for _, row := range rows {
		if row.complex || len(row.cells) != expectedColumns {
			complex = true
			break
		}
	}
	if complex {
		return r.renderPlainTableRows(rows)
	}

	lines := make([]string, 0, len(rows)+1)
	lines = append(lines, renderTableRow(r, rows[0]))
	separator := make([]string, expectedColumns)
	for index := range separator {
		separator[index] = "---"
	}
	lines = append(lines, "| "+strings.Join(separator, " | ")+" |")
	for _, row := range rows[1:] {
		lines = append(lines, renderTableRow(r, row))
	}
	return strings.Join(lines, "\n") + "\n\n"
}

type htmlTableRow struct {
	cells   []*xhtml.Node
	complex bool
}

func collectTableRows(table *xhtml.Node) []htmlTableRow {
	rows := make([]htmlTableRow, 0)
	if table == nil || isHiddenElement(table) {
		return rows
	}
	appendRow := func(row *xhtml.Node) {
		if isHiddenElement(row) {
			return
		}
		cells := make([]*xhtml.Node, 0)
		complex := false
		for child := row.FirstChild; child != nil; child = child.NextSibling {
			if child.Type != xhtml.ElementNode || isHiddenElement(child) {
				continue
			}
			name := elementName(child)
			if name != "th" && name != "td" {
				continue
			}
			if _, exists := attribute(child, "rowspan"); exists {
				complex = true
			}
			if _, exists := attribute(child, "colspan"); exists {
				complex = true
			}
			cells = append(cells, child)
		}
		rows = append(rows, htmlTableRow{cells: cells, complex: complex})
	}

	for child := table.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != xhtml.ElementNode || isHiddenElement(child) {
			continue
		}
		switch elementName(child) {
		case "tr":
			appendRow(child)
		case "thead", "tbody", "tfoot":
			for row := child.FirstChild; row != nil; row = row.NextSibling {
				if row.Type == xhtml.ElementNode && elementName(row) == "tr" && !isHiddenElement(row) {
					appendRow(row)
				}
			}
		}
	}
	return rows
}

func renderTableRow(renderer htmlRenderer, row htmlTableRow) string {
	cells := make([]string, 0, len(row.cells))
	for _, cell := range row.cells {
		value := normalizeMarkdown(renderer.renderChildren(cell, htmlRenderContext{}))
		if value == "" {
			value = visibleText(cell)
		}
		value = strings.ReplaceAll(value, "|", "\\|")
		value = strings.ReplaceAll(value, "\n", "\\n")
		cells = append(cells, strings.TrimSpace(value))
	}
	return "| " + strings.Join(cells, " | ") + " |"
}

func (r htmlRenderer) renderPlainTableRows(rows []htmlTableRow) string {
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		cells := make([]string, 0, len(row.cells))
		for _, cell := range row.cells {
			value := strings.ReplaceAll(visibleText(cell), "|", "\\|")
			if value != "" {
				cells = append(cells, value)
			}
		}
		if len(cells) > 0 {
			lines = append(lines, strings.Join(cells, " | "))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n\n"
}

func (r htmlRenderer) renderLink(node *xhtml.Node, context htmlRenderContext) string {
	label := strings.TrimSpace(r.renderChildren(node, context))
	if label == "" {
		return ""
	}
	href, ok := attribute(node, "href")
	if !ok || !isSafeLink(href) {
		return label
	}
	return "[" + label + "](" + escapeLinkDestination(strings.TrimSpace(href)) + ")"
}

func renderImageAlt(node *xhtml.Node) string {
	alt, ok := attribute(node, "alt")
	if !ok {
		return ""
	}
	return escapeMarkdownText(collapseHTMLText(alt))
}

func isSafeLink(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return false
	}
	if strings.HasPrefix(value, "#") {
		return len(value) > 1 && !strings.ContainsAny(value, " \t")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.Host != ""
	case "mailto", "tel", "atlasnote":
		return parsed.Opaque != "" || parsed.Host != "" || parsed.Path != ""
	default:
		return false
	}
}

func escapeLinkDestination(value string) string {
	return strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)").Replace(value)
}

func asBlock(content string) string {
	content = strings.Trim(content, "\n")
	if strings.TrimSpace(content) == "" {
		return ""
	}
	return content + "\n\n"
}

func wrapInline(marker string, content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	return marker + content + marker
}

func normalizeMarkdown(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	result := make([]string, 0, len(lines))
	inFence := false
	fence := ""
	previousBlank := false

	for _, line := range lines {
		if marker := codeFenceMarker(line); marker != "" {
			if !inFence {
				inFence = true
				fence = marker
			} else if marker == fence {
				inFence = false
				fence = ""
			}
			result = append(result, line)
			previousBlank = false
			continue
		}
		if inFence {
			result = append(result, line)
			continue
		}
		if strings.TrimSpace(line) == "" {
			if len(result) == 0 || previousBlank {
				continue
			}
			result = append(result, "")
			previousBlank = true
			continue
		}
		result = append(result, line)
		previousBlank = false
	}

	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}
	return strings.Join(result, "\n")
}

func codeFenceMarker(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "```") {
		return ""
	}
	length := 0
	for length < len(trimmed) && trimmed[length] == '`' {
		length++
	}
	if length < 3 {
		return ""
	}
	return strings.Repeat("`", length)
}

func longestBacktickRun(value string) int {
	longest := 0
	current := 0
	for _, character := range value {
		if character == '`' {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	return longest
}

func collapseHTMLText(value string) string {
	if value == "" {
		return ""
	}
	first, _ := utf8.DecodeRuneInString(value)
	last, _ := utf8.DecodeLastRuneInString(value)
	leadingSpace := unicode.IsSpace(first)
	trailingSpace := unicode.IsSpace(last)
	fields := strings.Fields(value)
	if len(fields) == 0 {
		if leadingSpace || trailingSpace {
			return " "
		}
		return ""
	}
	result := strings.Join(fields, " ")
	if leadingSpace {
		result = " " + result
	}
	if trailingSpace {
		result += " "
	}
	return result
}

func escapeMarkdownText(value string) string {
	return strings.NewReplacer(
		"\\", "\\\\",
		"`", "\\`",
		"*", "\\*",
		"_", "\\_",
		"[", "\\[",
		"]", "\\]",
		"<", "\\<",
		">", "\\>",
		"~", "\\~",
		"#", "\\#",
	).Replace(value)
}

func rawText(node *xhtml.Node) string {
	var builder strings.Builder
	var visit func(*xhtml.Node)
	visit = func(current *xhtml.Node) {
		if current == nil {
			return
		}
		if current.Type == xhtml.TextNode {
			builder.WriteString(current.Data)
			return
		}
		if current.Type == xhtml.ElementNode && isDroppedElement(elementName(current)) {
			return
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return builder.String()
}

func visibleRawText(node *xhtml.Node) string {
	var builder strings.Builder
	var visit func(*xhtml.Node)
	visit = func(current *xhtml.Node) {
		if current == nil {
			return
		}
		if current.Type == xhtml.TextNode {
			builder.WriteString(current.Data)
			return
		}
		if current.Type == xhtml.ElementNode && (isHiddenElement(current) || isDroppedElement(elementName(current))) {
			return
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return builder.String()
}

func visibleText(node *xhtml.Node) string {
	var builder strings.Builder
	var visit func(*xhtml.Node)
	visit = func(current *xhtml.Node) {
		if current == nil {
			return
		}
		if current.Type == xhtml.TextNode {
			builder.WriteString(current.Data)
			return
		}
		if current.Type == xhtml.ElementNode {
			name := elementName(current)
			if isHiddenElement(current) || isDroppedElement(name) || name == "head" || name == "input" {
				return
			}
			if name == "img" {
				if alt, ok := attribute(current, "alt"); ok {
					builder.WriteString(" " + alt + " ")
				}
				return
			}
			if name == "br" {
				builder.WriteByte(' ')
				return
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return strings.Join(strings.Fields(builder.String()), " ")
}

func elementName(node *xhtml.Node) string {
	return strings.ToLower(node.Data)
}

func attribute(node *xhtml.Node, name string) (string, bool) {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return attribute.Val, true
		}
	}
	return "", false
}

func isHiddenElement(node *xhtml.Node) bool {
	if node == nil || node.Type != xhtml.ElementNode {
		return false
	}
	_, hidden := attribute(node, "hidden")
	return hidden
}

func hasHiddenAncestor(node *xhtml.Node) bool {
	for current := node; current != nil; current = current.Parent {
		if isHiddenElement(current) {
			return true
		}
	}
	return false
}

func isDroppedElement(name string) bool {
	switch name {
	case "script", "style", "noscript", "template", "iframe", "object", "embed", "svg", "canvas", "video", "audio", "source", "track", "base",
		"form", "button", "select", "textarea", "option", "optgroup", "fieldset", "legend", "datalist", "output":
		return true
	default:
		return false
	}
}
