package externalcmd

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"atlasnote/internal/localipc"
	"atlasnote/internal/readapi"
)

const supportedMCPProtocolVersion = "2025-06-18"

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema"`
}

func RunMCP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	scope, err := parseMCPScope(args, stderr)
	if err != nil {
		return 2
	}
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	var client *localipc.Client
	defer func() {
		if client == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = client.RevokeMCPSession(ctx)
		cancel()
	}()
	bindingAttempted := false
	initializeSucceeded := false
	initialized := false
	for scanner.Scan() {
		line := scanner.Bytes()
		var request jsonRPCRequest
		if err := json.Unmarshal(line, &request); err != nil || request.JSONRPC != "2.0" || strings.TrimSpace(request.Method) == "" {
			_ = encoder.Encode(jsonRPCResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &jsonRPCError{Code: -32700, Message: "Parse error"}})
			continue
		}
		if len(request.ID) == 0 {
			if request.Method == "notifications/initialized" && initializeSucceeded {
				initialized = true
			}
			continue
		}
		response := jsonRPCResponse{JSONRPC: "2.0", ID: request.ID}
		switch request.Method {
		case "initialize":
			if bindingAttempted {
				response.Error = &jsonRPCError{Code: -32001, Message: "Atlas Noteとの接続を再開するにはMCPプロセスを再起動してください。"}
				break
			}
			bindingAttempted = true
			var params struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			if err := json.Unmarshal(request.Params, &params); err != nil {
				response.Error = &jsonRPCError{Code: -32602, Message: "Invalid params"}
				break
			}
			rootClient, connectErr := connectClient()
			if connectErr != nil {
				response.Error = &jsonRPCError{Code: -32001, Message: "Atlas Note本体へ接続できませんでした。MCPプロセスを再起動してください。"}
				break
			}
			client, connectErr = rootClient.CreateMCPSession(context.Background(), scope)
			if connectErr != nil {
				response.Error = &jsonRPCError{Code: -32001, Message: "Atlas Note本体との読み取りセッションを開始できませんでした。MCPプロセスを再起動してください。"}
				break
			}
			initializeSucceeded = true
			response.Result = map[string]any{
				"protocolVersion": supportedMCPProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
				"serverInfo":      map[string]any{"name": "atlasnote", "version": "1"},
				"instructions":    "Atlas Note本体が公開する、保存空間に限定された読み取り専用ツールです。",
			}
		case "ping":
			response.Result = map[string]any{}
		case "tools/list":
			if !initialized {
				response.Error = &jsonRPCError{Code: -32002, Message: "Server not initialized"}
				break
			}
			response.Result = map[string]any{"tools": mcpTools()}
		case "tools/call":
			if !initialized {
				response.Error = &jsonRPCError{Code: -32002, Message: "Server not initialized"}
				break
			}
			response.Result = callMCPTool(request.Params, client)
		default:
			response.Error = &jsonRPCError{Code: -32601, Message: "Method not found"}
		}
		if err := encoder.Encode(response); err != nil {
			return 4
		}
	}
	if err := scanner.Err(); err != nil {
		_, _ = fmt.Fprintln(stderr, "MCP標準入力を読み取れませんでした。")
		return 4
	}
	return 0
}

func callMCPTool(raw json.RawMessage, client *localipc.Client) any {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return mcpErrorResult("INVALID_ARGUMENT", "ツール引数が正しくありません。", false, "")
	}
	operation, ok := mcpOperation(params.Name)
	if !ok {
		return mcpErrorResult("OPERATION_NOT_ALLOWED", "このツールは公開されていません。", false, "")
	}
	requestID, err := localipc.NewRequestID()
	if err != nil {
		return mcpErrorResult("READ_UNAVAILABLE", "読み取り処理を開始できませんでした。", true, "unavailable")
	}
	if len(params.Arguments) == 0 || string(params.Arguments) == "null" {
		params.Arguments = json.RawMessage("{}")
	}
	var arguments any
	decoder := json.NewDecoder(strings.NewReader(string(params.Arguments)))
	decoder.UseNumber()
	if err := decoder.Decode(&arguments); err != nil {
		return mcpErrorResult("INVALID_ARGUMENT", "ツール引数が正しくありません。", false, requestID)
	}
	if client == nil {
		return mcpResponseResult(mcpRestartRequired(requestID))
	}
	response, err := client.Call(context.Background(), operation, arguments, requestID)
	if err != nil {
		return mcpResponseResult(mcpRestartRequired(requestID))
	}
	return mcpResponseResult(response)
}

func mcpRestartRequired(requestID string) readapi.Response {
	return readapi.Response{
		APIVersion: readapi.APIVersion, RequestID: requestID, Status: readapi.StatusError,
		Data: map[string]any{},
		Error: &readapi.APIError{
			Code: "MCP_RESTART_REQUIRED", Message: "Atlas Noteとの接続が終了しました。MCPプロセスを再起動してください。", Retryable: false,
		},
	}
}

type publishedIDs []string

func (values *publishedIDs) String() string {
	return strings.Join(*values, ",")
}

func (values *publishedIDs) Set(value string) error {
	value = strings.TrimSpace(value)
	if len(value) != 32 || value != strings.ToLower(value) {
		return errors.New("ID must be 32 lowercase hexadecimal characters")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return errors.New("ID must be 32 lowercase hexadecimal characters")
	}
	for _, existing := range *values {
		if existing == value {
			return nil
		}
	}
	*values = append(*values, value)
	return nil
}

func parseMCPScope(args []string, stderr io.Writer) (localipc.MCPSessionScope, error) {
	flags := flag.NewFlagSet("mcp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var noteIDs publishedIDs
	var notebookIDs publishedIDs
	flags.Var(&noteIDs, "note", "publish one note ID to this MCP process (repeatable)")
	flags.Var(&notebookIDs, "notebook", "publish one notebook ID to this MCP process (repeatable)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || len(noteIDs)+len(notebookIDs) > 256 {
		return localipc.MCPSessionScope{}, errors.New("invalid MCP publication scope")
	}
	return localipc.MCPSessionScope{NoteIDs: noteIDs, NotebookIDs: notebookIDs}, nil
}

func mcpResponseResult(response readapi.Response) map[string]any {
	encoded, _ := json.Marshal(response)
	result := map[string]any{
		"content":           []map[string]any{{"type": "text", "text": string(encoded)}},
		"structuredContent": response,
	}
	if response.Status != readapi.StatusOK {
		result["isError"] = true
	}
	return result
}

func mcpErrorResult(code, message string, retryable bool, requestID string) map[string]any {
	if requestID == "" {
		requestID = "unavailable"
	}
	return mcpResponseResult(readapi.Response{
		APIVersion: readapi.APIVersion, RequestID: requestID, Status: readapi.StatusRejected,
		Data:  map[string]any{},
		Error: &readapi.APIError{Code: code, Message: message, Retryable: retryable},
	})
}

func mcpOperation(name string) (string, bool) {
	switch name {
	case readapi.OperationNotesList, readapi.OperationNotesGet, readapi.OperationNotesSearch,
		readapi.OperationNotebooksList, readapi.OperationTagsList, readapi.OperationBacklinks, readapi.OperationRelated:
		return name, true
	default:
		return "", false
	}
}

func mcpTools() []mcpTool {
	objectOutput := map[string]any{"type": "object", "additionalProperties": true}
	return []mcpTool{
		{Name: readapi.OperationNotesList, Description: "保護・ロック・ゴミ箱を除外してノート一覧を取得します。", InputSchema: schema(map[string]any{
			"notebookId": idProperty(), "tagId": idProperty(), "sortBy": enumProperty("updatedAt", "createdAt", "title"),
			"sortDirection": enumProperty("asc", "desc"), "limit": limitProperty(100), "cursor": stringProperty(),
		}, nil), OutputSchema: objectOutput},
		{Name: readapi.OperationNotesGet, Description: "指定revisionを検証し、許可されたノート本文とタグを取得します。", InputSchema: schema(map[string]any{
			"noteId": idProperty(), "expectedRevision": map[string]any{"type": "integer", "minimum": 1},
		}, []string{"noteId"}), OutputSchema: objectOutput},
		{Name: readapi.OperationNotesSearch, Description: "許可されたノートを検索し、検証済みの抜粋を返します。", InputSchema: schema(map[string]any{
			"query": map[string]any{"type": "string", "maxLength": 200}, "scope": enumProperty("all", "title"),
			"notebookId": idProperty(), "sortBy": enumProperty("updatedAt", "createdAt", "title"),
			"sortDirection": enumProperty("asc", "desc"), "limit": limitProperty(100), "cursor": stringProperty(),
		}, []string{"query"}), OutputSchema: objectOutput},
		{Name: readapi.OperationNotebooksList, Description: "許可されたノートブック一覧を取得します。", InputSchema: schema(map[string]any{
			"parentId": idProperty(), "includeDescendants": map[string]any{"type": "boolean"}, "limit": limitProperty(100), "cursor": stringProperty(),
		}, nil), OutputSchema: objectOutput},
		{Name: readapi.OperationTagsList, Description: "タグ一覧を取得します。", InputSchema: schema(map[string]any{
			"limit": limitProperty(100), "cursor": stringProperty(),
		}, nil), OutputSchema: objectOutput},
		{Name: readapi.OperationBacklinks, Description: "検証済みのバックリンク元ノートを取得します。", InputSchema: schema(map[string]any{
			"noteId": idProperty(), "limit": limitProperty(100), "cursor": stringProperty(),
		}, []string{"noteId"}), OutputSchema: objectOutput},
		{Name: readapi.OperationRelated, Description: "リンク・タグ・語句に基づく関連ノート候補を取得します。", InputSchema: schema(map[string]any{
			"noteId": idProperty(), "notebookId": idProperty(), "descendants": map[string]any{"type": "boolean"}, "limit": limitProperty(20),
		}, []string{"noteId"}), OutputSchema: objectOutput},
	}
}

func schema(properties map[string]any, required []string) map[string]any {
	result := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		result["required"] = required
	}
	return result
}

func idProperty() map[string]any {
	return map[string]any{"type": "string", "pattern": "^[0-9a-f]{32}$"}
}

func stringProperty() map[string]any {
	return map[string]any{"type": "string"}
}

func enumProperty(values ...string) map[string]any {
	items := make([]any, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	return map[string]any{"type": "string", "enum": items}
}

func limitProperty(maximum int) map[string]any {
	return map[string]any{"type": "integer", "minimum": 1, "maximum": maximum}
}
