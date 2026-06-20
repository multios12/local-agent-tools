package calendar

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	gcalendar "google.golang.org/api/calendar/v3"
)

const dateLayout = "2006-01-02"

// 予定作成の業務ロジック
type Service struct {
	client          EventInserter // 登録先クライアント
	defaultCalendar string        // 既定の登録先カレンダー
}

// API エラー
type AppError struct {
	HTTPStatus int            // HTTP ステータスコード
	Code       string         // エラーコード
	Message    string         // メッセージ
	Details    map[string]any // 補足情報
}

// エラー文字列を返す
func (e AppError) Error() string {
	return e.Message
}

// 予定作成サービスを生成する
func NewService(client EventInserter, defaultCalendar string) *Service {
	if strings.TrimSpace(defaultCalendar) == "" {
		defaultCalendar = "primary"
	}
	return &Service{client: client, defaultCalendar: defaultCalendar}
}

// 予定を作成する
func (s *Service) CreateCalendarEvent(ctx context.Context, req CreateCalendarEventRequest) (CalendarEventResponse, error) {
	if err := validateCreateRequest(req); err != nil {
		return CalendarEventResponse{}, err
	}

	calendarID := strings.TrimSpace(req.CalendarID)
	if calendarID == "" {
		calendarID = s.defaultCalendar
	}
	if calendarID != s.defaultCalendar {
		return CalendarEventResponse{}, AppError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "invalid_request",
			Message:    "calendarId must match the configured target calendar",
			Details:    map[string]any{"field": "calendarId"},
		}
	}

	startMode, startValue, err := parseEventTime(req.Start)
	if err != nil {
		return CalendarEventResponse{}, AppError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "invalid_request",
			Message:    err.Error(),
			Details:    map[string]any{"field": "start"},
		}
	}
	endMode, endValue, err := parseEventTime(req.End)
	if err != nil {
		return CalendarEventResponse{}, AppError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "invalid_request",
			Message:    err.Error(),
			Details:    map[string]any{"field": "end"},
		}
	}

	if startMode != endMode {
		return CalendarEventResponse{}, AppError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "invalid_request",
			Message:    "start and end must use the same time format",
			Details:    map[string]any{"field": "start", "hint": "use either date or dateTime for both start and end"},
		}
	}

	event := &gcalendar.Event{
		Summary:     req.Summary,
		Description: req.Description,
		Location:    req.Location,
	}

	switch startMode {
	case eventTimeModeAllDay:
		if !endValue.After(startValue) {
			return CalendarEventResponse{}, AppError{
				HTTPStatus: http.StatusBadRequest,
				Code:       "invalid_request",
				Message:    "end.date must be after start.date",
				Details:    map[string]any{"field": "end.date"},
			}
		}
		event.Start = &gcalendar.EventDateTime{Date: req.Start.Date}
		event.End = &gcalendar.EventDateTime{Date: req.End.Date}
	case eventTimeModeTimed:
		if !endValue.After(startValue) {
			return CalendarEventResponse{}, AppError{
				HTTPStatus: http.StatusBadRequest,
				Code:       "invalid_request",
				Message:    "end.dateTime must be after start.dateTime",
				Details:    map[string]any{"field": "end.dateTime"},
			}
		}
		event.Start = &gcalendar.EventDateTime{
			DateTime: req.Start.DateTime,
			TimeZone: req.Start.TimeZone,
		}
		event.End = &gcalendar.EventDateTime{
			DateTime: req.End.DateTime,
			TimeZone: req.End.TimeZone,
		}
	default:
		return CalendarEventResponse{}, AppError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "invalid_request",
			Message:    "start must include either date or dateTime",
			Details:    map[string]any{"field": "start"},
		}
	}

	for _, attendee := range req.Attendees {
		event.Attendees = append(event.Attendees, &gcalendar.EventAttendee{
			Email:       attendee.Email,
			DisplayName: attendee.DisplayName,
		})
	}

	sendUpdates := strings.TrimSpace(req.SendUpdates)
	if sendUpdates == "" {
		sendUpdates = "none"
	}

	created, err := s.client.InsertEvent(ctx, calendarID, event, sendUpdates)
	if err != nil {
		return CalendarEventResponse{}, err
	}

	return mapCalendarEventResponse(created, calendarID), nil
}

// 予定作成リクエストを検証する
func validateCreateRequest(req CreateCalendarEventRequest) error {
	if strings.TrimSpace(req.Summary) == "" {
		return AppError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "invalid_request",
			Message:    "summary is required",
			Details:    map[string]any{"field": "summary"},
		}
	}
	if len(req.Summary) > 200 {
		return AppError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "invalid_request",
			Message:    "summary must be 200 characters or fewer",
			Details:    map[string]any{"field": "summary"},
		}
	}

	if _, _, err := parseEventTime(req.Start); err != nil {
		return AppError{HTTPStatus: http.StatusBadRequest, Code: "invalid_request", Message: err.Error(), Details: map[string]any{"field": "start"}}
	}
	if _, _, err := parseEventTime(req.End); err != nil {
		return AppError{HTTPStatus: http.StatusBadRequest, Code: "invalid_request", Message: err.Error(), Details: map[string]any{"field": "end"}}
	}

	for _, attendee := range req.Attendees {
		if strings.TrimSpace(attendee.Email) == "" {
			return AppError{HTTPStatus: http.StatusBadRequest, Code: "invalid_request", Message: "attendees.email is required", Details: map[string]any{"field": "attendees.email"}}
		}
		if _, err := mail.ParseAddress(attendee.Email); err != nil {
			return AppError{HTTPStatus: http.StatusBadRequest, Code: "invalid_request", Message: "attendees.email must be a valid email address", Details: map[string]any{"field": "attendees.email"}}
		}
	}

	switch strings.TrimSpace(req.SendUpdates) {
	case "", "all", "externalOnly", "none":
	default:
		return AppError{
			HTTPStatus: http.StatusBadRequest,
			Code:       "invalid_request",
			Message:    "sendUpdates must be one of all, externalOnly, none",
			Details:    map[string]any{"field": "sendUpdates"},
		}
	}

	return nil
}

type eventTimeMode int

const (
	eventTimeModeUnknown eventTimeMode = iota
	eventTimeModeAllDay
	eventTimeModeTimed
)

// 予定入力の時間表現を解釈する
func parseEventTime(v EventDateTime) (eventTimeMode, time.Time, error) {
	date := strings.TrimSpace(v.Date)
	dt := strings.TrimSpace(v.DateTime)

	switch {
	case date != "" && dt != "":
		return eventTimeModeUnknown, time.Time{}, fmt.Errorf("date and dateTime cannot both be set")
	case date != "":
		t, err := time.Parse(dateLayout, date)
		if err != nil {
			return eventTimeModeUnknown, time.Time{}, fmt.Errorf("date must be a valid YYYY-MM-DD value")
		}
		return eventTimeModeAllDay, t, nil
	case dt != "":
		t, err := time.Parse(time.RFC3339, dt)
		if err != nil {
			return eventTimeModeUnknown, time.Time{}, fmt.Errorf("dateTime must be a valid RFC3339 timestamp")
		}
		return eventTimeModeTimed, t, nil
	default:
		return eventTimeModeUnknown, time.Time{}, fmt.Errorf("either date or dateTime is required")
	}
}

// Google Calendar の予定レスポンスへ変換する
func mapCalendarEventResponse(event *gcalendar.Event, calendarID string) CalendarEventResponse {
	resp := CalendarEventResponse{CalendarID: calendarID}
	if event == nil {
		return resp
	}

	resp.EventID = event.Id
	resp.Summary = event.Summary
	resp.HTMLLink = event.HtmlLink
	resp.Status = event.Status
	if event.Start != nil {
		resp.Start = EventDateTime{Date: event.Start.Date, DateTime: event.Start.DateTime, TimeZone: event.Start.TimeZone}
	}
	if event.End != nil {
		resp.End = EventDateTime{Date: event.End.Date, DateTime: event.End.DateTime, TimeZone: event.End.TimeZone}
	}
	if event.Created != "" {
		if t, err := time.Parse(time.RFC3339, event.Created); err == nil {
			resp.Created = &t
		}
	}
	if event.Updated != "" {
		if t, err := time.Parse(time.RFC3339, event.Updated); err == nil {
			resp.Updated = &t
		}
	}
	return resp
}

// アプリケーションエラーかどうかを判定する
func IsAppError(err error) (AppError, bool) {
	var appErr AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return AppError{}, false
}

// HTTP ステータスコードを返す
func (e AppError) StatusCode() int {
	if e.HTTPStatus == 0 {
		return http.StatusInternalServerError
	}
	return e.HTTPStatus
}
