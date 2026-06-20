package main

import (
	"context"
	"log"
	"os"

	"local-agent-tools/internal/calendar"
	calendarmcp "local-agent-tools/internal/calendar/mcp"
)

// mcpサーバを起動する
func main() {
	googleClient, err := initGoogleClient(context.Background())
	if err != nil {
		log.Fatalf("init google client: %v", err)
	}

	defaultCalendarID := os.Getenv("GOOGLE_CALENDAR_ID")
	if defaultCalendarID == "" {
		log.Fatal("GOOGLE_CALENDAR_ID is required")
	}

	svc := calendar.NewService(googleClient, defaultCalendarID)
	server := calendarmcp.New(svc, log.New(os.Stderr, "", log.LstdFlags))

	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		log.Fatalf("mcp server exited: %v", err)
	}
}

// Google Calendar 用の認証済みクライアントを初期化する
func initGoogleClient(ctx context.Context) (*calendar.GoogleClient, error) {
	serviceAccountJSON := os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON")
	serviceAccountFile := os.Getenv("GOOGLE_SERVICE_ACCOUNT_FILE")

	httpClient, err := calendar.NewAuthorizedHTTPClient(ctx, serviceAccountJSON, serviceAccountFile)
	if err != nil {
		return nil, err
	}

	return calendar.NewGoogleClient(ctx, httpClient)
}
