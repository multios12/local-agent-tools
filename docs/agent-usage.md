# AI Agent Usage Guide

この MCP サーバーは、Google Calendar の固定カレンダーに予定を作成するための単機能ツールを提供します。
AI エージェントからは `create_calendar_event` を使います。終日イベントは `start.date` と `end.date` を使い、時刻指定イベントは `start.dateTime` と `end.dateTime` を使います。

## 前提

- 認証は service account を使います
- 登録先カレンダーは固定です
- `calendarId` を送る場合は、設定済みの値と一致している必要があります
- `sendUpdates` は `all`, `externalOnly`, `none` のいずれかです
- `sendUpdates` を省略した場合は `none` として扱われます
- 終日イベントでは `end.date` は `start.date` より後である必要があります

## 参照先

- MCP server: `go run ./cmd/mcpserver`
- Tool: `create_calendar_event`

## 最小リクエスト

```json
{
  "summary": "営業定例",
  "start": {
    "dateTime": "2026-07-01T13:00:00+09:00",
    "timeZone": "Asia/Tokyo"
  },
  "end": {
    "dateTime": "2026-07-01T14:00:00+09:00",
    "timeZone": "Asia/Tokyo"
  }
}
```

終日イベント例:

```json
{
  "summary": "休暇",
  "start": {
    "date": "2026-07-01"
  },
  "end": {
    "date": "2026-07-02"
  }
}
```

## 推奨フロー

1. MCP クライアントに `cmd/mcpserver` を登録する
2. `tools/list` で `create_calendar_event` が公開されていることを確認する
3. `create_calendar_event` で予定を作成する
4. レスポンスの `eventId` と `htmlLink` を使って後続処理を行う

## エラー処理

- `invalid_request` は入力の修正が必要です
- `google_permission_denied` はカレンダー共有や権限を確認してください
- `calendar_not_found` は対象カレンダー ID を確認してください
- `rate_limited` はリトライ前に待機してください
- `google_api_error` は Google 側の一時障害を疑ってください

## エージェント向けの注意

- 同じ予定を重複作成しないように、再試行は慎重に行ってください
- `summary` は 200 文字以内です
- `end.dateTime` は `start.dateTime` より後である必要があります
- 日時は RFC3339 形式で指定してください

詳しい入力仕様は [create_calendar_event](create_calendar_event.md) を参照してください。
