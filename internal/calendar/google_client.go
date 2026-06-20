package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2/google"
	gcalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// 予定登録を実行するインターフェース
type EventInserter interface {
	// 予定を指定したカレンダーへ登録する
	InsertEvent(ctx context.Context, calendarID string, event *gcalendar.Event, sendUpdates string) (*gcalendar.Event, error)
}

// Google Calendar API クライアント
type GoogleClient struct {
	service *gcalendar.Service // Google Calendar API のサービスクライアント
}

// Google Calendar API のサービスクライアントを生成する
func NewGoogleClient(ctx context.Context, httpClient *http.Client) (*GoogleClient, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	svc, err := gcalendar.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create google calendar service: %w", err)
	}

	return &GoogleClient{service: svc}, nil
}

// service account の認証情報から HTTP クライアントを生成する
func NewAuthorizedHTTPClient(ctx context.Context, serviceAccountJSON, serviceAccountFile string) (*http.Client, error) {
	credJSON, err := loadServiceAccountJSON(serviceAccountJSON, serviceAccountFile)
	if err != nil {
		return nil, err
	}

	cfg, err := google.JWTConfigFromJSON(credJSON, gcalendar.CalendarEventsScope)
	if err != nil {
		return nil, fmt.Errorf("parse service account credentials: %w", err)
	}

	client := cfg.Client(ctx)
	return client, nil
}

// service account の JSON を文字列またはファイルから読み込む
func loadServiceAccountJSON(inlineJSON, filePath string) ([]byte, error) {
	if strings.TrimSpace(inlineJSON) != "" {
		if !json.Valid([]byte(inlineJSON)) {
			return nil, fmt.Errorf("GOOGLE_SERVICE_ACCOUNT_JSON must contain valid JSON")
		}
		return []byte(inlineJSON), nil
	}

	if strings.TrimSpace(filePath) == "" {
		return nil, fmt.Errorf("google service account configuration is incomplete")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read service account file: %w", err)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("service account file must contain valid JSON")
	}
	return data, nil
}

// 予定を登録する
func (c *GoogleClient) InsertEvent(ctx context.Context, calendarID string, event *gcalendar.Event, sendUpdates string) (*gcalendar.Event, error) {
	call := c.service.Events.Insert(calendarID, event).Context(ctx)
	if sendUpdates != "" {
		call = call.SendUpdates(sendUpdates)
	}

	created, err := call.Do()
	if err != nil {
		return nil, classifyGoogleError(err)
	}
	return created, nil
}

func classifyGoogleError(err error) error {
	if gerr, ok := err.(*googleapi.Error); ok {
		switch gerr.Code {
		case http.StatusForbidden:
			return AppError{
				HTTPStatus: http.StatusForbidden,
				Code:       "google_permission_denied",
				Message:    "Google Calendar access was denied",
				Details:    map[string]any{"googleStatus": gerr.Code},
			}
		case http.StatusNotFound:
			return AppError{
				HTTPStatus: http.StatusNotFound,
				Code:       "calendar_not_found",
				Message:    "The target calendar was not found",
				Details:    map[string]any{"googleStatus": gerr.Code},
			}
		case http.StatusTooManyRequests:
			return AppError{
				HTTPStatus: http.StatusTooManyRequests,
				Code:       "rate_limited",
				Message:    "Google Calendar rate limit exceeded",
				Details:    map[string]any{"googleStatus": gerr.Code},
			}
		default:
			if gerr.Code >= 500 {
				return AppError{
					HTTPStatus: http.StatusBadGateway,
					Code:       "google_api_error",
					Message:    "Google Calendar API returned an error",
					Details:    map[string]any{"googleStatus": gerr.Code},
				}
			}
		}
	}

	return err
}
