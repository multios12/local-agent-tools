package calendar

import "time"

// 予定作成リクエストの入力値
type CreateCalendarEventRequest struct {
	Summary     string        `json:"summary"`               // 件名
	Description string        `json:"description,omitempty"` // 説明
	Location    string        `json:"location,omitempty"`    // 場所
	CalendarID  string        `json:"calendarId,omitempty"`  // 登録先カレンダー ID
	Start       EventDateTime `json:"start"`                 // 開始日時または開始日
	End         EventDateTime `json:"end"`                   // 終了日時または終了日
	Attendees   []Attendee    `json:"attendees,omitempty"`   // 参加者一覧
	SendUpdates string        `json:"sendUpdates,omitempty"` // 招待通知の送信方法
}

// 予定の日付と時刻または日付のみ
type EventDateTime struct {
	Date     string `json:"date,omitempty"`     // 日付
	DateTime string `json:"dateTime,omitempty"` // 日時文字列
	TimeZone string `json:"timeZone,omitempty"`  // タイムゾーン
}

// 参加者の情報
type Attendee struct {
	Email       string `json:"email"`                 // メールアドレス
	DisplayName string `json:"displayName,omitempty"` // 表示名
}

// 予定作成結果
type CalendarEventResponse struct {
	EventID    string        `json:"eventId"`            // 予定 ID
	CalendarID string        `json:"calendarId"`         // 登録先カレンダー ID
	Summary    string        `json:"summary"`            // 件名
	HTMLLink   string        `json:"htmlLink,omitempty"` // Google Calendar のリンク
	Start      EventDateTime `json:"start"`              // 開始日時または開始日
	End        EventDateTime `json:"end"`                // 終了日時または終了日
	Status     string        `json:"status,omitempty"`   // 状態
	Created    *time.Time    `json:"created,omitempty"`  // 作成日時
	Updated    *time.Time    `json:"updated,omitempty"`  // 更新日時
}

// エラーレスポンス全体
type ErrorResponse struct {
	Error APIError `json:"error"` // エラー本体
}

// エラーの詳細
type APIError struct {
	Code    string         `json:"code"`              // エラーコード
	Message string         `json:"message"`           // メッセージ
	Details map[string]any `json:"details,omitempty"` // 補足情報
}
