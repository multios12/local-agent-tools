package calendarmcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"local-agent-tools/internal/calendar"
)

const protocolVersion = "2024-11-05"

// stdio で Google Calendar API を公開する MCP サーバー
type Server struct {
	svc    *calendar.Service // 予定作成サービス
	logger *log.Logger       // ロガー
}

// MCP サーバーを生成する
func New(svc *calendar.Service, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Server{svc: svc, logger: logger}
}

// MCP プロトコルのループを実行する
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	s.logger.Print("serve loop entered")
	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		body, err := readFrame(br)
		if err != nil {
			if errors.Is(err, io.EOF) {
				s.logger.Print("input closed")
				return nil
			}
			s.logger.Printf("read frame failed: %v", err)
			return err
		}
		s.logger.Printf("read frame bytes=%d", len(body))

		var req rpcRequest
		if err := json.Unmarshal(body, &req); err != nil {
			s.logger.Printf("parse request failed: %v", err)
			if err := writeResponse(bw, rpcResponse{
				JSONRPC: "2.0",
				Error: &rpcError{
					Code:    -32700,
					Message: "Parse error",
				},
			}); err != nil {
				return err
			}
			continue
		}

		if req.JSONRPC != "2.0" {
			s.logger.Printf("invalid request jsonrpc=%q hasID=%t", req.JSONRPC, req.hasID())
			if req.hasID() {
				if err := writeErrorResponse(bw, req.ID, -32600, "Invalid Request", "jsonrpc must be 2.0"); err != nil {
					return err
				}
			}
			continue
		}

		if !req.hasID() {
			s.logger.Printf("received notification method=%s", req.Method)
			s.handleNotification(req)
			continue
		}

		s.logger.Printf("received request method=%s", req.Method)
		resp, err := s.handleRequest(ctx, req)
		if err != nil {
			s.logger.Printf("handle request method=%s failed: %v", req.Method, err)
			return err
		}
		if resp != nil {
			if err := writeResponse(bw, *resp); err != nil {
				s.logger.Printf("write response method=%s failed: %v", req.Method, err)
				return err
			}
			s.logger.Printf("wrote response method=%s error=%t", req.Method, resp.Error != nil)
		}
	}
}

func (s *Server) handleNotification(req rpcRequest) {
	switch req.Method {
	case "notifications/initialized":
		return
	default:
		s.logger.Printf("ignoring notification method=%s", req.Method)
	}
}

func (s *Server) handleRequest(ctx context.Context, req rpcRequest) (*rpcResponse, error) {
	switch req.Method {
	case "initialize":
		return okResponse(req.ID, initializeResult{
			ProtocolVersion: protocolVersion,
			ServerInfo: serverInfo{
				Name:    "local-agent-tools",
				Version: "0.2.0",
			},
			Capabilities: capabilities{
				Tools:     toolsCapability{},
				Resources: resourcesCapability{},
				Prompts:   promptsCapability{},
			},
		}), nil
	case "resources/list":
		return okResponse(req.ID, resourcesListResult{Resources: []resourceDefinition{}}), nil
	case "prompts/list":
		return okResponse(req.ID, promptsListResult{Prompts: []promptDefinition{}}), nil
	case "tools/list":
		return okResponse(req.ID, toolsListResult{Tools: []toolDefinition{
			createCalendarEventToolDefinition(),
		}}), nil
	case "tools/call":
		return s.handleToolCall(ctx, req.ID, req.Params)
	default:
		return errorResponse(req.ID, -32601, "Method not found", fmt.Sprintf("unknown method %q", req.Method)), nil
	}
}

func (s *Server) handleToolCall(ctx context.Context, id json.RawMessage, rawParams json.RawMessage) (*rpcResponse, error) {
	var params toolCallParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return errorResponse(id, -32602, "Invalid params", "tool call parameters must be an object"), nil
	}

	switch params.Name {
	case "create_calendar_event":
		return s.callCreateCalendarEvent(ctx, id, params.Arguments)
	default:
		return errorResponse(id, -32602, "Invalid params", fmt.Sprintf("unknown tool %q", params.Name)), nil
	}
}

func (s *Server) callCreateCalendarEvent(ctx context.Context, id json.RawMessage, rawArgs json.RawMessage) (*rpcResponse, error) {
	var req calendar.CreateCalendarEventRequest
	if len(rawArgs) > 0 && string(rawArgs) != "null" {
		if err := json.Unmarshal(rawArgs, &req); err != nil {
			return okResponse(id, toolCallResult{
				IsError: true,
				Content: []toolContent{{
					Type: "text",
					Text: marshalText(map[string]any{
						"error": map[string]any{
							"code":    "invalid_params",
							"message": "tool arguments must match the calendar event schema",
						},
					}),
				}},
			}), nil
		}
	}

	resp, err := s.svc.CreateCalendarEvent(ctx, req)
	if err != nil {
		if appErr, ok := calendar.IsAppError(err); ok {
			return okResponse(id, toolCallResult{
				IsError: true,
				Content: []toolContent{{
					Type: "text",
					Text: marshalText(map[string]any{
						"error": map[string]any{
							"code":    appErr.Code,
							"message": appErr.Message,
							"details": appErr.Details,
						},
					}),
				}},
			}), nil
		}
		return okResponse(id, toolCallResult{
			IsError: true,
			Content: []toolContent{{
				Type: "text",
				Text: marshalText(map[string]any{
					"error": map[string]any{
						"code":    "internal_error",
						"message": "internal error",
					},
				}),
			}},
		}), nil
	}

	return okResponse(id, toolCallResult{Content: []toolContent{{Type: "text", Text: marshalText(resp)}}}), nil
}

func createCalendarEventToolDefinition() toolDefinition {
	return toolDefinition{
		Name:        "create_calendar_event",
		Description: "Create an event in the configured Google Calendar.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"summary", "start", "end"},
			"properties": map[string]any{
				"summary": map[string]any{
					"type":        "string",
					"minLength":   1,
					"maxLength":   200,
					"description": "Event title",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Optional event description",
				},
				"location": map[string]any{
					"type":        "string",
					"description": "Optional event location",
				},
				"calendarId": map[string]any{
					"type":        "string",
					"description": "Must match the configured target calendar if provided",
				},
				"start": eventDateTimeSchema(),
				"end":   eventDateTimeSchema(),
				"attendees": map[string]any{
					"type":  "array",
					"items": attendeeSchema(),
				},
				"sendUpdates": map[string]any{
					"type":        "string",
					"enum":        []string{"all", "externalOnly", "none"},
					"default":     "none",
					"description": "How Google Calendar should notify attendees",
				},
			},
		},
	}
}

func eventDateTimeSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Use either date for all-day events or dateTime for timed events",
		"oneOf": []map[string]any{
			{"required": []string{"date"}},
			{"required": []string{"dateTime"}},
		},
		"properties": map[string]any{
			"date": map[string]any{
				"type":        "string",
				"format":      "date",
				"description": "YYYY-MM-DD date for all-day events",
			},
			"dateTime": map[string]any{
				"type":        "string",
				"format":      "date-time",
				"description": "RFC3339 timestamp",
			},
			"timeZone": map[string]any{
				"type":        "string",
				"description": "IANA time zone name",
			},
		},
	}
}

func attendeeSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"email"},
		"properties": map[string]any{
			"email": map[string]any{
				"type":   "string",
				"format": "email",
			},
			"displayName": map[string]any{
				"type": "string",
			},
		},
	}
}

func marshalText(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":{"code":"internal_error","message":"%s"}}`, err.Error())
	}
	return string(b)
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`          // JSON-RPC バージョン
	ID      json.RawMessage `json:"id,omitempty"`     // 呼び出し ID
	Method  string          `json:"method"`           // メソッド名
	Params  json.RawMessage `json:"params,omitempty"` // 引数
}

func (r rpcRequest) hasID() bool {
	return len(r.ID) > 0
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`          // JSON-RPC バージョン
	ID      json.RawMessage `json:"id,omitempty"`     // 呼び出し ID
	Result  any             `json:"result,omitempty"` // 正常結果
	Error   *rpcError       `json:"error,omitempty"`  // エラー情報
}

type rpcError struct {
	Code    int    `json:"code"`           // エラーコード
	Message string `json:"message"`        // メッセージ
	Data    any    `json:"data,omitempty"` // 補足情報
}

type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"` // プロトコルバージョン
	ServerInfo      serverInfo   `json:"serverInfo"`      // サーバー情報
	Capabilities    capabilities `json:"capabilities"`    // 機能一覧
}

type serverInfo struct {
	Name    string `json:"name"`    // サーバー名
	Version string `json:"version"` // バージョン
}

type capabilities struct {
	Tools     toolsCapability     `json:"tools"`     // tools 機能
	Resources resourcesCapability `json:"resources"` // resources 機能
	Prompts   promptsCapability   `json:"prompts"`   // prompts 機能
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"` // ツール一覧変更通知の有無
}

type resourcesCapability struct {
	ListChanged bool `json:"listChanged,omitempty"` // リソース一覧変更通知の有無
}

type promptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"` // プロンプト一覧変更通知の有無
}

type toolsListResult struct {
	Tools []toolDefinition `json:"tools"` // tool 一覧
}

type resourcesListResult struct {
	Resources []resourceDefinition `json:"resources"` // resource 一覧
}

type promptsListResult struct {
	Prompts []promptDefinition `json:"prompts"` // prompt 一覧
}

type toolDefinition struct {
	Name        string `json:"name"`        // tool 名
	Description string `json:"description"` // tool の説明
	InputSchema any    `json:"inputSchema"` // 入力スキーマ
}

type resourceDefinition struct{}

type promptDefinition struct{}

type toolCallParams struct {
	Name      string          `json:"name"`      // tool 名
	Arguments json.RawMessage `json:"arguments"` // tool 引数
}

type toolCallResult struct {
	Content []toolContent `json:"content"`           // 応答本文
	IsError bool          `json:"isError,omitempty"` // エラー扱いかどうか
}

type toolContent struct {
	Type string `json:"type"` // コンテンツ種別
	Text string `json:"text"` // テキスト本文
}

func okResponse(id json.RawMessage, result any) *rpcResponse {
	return &rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func errorResponse(id json.RawMessage, code int, message string, data any) *rpcResponse {
	return &rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message, Data: data}}
}

func writeErrorResponse(w *bufio.Writer, id json.RawMessage, code int, message string, data any) error {
	return writeResponse(w, rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message, Data: data}})
}

func writeResponse(w *bufio.Writer, resp rpcResponse) error {
	body, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	return w.Flush()
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), "Content-Length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(val))
			if err != nil {
				return nil, fmt.Errorf("invalid content length: %w", err)
			}
			contentLength = parsed
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing content length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}
