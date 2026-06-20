package calendar

import (
	"context"
	"testing"

	gcalendar "google.golang.org/api/calendar/v3"
)

// 予定登録を模擬する
type fakeInserter struct {
	event *gcalendar.Event // 受け取った予定
}

// 予定登録を模擬する
func (f *fakeInserter) InsertEvent(ctx context.Context, calendarID string, event *gcalendar.Event, sendUpdates string) (*gcalendar.Event, error) {
	f.event = event
	return &gcalendar.Event{
		Id:       "evt-123",
		Summary:  event.Summary,
		HtmlLink: "https://example.com",
		Start:    event.Start,
		End:      event.End,
		Status:   "confirmed",
	}, nil
}

func TestCreateCalendarEvent(t *testing.T) {
	ins := &fakeInserter{}
	svc := NewService(ins, "primary")
	resp, err := svc.CreateCalendarEvent(context.Background(), CreateCalendarEventRequest{
		Summary: "Meeting",
		Start:   EventDateTime{DateTime: "2026-07-01T10:00:00+09:00"},
		End:     EventDateTime{DateTime: "2026-07-01T11:00:00+09:00"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.EventID != "evt-123" {
		t.Fatalf("unexpected event id: %s", resp.EventID)
	}
	if resp.CalendarID != "primary" {
		t.Fatalf("unexpected calendar id: %s", resp.CalendarID)
	}
	if ins.event == nil || ins.event.Start == nil || ins.event.Start.DateTime != "2026-07-01T10:00:00+09:00" {
		t.Fatalf("unexpected inserted event: %+v", ins.event)
	}
}

func TestCreateCalendarEventAllDay(t *testing.T) {
	ins := &fakeInserter{}
	svc := NewService(ins, "primary")
	resp, err := svc.CreateCalendarEvent(context.Background(), CreateCalendarEventRequest{
		Summary: "All day",
		Start:   EventDateTime{Date: "2026-07-01"},
		End:     EventDateTime{Date: "2026-07-02"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Start.Date != "2026-07-01" {
		t.Fatalf("unexpected start date: %s", resp.Start.Date)
	}
	if resp.End.Date != "2026-07-02" {
		t.Fatalf("unexpected end date: %s", resp.End.Date)
	}
	if ins.event == nil || ins.event.Start == nil || ins.event.Start.Date != "2026-07-01" {
		t.Fatalf("unexpected inserted all-day event: %+v", ins.event)
	}
}

func TestValidateCreateRequestRejectsBadOrder(t *testing.T) {
	svc := NewService(&fakeInserter{}, "primary")
	_, err := svc.CreateCalendarEvent(context.Background(), CreateCalendarEventRequest{
		Summary: "Meeting",
		Start:   EventDateTime{DateTime: "2026-07-01T11:00:00+09:00"},
		End:     EventDateTime{DateTime: "2026-07-01T10:00:00+09:00"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateCalendarEventRejectsMixedModes(t *testing.T) {
	svc := NewService(&fakeInserter{}, "primary")
	_, err := svc.CreateCalendarEvent(context.Background(), CreateCalendarEventRequest{
		Summary: "Meeting",
		Start:   EventDateTime{Date: "2026-07-01"},
		End:     EventDateTime{DateTime: "2026-07-02T00:00:00+09:00"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
