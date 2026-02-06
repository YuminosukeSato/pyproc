# pyproc

同一ホスト/同一Pod内で Go から Python を UDS 経由で低遅延 IPC するライブラリ。
v1.0 = 機能追加ではなく「企業が採用判断できる条件の充足」（API/プロトコル固定、運用・観測・セキュリティ・互換性の明文化と自動化）。

## スコープ

- やること: Go→Python の UDS IPC、K8s/コンテナ配布の完成度
- やらないこと: クロスホスト通信、任意コード実行（trusted code前提）、GPU分散

## コマンド

```bash
# セットアップ
go mod tidy && cd worker/python && uv sync --all-extras --dev && cd ../..

# Go テスト（race detector 有効）
go test -v -race ./...

# Python テスト
cd worker/python && uv run pytest -v

# Go lint
golangci-lint run ./...

# Python lint + format
cd worker/python && uv run ruff check . && uv run ruff format --check .

# ベンチマーク
make bench-quick
```

## コード規約

- pip 禁止。パッケージ管理は uv のみ
- Go エラーは `fmt.Errorf("context: %w", err)` でラップ
- Python は全関数に型ヒント必須、Google 形式 docstring
- Export 型/関数には doc comment 必須（Go）
- チャネル操作は `select + context.Done()` でキャンセル可能に

## SemVer方針

- Public API: Go `pkg/pyproc` exported symbols, Python `expose`/`run_worker`, wire protocol, config schema
- 0.y.z 期間: 破壊的変更は MINOR(y) を上げる
- v1.0.0 = Public API確定、互換性テスト完備

## セキュリティ

docs/security.md, internal/protocol/, .claude/rules/security.md の変更時は必ずユーザーに確認を求める。

## v1.0ロードマップ

.ssd/ に v1.0 リリース戦略がある。openspec/ で変更提案を管理する。

## 注意

- 日本語で対応する
- Go v0.4.0 と worker 0.1.0 のバージョン乖離を意識する
