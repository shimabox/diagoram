# Phase 1: プロジェクト基盤

**目的**: 以降のフェーズが TDD で迷いなく進められる土台を作る。
**完了時の姿**: `go run ./cmd/diagoram -h` がヘルプを表示し、golden テストヘルパと CI が動いている。

## 前提
- なし（最初のフェーズ）
- 全体方針は [00-overview.md](00-overview.md) を必ず先に読むこと

## タスク

### 1-1. リポジトリ初期化
- [x] `git init`（デフォルトブランチ `main`）
- [x] `go mod init github.com/shimabox/diagoram`
- [x] `go` ディレクティブは `1.21` 程度に抑える（新しすぎる言語機能に依存しない）
- [x] `.gitignore`（バイナリ、`dist/` 等）
- [x] `LICENSE`（MIT）

### 1-2. CLI 骨格（TDD）
- [x] テスト先行: `internal/cli` に対する table test を書く
  - 引数なし → エラー（usage を案内するメッセージ）
  - 存在しないディレクトリ → エラー（「ディレクトリが存在しない」ことが分かるメッセージ）
  - `-h` / `--help` → usage 文字列を返し正常終了
  - `-v` / `--version` → バージョン文字列
- [x] `internal/cli`: 標準 `flag` パッケージで `Options` 構造体にパース。**この時点のフラグは `-h`/`-v` のみ**（残りは各フェーズで追加）
- [x] `cmd/diagoram/main.go`: `cli.Run(args, stdout, stderr) int` を呼んで exit code を返すだけの薄い main
  - `Run` が io.Writer を受ける設計にすることで、テストから出力を検証できるようにする
- [x] バージョンは `var version = "dev"` とし、`-ldflags` で注入可能にする

### 1-3. golden テストヘルパ（TDD の基盤）
- [x] `internal/testutil/golden.go`:
  - `testutil.Golden(t, goldenPath, actual string)` — golden ファイルと比較。差分があれば分かりやすく fail
  - パッケージ変数 `var update = flag.Bool("update", false, ...)` により `go test ./... -update` で golden を書き換え
- [x] ヘルパ自体の動作をテストで確認する

### 1-4. CI
- [x] `.github/workflows/test.yml`: push/PR で `go test ./...`, `go vet ./...`, `gofmt -l .` のチェック（gofmt は差分があれば fail）
- [x] `Makefile`（または不要なら省略可）: `make test`, `make build` 程度。凝らない

### 1-5. フェーズ完了処理
- [x] E2E 確認: `go run ./cmd/diagoram -h` / `-v` / 引数なし の 3 パターンを実際に叩き、出力を目視確認
- [x] コミット（例: `feat: project foundation with CLI skeleton and golden test helper`）

## 受け入れ基準
- `go test ./...` / `go vet ./...` 全緑、`gofmt -l .` 出力なし
- 外部依存ゼロ（`go.mod` に require がない）
- main.go が 20 行以内程度に薄い

## スコープ外
- 解析処理・図の出力（Phase 2, 3）
- Dockerfile（Phase 7）
