# local-agent-tools

`local-agent-tools` は、AI エージェントからローカルまたは周辺サービスを扱うための複数のツールを提供します。

## 提供ツール

| ツール                | 提供形態   | 説明                                 |
| --------------------- | ---------- | ------------------------------------ |
| create_calendar_event | MCP server | Google Calendar に予定を作成します。 |

### mcpserver

`cmd/mcpserver` は stdio ベースの MCP サーバーです。

公開ツール:

- `create_calendar_event`: 固定の Google Calendar に予定を作成

起動例:

```bash
go build -o ./bin/mcpserver ./cmd/mcpserver
./bin/mcpserver
```

## 環境変数

| 変数 | 必須 | 説明 |
| --- | --- | --- |
| `GOOGLE_CALENDAR_ID` | 必須 | 予定を書き込む Google Calendar ID。 |
| `GOOGLE_SERVICE_ACCOUNT_JSON` | どちらか必須 | service account の JSON を文字列で指定します。 |
| `GOOGLE_SERVICE_ACCOUNT_FILE` | どちらか必須 | service account JSON ファイルのパスを指定します。 |

`GOOGLE_SERVICE_ACCOUNT_JSON` と `GOOGLE_SERVICE_ACCOUNT_FILE` はどちらか一方だけ設定してください。

Google Calendar 側では、対象カレンダーを service account のメールアドレスに共有し、予定を追加できる権限を付与しておく必要があります。

## MCP サーバーとして登録する

MCP クライアントには `cmd/mcpserver` を stdio サーバーとして登録します。

以下は `mcpServers` 形式の設定を使う MCP クライアント向けの例です。設定ファイルの場所や項目名はクライアントごとに異なります。

例:

```json
{
  "mcpServers": {
    "local-agent-tools": {
      "command": "/workspaces/local-agent-tools/bin/mcpserver",
      "cwd": "/workspaces/local-agent-tools",
      "env": {
        "GOOGLE_CALENDAR_ID": "primary", # 使用するGoogleカレンダID
        "GOOGLE_SERVICE_ACCOUNT_FILE": "/absolute/path/to/service-account.json" # サービスアカウントファイルのパス
      }
    }
  }
}
```

`GOOGLE_SERVICE_ACCOUNT_JSON` を使う場合は、`GOOGLE_SERVICE_ACCOUNT_FILE` の代わりに JSON 文字列を渡してください。

### VS Code の Codex 拡張機能で使う場合

Codex 拡張機能では `config.toml` に MCP サーバーを登録します。

例:

```toml
[mcp_servers."local-agent-tools"]
command = "/workspaces/local-agent-tools/bin/mcpserver"
cwd = "/workspaces/local-agent-tools"

[mcp_servers."local-agent-tools".env]
GOOGLE_CALENDAR_ID = "primary"
GOOGLE_SERVICE_ACCOUNT_FILE = "/absolute/path/to/service-account.json"
```

`GOOGLE_SERVICE_ACCOUNT_JSON` を使う場合は、`GOOGLE_SERVICE_ACCOUNT_FILE` の代わりに JSON 文字列を設定してください。

## ドキュメント

- `create_calendar_event` の使い方: [docs/create_calendar_event.md](docs/create_calendar_event.md)
- AI エージェント向け利用ガイド: [docs/agent-usage.md](docs/agent-usage.md)

## 開発

テスト:

```bash
go test ./...
```

VS Code デバッグ:

1. `.env.debug.example` を `.env.debug` にコピーして値を埋める
2. VS Code の `Run and Debug` から `Debug MCP Server` を選ぶ

`.env.debug` では `GOOGLE_SERVICE_ACCOUNT_JSON` か `GOOGLE_SERVICE_ACCOUNT_FILE` のどちらか一方だけを設定してください。
