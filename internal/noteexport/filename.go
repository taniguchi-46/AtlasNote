package noteexport

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maxSuggestedBaseBytes = 180

var windowsReservedFilenames = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {},
	"COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {},
	"LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
	"COM¹": {}, "COM²": {}, "COM³": {},
	"LPT¹": {}, "LPT²": {}, "LPT³": {},
}

func SuggestedFilename(title string, format Format) string {
	base := safeFilenameBase(title)
	return base + format.Extension()
}

func safeFilenameBase(title string) string {
	var builder strings.Builder
	for _, character := range strings.TrimSpace(title) {
		switch {
		case character < 0x20 || character == 0x7f:
			builder.WriteRune('_')
		case strings.ContainsRune(`<>:"/\|?*`, character):
			builder.WriteRune('_')
		default:
			builder.WriteRune(character)
		}
	}

	base := strings.TrimRight(builder.String(), " .")
	base = truncateUTF8(base, maxSuggestedBaseBytes)
	base = strings.TrimRight(base, " .")
	if base == "" || base == "." || base == ".." {
		base = "note"
	}

	deviceName := base
	if index := strings.IndexByte(deviceName, '.'); index >= 0 {
		deviceName = deviceName[:index]
	}
	deviceName = strings.TrimRight(deviceName, " .")
	if _, reserved := windowsReservedFilenames[strings.ToUpper(deviceName)]; reserved {
		base = "_" + base
	}
	return base
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func outputPath(path string, format Format) (string, *APIError) {
	if strings.TrimSpace(path) == "" {
		return "", apiError(ErrorCodeInvalidInput, "保存先を確認できません。", "path", false)
	}
	cleaned := filepath.Clean(path)
	base := filepath.Base(cleaned)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "", apiError(ErrorCodeInvalidInput, "保存先を確認できません。", "path", false)
	}
	extension := format.Extension()
	if extension == "" {
		return "", apiError(ErrorCodeInvalidFormat, "エクスポート形式を確認できません。", "format", false)
	}
	if !strings.EqualFold(filepath.Ext(cleaned), extension) {
		cleaned += extension
	}
	return cleaned, nil
}
