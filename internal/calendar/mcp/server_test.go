package calendarmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"testing"

	gcalendar "google.golang.org/api/calendar/v3"
	"local-agent-tools/internal/calendar"
)

// 予定登録を模擬するインサーター
type fakeInserter struct{}

func (f *fakeInserter) InsertEvent(ctx context.Context, calendarID string, event *gcalendar.Event, sendUpdates string) (*gcalendar.Event, error) {
	return &gcalendar.Event{
		Id:       "evt-123",
		Summary:  event.Summary,
		HtmlLink: "https://example.com",
		Start:    event.Start,
		End:      event.End,
		Status:   "confirmed",
	}, nil
}

func TestServeHandlesInitializeAndToolList(t *testing.T) {
	input := bytes.NewBufferString(
		frame(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]any{
				"protocolVersion": "2024-11-05",
			},
		}) +
			frame(map[string]any{
				"jsonrpc": "2.0",
				"method":  "notifications/initialized",
			}) +
			frame(map[string]any{
				"jsonrpc": "2.0",
				"id":      2,
				"method":  "resources/list",
			}) +
			frame(map[string]any{
				"jsonrpc": "2.0",
				"id":      3,
				"method":  "prompts/list",
			}) +
			frame(map[string]any{
				"jsonrpc": "2.0",
				"id":      4,
				"method":  "tools/list",
			}),
	)
	var output bytes.Buffer

	svc := calendar.NewService(&fakeInserter{}, "primary")
	srv := New(svc, log.New(io.Discard, "", 0))
	if err := srv.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("serve failed: %v", err)
	}

	responses := readAllFrames(t, output.String())
	if len(responses) != 4 {
		t.Fatalf("expected 4 responses, got %d", len(responses))
	}

	var initResp rpcResponse
	mustUnmarshal(t, []byte(responses[0]), &initResp)
	if initResp.Error != nil {
		t.Fatalf("unexpected error: %+v", initResp.Error)
	}
	var initResult initializeResult
	mustUnmarshal(t, mustMarshal(t, initResp.Result), &initResult)
	if initResult.ProtocolVersion != protocolVersion {
		t.Fatalf("unexpected protocol version: %s", initResult.ProtocolVersion)
	}

	var resourcesResp rpcResponse
	mustUnmarshal(t, []byte(responses[1]), &resourcesResp)
	if resourcesResp.Error != nil {
		t.Fatalf("unexpected resources/list error: %+v", resourcesResp.Error)
	}
	var resourcesResult resourcesListResult
	mustUnmarshal(t, mustMarshal(t, resourcesResp.Result), &resourcesResult)
	if len(resourcesResult.Resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resourcesResult.Resources))
	}

	var promptsResp rpcResponse
	mustUnmarshal(t, []byte(responses[2]), &promptsResp)
	if promptsResp.Error != nil {
		t.Fatalf("unexpected prompts/list error: %+v", promptsResp.Error)
	}
	var promptsResult promptsListResult
	mustUnmarshal(t, mustMarshal(t, promptsResp.Result), &promptsResult)
	if len(promptsResult.Prompts) != 0 {
		t.Fatalf("expected 0 prompts, got %d", len(promptsResult.Prompts))
	}

	var listResp rpcResponse
	mustUnmarshal(t, []byte(responses[3]), &listResp)
	if listResp.Error != nil {
		t.Fatalf("unexpected error: %+v", listResp.Error)
	}
	var listResult toolsListResult
	mustUnmarshal(t, mustMarshal(t, listResp.Result), &listResult)
	if len(listResult.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(listResult.Tools))
	}
	if listResult.Tools[0].Name != "create_calendar_event" {
		t.Fatalf("unexpected tool: %s", listResult.Tools[0].Name)
	}
}

func TestServeCreatesCalendarEvent(t *testing.T) {
	input := bytes.NewBufferString(
		frame(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
		}) +
			frame(map[string]any{
				"jsonrpc": "2.0",
				"id":      2,
				"method":  "tools/call",
				"params": map[string]any{
					"name": "create_calendar_event",
					"arguments": map[string]any{
						"summary": "営業定例",
						"start": map[string]any{
							"dateTime": "2026-07-01T13:00:00+09:00",
						},
						"end": map[string]any{
							"dateTime": "2026-07-01T14:00:00+09:00",
						},
					},
				},
			}),
	)
	var output bytes.Buffer

	svc := calendar.NewService(&fakeInserter{}, "primary")
	srv := New(svc, log.New(io.Discard, "", 0))
	if err := srv.Serve(context.Background(), input, &output); err != nil {
		t.Fatalf("serve failed: %v", err)
	}

	responses := readAllFrames(t, output.String())
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}

	var callResp rpcResponse
	mustUnmarshal(t, []byte(responses[1]), &callResp)
	if callResp.Error != nil {
		t.Fatalf("unexpected error: %+v", callResp.Error)
	}
	var result toolCallResult
	mustUnmarshal(t, mustMarshal(t, callResp.Result), &result)
	if result.IsError {
		t.Fatalf("unexpected tool error: %+v", result)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected one content item")
	}
	if !strings.Contains(result.Content[0].Text, "evt-123") {
		t.Fatalf("missing event payload: %s", result.Content[0].Text)
	}
}

func frame(v any) string {
	b, _ := json.Marshal(v)
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(b), b)
}

func readAllFrames(t *testing.T, s string) []string {
	t.Helper()
	var out []string
	for len(s) > 0 {
		idx := strings.Index(s, "\r\n\r\n")
		if idx < 0 {
			t.Fatalf("missing frame separator in %q", s)
		}
		headers := s[:idx]
		if !strings.HasPrefix(headers, "Content-Length: ") {
			t.Fatalf("missing content length in %q", headers)
		}
		lengthStr := strings.TrimSpace(strings.TrimPrefix(headers, "Content-Length: "))
		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			t.Fatalf("bad content length %q: %v", lengthStr, err)
		}
		start := idx + 4
		end := start + length
		if end > len(s) {
			t.Fatalf("frame shorter than expected")
		}
		out = append(out, s[start:end])
		s = s[end:]
	}
	return out
}

func mustUnmarshal(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return b
}
