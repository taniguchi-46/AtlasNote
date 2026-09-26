package externalcmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"strings"

	"atlasnote/internal/config"
	"atlasnote/internal/localipc"
	"atlasnote/internal/readapi"
)

func Run(args []string, stdout, stderr io.Writer) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}
	if args[0] == "mcp" {
		return true, RunMCP(args[1:], io.Reader(nil), stdout, stderr)
	}
	if args[0] != "notes" && args[0] != "notebooks" && args[0] != "tags" && args[0] != "organize" && args[0] != "operations" {
		return false, 0
	}
	operation, params, parseErr := parseCommand(args, stderr)
	requestID, idErr := localipc.NewRequestID()
	if idErr != nil {
		requestID = "unavailable"
	}
	if parseErr != nil {
		response := readapi.Response{
			APIVersion: readapi.APIVersion, RequestID: requestID, Status: readapi.StatusRejected,
			Data:  map[string]any{},
			Error: &readapi.APIError{Code: "INVALID_ARGUMENT", Message: "コマンド引数が正しくありません。", Retryable: false},
		}
		writeJSON(stdout, response)
		return true, 2
	}
	if idErr != nil {
		response := connectionFailure(requestID)
		writeJSON(stdout, response)
		return true, 4
	}
	client, err := connectClient()
	if err != nil {
		writeJSON(stdout, connectionFailure(requestID))
		return true, 4
	}
	response, err := client.Call(context.Background(), operation, params, requestID)
	if err != nil {
		response := transportFailure(requestID, err)
		writeJSON(stdout, response)
		return true, exitCode(response)
	}
	writeJSON(stdout, response)
	return true, exitCode(response)
}

func parseCommand(args []string, stderr io.Writer) (string, any, error) {
	if len(args) >= 2 {
		changeOperations := map[string]string{
			"notes propose-edit":     readapi.OperationNotesProposeEdit,
			"notes request-create":   readapi.OperationNotesRequestCreate,
			"notes request-update":   readapi.OperationNotesRequestUpdate,
			"notes request-move":     readapi.OperationNotesRequestMove,
			"notes request-tags":     readapi.OperationNotesRequestTags,
			"notes request-trash":    readapi.OperationNotesRequestTrash,
			"organize request-apply": readapi.OperationOrganizeRequestApply,
		}
		if operation, ok := changeOperations[args[0]+" "+args[1]]; ok {
			flags := newFlags(args[0]+" "+args[1], stderr)
			input := flags.String("input", "", "JSON request, or '-' to read standard input")
			addJSONFlag(flags)
			if err := flags.Parse(args[2:]); err != nil || flags.NArg() != 0 || *input == "" {
				return "", nil, errors.New("invalid change request")
			}
			value := *input
			if value == "-" {
				body, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20+1))
				if err != nil || len(body) > 1<<20 {
					return "", nil, errors.New("invalid change input")
				}
				value = string(body)
			}
			var params map[string]any
			if len(value) > 1<<20 || json.Unmarshal([]byte(value), &params) != nil || params == nil {
				return "", nil, errors.New("invalid change input")
			}
			return operation, params, nil
		}
		if args[0] == "operations" && args[1] == "get" {
			if len(args) < 3 {
				return "", nil, errors.New("operation ID is required")
			}
			flags := newFlags("operations get", stderr)
			addJSONFlag(flags)
			if err := flags.Parse(args[3:]); err != nil || flags.NArg() != 0 {
				return "", nil, errors.New("invalid operations get arguments")
			}
			return readapi.OperationOperationsGet, map[string]string{"operationId": args[2]}, nil
		}
	}
	switch args[0] {
	case "notes":
		if len(args) < 2 {
			return "", nil, errors.New("notes subcommand is required")
		}
		switch args[1] {
		case "list":
			var input readapi.NoteListInput
			flags := newFlags("notes list", stderr)
			flags.StringVar(&input.SortBy, "sort", "", "sort field")
			flags.StringVar(&input.SortDirection, "direction", "", "sort direction")
			flags.IntVar(&input.Limit, "limit", 0, "result limit")
			flags.StringVar(&input.Cursor, "cursor", "", "page cursor")
			notebookID := flags.String("notebook", "", "notebook ID")
			tagID := flags.String("tag", "", "tag ID")
			addJSONFlag(flags)
			if err := flags.Parse(args[2:]); err != nil || flags.NArg() != 0 {
				return "", nil, errors.New("invalid notes list arguments")
			}
			input.NotebookID = optionalString(*notebookID)
			input.TagID = optionalString(*tagID)
			return readapi.OperationNotesList, input, nil
		case "get":
			if len(args) < 3 {
				return "", nil, errors.New("note ID is required")
			}
			input := readapi.NoteGetInput{NoteID: args[2]}
			flags := newFlags("notes get", stderr)
			expected := flags.Int64("expected-revision", 0, "expected revision")
			addJSONFlag(flags)
			if err := flags.Parse(args[3:]); err != nil || flags.NArg() != 0 {
				return "", nil, errors.New("invalid notes get arguments")
			}
			if *expected != 0 {
				input.ExpectedRevision = expected
			}
			return readapi.OperationNotesGet, input, nil
		case "search":
			if len(args) < 3 {
				return "", nil, errors.New("search query is required")
			}
			input := readapi.SearchInput{Query: args[2]}
			flags := newFlags("notes search", stderr)
			flags.StringVar(&input.Scope, "scope", "", "all or title")
			flags.StringVar(&input.SortBy, "sort", "", "sort field")
			flags.StringVar(&input.SortDirection, "direction", "", "sort direction")
			flags.IntVar(&input.Limit, "limit", 0, "result limit")
			flags.StringVar(&input.Cursor, "cursor", "", "page cursor")
			notebookID := flags.String("notebook", "", "notebook ID")
			addJSONFlag(flags)
			if err := flags.Parse(args[3:]); err != nil || flags.NArg() != 0 {
				return "", nil, errors.New("invalid notes search arguments")
			}
			input.NotebookID = optionalString(*notebookID)
			return readapi.OperationNotesSearch, input, nil
		case "backlinks":
			if len(args) < 3 {
				return "", nil, errors.New("note ID is required")
			}
			input := readapi.BacklinkInput{NoteID: args[2]}
			flags := newFlags("notes backlinks", stderr)
			flags.IntVar(&input.Limit, "limit", 0, "result limit")
			flags.StringVar(&input.Cursor, "cursor", "", "page cursor")
			addJSONFlag(flags)
			if err := flags.Parse(args[3:]); err != nil || flags.NArg() != 0 {
				return "", nil, errors.New("invalid notes backlinks arguments")
			}
			return readapi.OperationBacklinks, input, nil
		case "related":
			if len(args) < 3 {
				return "", nil, errors.New("note ID is required")
			}
			input := readapi.RelatedInput{NoteID: args[2]}
			flags := newFlags("notes related", stderr)
			flags.BoolVar(&input.Descendants, "descendants", false, "include notebook descendants")
			flags.IntVar(&input.Limit, "limit", 0, "result limit")
			notebookID := flags.String("notebook", "", "notebook ID")
			addJSONFlag(flags)
			if err := flags.Parse(args[3:]); err != nil || flags.NArg() != 0 {
				return "", nil, errors.New("invalid notes related arguments")
			}
			input.NotebookID = optionalString(*notebookID)
			return readapi.OperationRelated, input, nil
		}
	case "notebooks":
		if len(args) >= 2 && args[1] == "list" {
			var input readapi.NotebookListInput
			flags := newFlags("notebooks list", stderr)
			parentID := flags.String("parent", "", "parent notebook ID")
			flags.BoolVar(&input.IncludeDescendants, "descendants", false, "include descendants")
			flags.IntVar(&input.Limit, "limit", 0, "result limit")
			flags.StringVar(&input.Cursor, "cursor", "", "page cursor")
			addJSONFlag(flags)
			if err := flags.Parse(args[2:]); err != nil || flags.NArg() != 0 {
				return "", nil, errors.New("invalid notebooks list arguments")
			}
			input.ParentID = optionalString(*parentID)
			return readapi.OperationNotebooksList, input, nil
		}
	case "tags":
		if len(args) >= 2 && args[1] == "list" {
			var input readapi.TagListInput
			flags := newFlags("tags list", stderr)
			flags.IntVar(&input.Limit, "limit", 0, "result limit")
			flags.StringVar(&input.Cursor, "cursor", "", "page cursor")
			addJSONFlag(flags)
			if err := flags.Parse(args[2:]); err != nil || flags.NArg() != 0 {
				return "", nil, errors.New("invalid tags list arguments")
			}
			return readapi.OperationTagsList, input, nil
		}
	case "organize":
		if len(args) < 2 {
			return "", nil, errors.New("organize subcommand is required")
		}
		switch args[1] {
		case "analyze":
			input := readapi.OrganizeAnalyzeInput{Scope: "space"}
			flags := newFlags("organize analyze", stderr)
			flags.StringVar(&input.Scope, "scope", "space", "space, notebook, descendants, or note")
			flags.StringVar(&input.NotebookID, "notebook", "", "notebook ID")
			flags.StringVar(&input.NoteID, "note", "", "note ID")
			flags.IntVar(&input.Limit, "limit", 0, "candidate result limit")
			addJSONFlag(flags)
			if err := flags.Parse(args[2:]); err != nil || flags.NArg() != 0 {
				return "", nil, errors.New("invalid organize analyze arguments")
			}
			return readapi.OperationOrganizeAnalyze, input, nil
		case "candidates":
			if len(args) < 3 {
				return "", nil, errors.New("analysis ID is required")
			}
			input := readapi.OrganizeCandidatesInput{AnalysisID: args[2]}
			flags := newFlags("organize candidates", stderr)
			flags.StringVar(&input.Kind, "kind", "", "candidate kind")
			flags.IntVar(&input.Limit, "limit", 0, "candidate result limit")
			flags.StringVar(&input.Cursor, "cursor", "", "page cursor")
			addJSONFlag(flags)
			if err := flags.Parse(args[3:]); err != nil || flags.NArg() != 0 {
				return "", nil, errors.New("invalid organize candidates arguments")
			}
			return readapi.OperationOrganizeGetCandidates, input, nil
		}
	}
	return "", nil, errors.New("unknown read command")
}

func newFlags(name string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	return flags
}

func addJSONFlag(flags *flag.FlagSet) {
	flags.Bool("json", false, "write a single JSON response")
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func connectClient() (*localipc.Client, error) {
	resolution, err := config.ResolveStorageLocations()
	if err != nil || resolution.SetupRequired {
		return nil, errors.New("storage location is unavailable")
	}
	return localipc.Connect(resolution.Locations.DataRoot)
}

func connectionFailure(requestID string) readapi.Response {
	return readapi.Response{
		APIVersion: readapi.APIVersion, RequestID: requestID, Status: readapi.StatusError,
		Data:  map[string]any{},
		Error: &readapi.APIError{Code: "APP_NOT_RUNNING", Message: "Atlas Note本体へ接続できませんでした。", Retryable: true},
	}
}

func transportFailure(requestID string, err error) readapi.Response {
	if errors.Is(err, localipc.ErrAuthentication) {
		return readapi.Response{
			APIVersion: readapi.APIVersion, RequestID: requestID, Status: readapi.StatusRejected,
			Data:  map[string]any{},
			Error: &readapi.APIError{Code: "AUTHENTICATION_FAILED", Message: "Atlas Note本体との接続を認証できませんでした。", Retryable: false},
		}
	}
	return connectionFailure(requestID)
}

func writeJSON(writer io.Writer, value any) {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}

func exitCode(response readapi.Response) int {
	if response.Status == readapi.StatusOK || response.Status == readapi.StatusPending {
		return 0
	}
	if response.Status == readapi.StatusConflict {
		return 3
	}
	if response.Error != nil && (response.Error.Code == "INVALID_ARGUMENT" || response.Error.Code == "CURSOR_INVALID" ||
		response.Error.Code == "PERMISSION_DENIED" || response.Error.Code == "SCOPE_MISMATCH" || response.Error.Code == "RESOURCE_UNAVAILABLE" ||
		response.Error.Code == "ANALYSIS_UNAVAILABLE" || response.Error.Code == "AUTHENTICATION_FAILED") {
		return 2
	}
	return 4
}
