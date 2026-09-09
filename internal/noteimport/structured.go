package noteimport

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	structuredFormat       = "atlasnote-notes"
	structuredVersion      = 1
	maxStructuredRecords   = 1000
	maxImportedTitleRunes  = 200
	maxImportedContentSize = 2 * 1024 * 1024
)

type structuredRecord struct {
	Title   string
	Content string
}

type structuredParseError struct {
	recordNumber int
	message      string
}

func (err *structuredParseError) Error() string {
	return err.message
}

func parseStructuredCandidates(extension string, content string, sourceName string, titleMode TitleMode) ([]candidate, []FileFailure) {
	var (
		records []structuredRecord
		err     error
	)
	switch extension {
	case ".json":
		records, err = parseJSONRecords([]byte(content))
	case ".csv":
		records, err = parseCSVRecords([]byte(content))
	default:
		return nil, []FileFailure{{
			SourceName: sourceName,
			Code:       FailureCodeStructured,
			Message:    "構造化ファイルの形式を確認できません。",
		}}
	}
	if err != nil {
		failure := FileFailure{SourceName: sourceName, Code: FailureCodeStructured, Message: err.Error()}
		var parseErr *structuredParseError
		if errors.As(err, &parseErr) {
			failure.RecordNumber = parseErr.recordNumber
		}
		return nil, []FileFailure{failure}
	}

	validated := make([]candidate, 0, len(records))
	failures := make([]FileFailure, 0)
	for index, record := range records {
		recordNumber := index + 1
		if !utf8.ValidString(record.Title) || !utf8.ValidString(record.Content) {
			failures = append(failures, structuredFailure(sourceName, recordNumber, "レコードをUTF-8として検証できません。"))
			continue
		}
		if len([]rune(strings.TrimSpace(record.Title))) > maxImportedTitleRunes {
			failures = append(failures, structuredFailure(sourceName, recordNumber, "タイトルが長すぎます。"))
			continue
		}
		if len(record.Content) > maxImportedContentSize {
			failures = append(failures, structuredFailure(sourceName, recordNumber, "本文が大きすぎます。"))
			continue
		}

		headingTitle := titleFromFirstLine(record.Content)
		metadataTitle := strings.TrimSpace(record.Title)
		validated = append(validated, candidate{
			sourceName:   sourceName,
			title:        resolveStructuredTitle(titleMode, sourceName, headingTitle, metadataTitle),
			content:      record.Content,
			recordNumber: recordNumber,
		})
	}
	if len(failures) > 0 {
		// Structured files are validated as a whole. Returning no candidates is
		// what guarantees that a bad record cannot cause earlier records to be
		// persisted before the file-level validation completes.
		return nil, failures
	}
	return validated, nil
}

func resolveStructuredTitle(titleMode TitleMode, sourceName string, headingTitle string, metadataTitle string) string {
	if titleMode == TitleModeAuto {
		// Structured formats carry an explicit record title. It is the most
		// stable title source for a batch export, even when the Markdown body
		// also starts with a heading.
		return resolveTitle(TitleModeMetadata, sourceName, headingTitle, metadataTitle)
	}
	return resolveTitle(titleMode, sourceName, headingTitle, metadataTitle)
}

func structuredFailure(sourceName string, recordNumber int, message string) FileFailure {
	return FileFailure{
		SourceName:   sourceName,
		Code:         FailureCodeStructured,
		Message:      message,
		RecordNumber: recordNumber,
	}
}

func parseJSONRecords(data []byte) ([]structuredRecord, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("JSONファイルが空です。")
	}
	values, err := parseJSONObject(data, map[string]struct{}{
		"format":  {},
		"version": {},
		"notes":   {},
	})
	if err != nil {
		return nil, fmt.Errorf("JSONの構造を確認できません。")
	}

	var format string
	if err := json.Unmarshal(values["format"], &format); err != nil || format != structuredFormat {
		return nil, fmt.Errorf("JSONのformatがAtlas Noteの形式ではありません。")
	}
	var version int
	if err := json.Unmarshal(values["version"], &version); err != nil || version != structuredVersion {
		return nil, fmt.Errorf("JSONのversionが対応していません。")
	}

	notesValue := values["notes"]
	if len(notesValue) == 0 || bytes.Equal(bytes.TrimSpace(notesValue), []byte("null")) {
		return nil, fmt.Errorf("JSONのnotesは配列で指定してください。")
	}
	var rawRecords []json.RawMessage
	if err := json.Unmarshal(notesValue, &rawRecords); err != nil {
		return nil, fmt.Errorf("JSONのnotesは配列で指定してください。")
	}
	if len(rawRecords) > maxStructuredRecords {
		return nil, fmt.Errorf("JSONのレコード数が上限（%d件）を超えています。", maxStructuredRecords)
	}

	records := make([]structuredRecord, 0, len(rawRecords))
	for index, rawRecord := range rawRecords {
		values, err := parseJSONObject(rawRecord, map[string]struct{}{
			"title":   {},
			"content": {},
		})
		if err != nil {
			return nil, structuredRecordError(index+1, "JSONの%d件目のレコードを確認できません。", index+1)
		}
		var record structuredRecord
		if err := json.Unmarshal(values["title"], &record.Title); err != nil {
			return nil, structuredRecordError(index+1, "JSONの%d件目のtitleは文字列で指定してください。", index+1)
		}
		if err := json.Unmarshal(values["content"], &record.Content); err != nil {
			return nil, structuredRecordError(index+1, "JSONの%d件目のcontentは文字列で指定してください。", index+1)
		}
		records = append(records, record)
	}
	return records, nil
}

func parseJSONObject(data []byte, allowed map[string]struct{}) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	values := make(map[string]json.RawMessage, len(allowed))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("object required")
	}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("object key required")
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("duplicate key")
		}
		if _, known := allowed[key]; !known {
			return nil, fmt.Errorf("unknown key")
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		values[key] = raw
	}
	closing, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if closing != json.Delim('}') {
		return nil, fmt.Errorf("object closing delimiter required")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing JSON data")
		}
		return nil, err
	}
	return values, nil
}

func parseCSVRecords(data []byte) ([]structuredRecord, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("CSVファイルが空です。")
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = false
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("CSVのヘッダーを読み込めません。")
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\uFEFF")
	}
	if len(header) != 2 || header[0] != "title" || header[1] != "content" {
		return nil, fmt.Errorf("CSVの列はtitle,contentの順で指定してください。")
	}

	records := make([]structuredRecord, 0)
	for recordNumber := 1; ; recordNumber++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, structuredRecordError(recordNumber, "CSVの%d件目のレコードを読み込めません。", recordNumber)
		}
		if len(records) >= maxStructuredRecords {
			return nil, structuredRecordError(recordNumber, "CSVのレコード数が上限（%d件）を超えています。", maxStructuredRecords)
		}
		if len(row) != 2 {
			return nil, structuredRecordError(recordNumber, "CSVの%d件目は2列で指定してください。", recordNumber)
		}
		records = append(records, structuredRecord{Title: row[0], Content: row[1]})
	}
	return records, nil
}

func structuredRecordError(recordNumber int, format string, args ...any) error {
	return &structuredParseError{recordNumber: recordNumber, message: fmt.Sprintf(format, args...)}
}
