package main

import (
	"context"
	"io"
	"log"
	"os"

	"local-agent-tools/internal/calendar"
	calendarmcp "local-agent-tools/internal/calendar/mcp"
)

// mcpサーバを起動する
func main() {
	logger, closeLog := newLogger()
	defer closeLog()

	logger.Printf("starting mcpserver pid=%d", os.Getpid())
	if cwd, err := os.Getwd(); err == nil {
		logger.Printf("working directory=%s", cwd)
	} else {
		logger.Printf("get working directory failed: %v", err)
	}
	logger.Printf("env GOOGLE_CALENDAR_ID set=%t", os.Getenv("GOOGLE_CALENDAR_ID") != "")
	logger.Printf("env GOOGLE_SERVICE_ACCOUNT_JSON set=%t", os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON") != "")
	logger.Printf("env GOOGLE_SERVICE_ACCOUNT_FILE=%q", os.Getenv("GOOGLE_SERVICE_ACCOUNT_FILE"))

	logger.Print("initializing google client")
	googleClient, err := initGoogleClient(context.Background())
	if err != nil {
		logger.Printf("init google client failed: %v", err)
		os.Exit(1)
	}
	logger.Print("google client initialized")

	defaultCalendarID := os.Getenv("GOOGLE_CALENDAR_ID")
	if defaultCalendarID == "" {
		logger.Print("GOOGLE_CALENDAR_ID is required")
		os.Exit(1)
	}

	svc := calendar.NewService(googleClient, defaultCalendarID)
	server := calendarmcp.New(svc, logger)

	logger.Print("starting stdio MCP serve loop")
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		logger.Printf("mcp server exited: %v", err)
		os.Exit(1)
	}
	logger.Print("mcp server stopped")
}

func newLogger() (*log.Logger, func()) {
	logPath := os.Getenv("MCP_SERVER_LOG_FILE")
	if logPath == "" {
		logPath = "mcpserver.log"
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		logger := log.New(os.Stderr, "", log.LstdFlags)
		logger.Printf("open log file %q failed: %v", logPath, err)
		return logger, func() {}
	}

	logger := log.New(io.MultiWriter(os.Stderr, f), "", log.LstdFlags)
	logger.Printf("logging to %s", logPath)
	return logger, func() {
		if err := f.Close(); err != nil {
			log.New(os.Stderr, "", log.LstdFlags).Printf("close log file %q failed: %v", logPath, err)
		}
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
