package note

import "strings"

const noteLinkHrefPrefix = "atlasnote://note/"

const noteIDLength = 32

// ExtractNoteLinkTargets returns the unique note IDs referenced by canonical
// internal Markdown links. Links inside code blocks, inline code, escaped
// brackets, images, and non-Atlas URLs are intentionally ignored.
func ExtractNoteLinkTargets(markdown string) []string {
	targets := make([]string, 0)
	seen := make(map[string]struct{})
	inFence := byte(0)

	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if marker, ok := fenceMarker(trimmed); ok {
			if inFence == 0 {
				inFence = marker
			} else if inFence == marker {
				inFence = 0
			}
			continue
		}
		if inFence != 0 {
			continue
		}

		extractLineTargets(line, seen, &targets)
	}

	return targets
}

// ParseNoteLinkHref validates an internal link href and returns its target ID.
func ParseNoteLinkHref(href string) (string, bool) {
	if !strings.HasPrefix(href, noteLinkHrefPrefix) {
		return "", false
	}

	id := strings.TrimPrefix(href, noteLinkHrefPrefix)
	if len(id) != noteIDLength {
		return "", false
	}
	for _, character := range id {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return "", false
		}
	}

	return id, true
}

func extractLineTargets(line string, seen map[string]struct{}, targets *[]string) {
	inlineCodeTicks := 0
	for index := 0; index < len(line); {
		if line[index] == '`' {
			tickCount := countRun(line, index, '`')
			if inlineCodeTicks == 0 {
				inlineCodeTicks = tickCount
			} else if inlineCodeTicks == tickCount {
				inlineCodeTicks = 0
			}
			index += tickCount
			continue
		}
		if inlineCodeTicks != 0 {
			index++
			continue
		}
		if line[index] != '[' || isEscaped(line, index) || (index > 0 && line[index-1] == '!') {
			index++
			continue
		}

		closingBracket := findClosingBracket(line, index+1)
		if closingBracket < 0 || closingBracket+1 >= len(line) || line[closingBracket+1] != '(' {
			index++
			continue
		}

		hrefStart := closingBracket + 2
		hrefEnd := hrefStart + len(noteLinkHrefPrefix) + noteIDLength
		if hrefEnd >= len(line) || line[hrefEnd] != ')' {
			index++
			continue
		}
		if targetID, ok := ParseNoteLinkHref(line[hrefStart:hrefEnd]); ok {
			if _, exists := seen[targetID]; !exists {
				seen[targetID] = struct{}{}
				*targets = append(*targets, targetID)
			}
		}
		index = hrefEnd + 1
	}
}

func fenceMarker(line string) (byte, bool) {
	if len(line) < 3 {
		return 0, false
	}
	if line[0] == '`' && strings.HasPrefix(line, "```") {
		return '`', true
	}
	if line[0] == '~' && strings.HasPrefix(line, "~~~") {
		return '~', true
	}
	return 0, false
}

func findClosingBracket(line string, start int) int {
	for index := start; index < len(line); index++ {
		if line[index] == ']' && !isEscaped(line, index) {
			return index
		}
	}
	return -1
}

func countRun(value string, start int, character byte) int {
	count := 0
	for start+count < len(value) && value[start+count] == character {
		count++
	}
	return count
}

func isEscaped(value string, index int) bool {
	backslashes := 0
	for index > 0 && value[index-1] == '\\' {
		backslashes++
		index--
	}
	return backslashes%2 == 1
}
