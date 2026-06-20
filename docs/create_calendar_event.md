# create_calendar_event

`create_calendar_event` は、設定済みの Google Calendar に予定を作成するためのツールです。

MCP サーバーのツールとして提供されます。

## 前提

- 登録先カレンダーは `GOOGLE_CALENDAR_ID` で固定されます。
- Google Calendar API の認証には service account を使います。
- 対象カレンダーは service account のメールアドレスへ事前に共有してください。
- `calendarId` を入力する場合は、`GOOGLE_CALENDAR_ID` と一致している必要があります。
- `sendUpdates` を省略した場合は `none` として扱われます。

## 入力項目

| 項目 | 必須 | 説明 |
| --- | --- | --- |
| `summary` | 必須 | 予定の件名。1 文字以上 200 文字以内です。 |
| `description` | 任意 | 予定の説明です。 |
| `location` | 任意 | 予定の場所です。 |
| `calendarId` | 任意 | 指定する場合は設定済みのカレンダー ID と一致させます。 |
| `start` | 必須 | 開始日時または開始日です。 |
| `end` | 必須 | 終了日時または終了日です。 |
| `attendees` | 任意 | 参加者一覧です。 |
| `sendUpdates` | 任意 | `all`, `externalOnly`, `none` のいずれかです。 |

`start` と `end` は、時刻指定イベントでは `dateTime`、終日イベントでは `date` を使います。

## MCP

MCP クライアントからは `create_calendar_event` を呼び出します。

時刻指定イベントの引数例:

```json
{
  "summary": "営業定例",
  "description": "週次の営業進捗確認",
  "location": "オンライン",
  "start": {
    "dateTime": "2026-07-01T13:00:00+09:00",
    "timeZone": "Asia/Tokyo"
  },
  "end": {
    "dateTime": "2026-07-01T14:00:00+09:00",
    "timeZone": "Asia/Tokyo"
  },
  "attendees": [
    {
      "email": "tanaka@example.com",
      "displayName": "田中"
    }
  ],
  "sendUpdates": "all"
}
```

終日イベントの引数例:

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

レスポンスは MCP の text content として JSON 文字列で返ります。

## 主なバリデーション

- `summary` は 1 文字以上 200 文字以内です。
- 時刻指定イベントでは `end.dateTime` は `start.dateTime` より後である必要があります。
- 終日イベントでは `end.date` は `start.date` より後である必要があります。
- `attendees.email` はメールアドレス形式です。
- `sendUpdates` は `all`, `externalOnly`, `none` のいずれかです。

## エラー形式

エラー時はツール実行結果の `isError` が `true` になり、次のような JSON 文字列が text content として返ります。

```json
{
  "error": {
    "code": "invalid_request",
    "message": "end.dateTime must be after start.dateTime"
  }
}
```

主なエラーコード:

- `invalid_request`
- `google_permission_denied`
- `calendar_not_found`
- `rate_limited`
- `google_api_error`
- `internal_error`
