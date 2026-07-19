# diagoram 開発計画 — 全体像

> **このファイル群だけで、いつでも・どのモデルでも開発を再開できること** を目的とする。
> 各フェーズの詳細は `01-*.md` 〜 `07-*.md` を参照。進捗は各フェーズファイルのチェックボックスで管理する。

## 1. diagoram とは

Go のソースコードを解析し、**struct/interfaceや名前付き型とその依存関係**、**パッケージ間の依存関係**を Mermaid / PlantUML テキストとして図示する CLI ツール。
[smeghead/php-class-diagram](https://github.com/smeghead/php-class-diagram) の思想（ソースから図を継続生成して設計改善に活かす）を Go 向けに再構築する。名前は diagram + Go のもじり。

### なぜ作るか
複雑な Go プロジェクトの改善を練るために、依存関係を可視化したい。

## 2. 確定済みの設計判断（変更には相応の理由が必要）

| 項目 | 決定 | 理由 |
|---|---|---|
| 実装言語 | Go（開発は Go 1.24 系、`go.mod` の要求は低めに保つ） | 単一バイナリ配布、標準ライブラリに AST がある |
| モジュールパス | `github.com/shimabox/diagoram` | ユーザー確認済み |
| 解析方式 | **純構文解析**（`go/parser` + `go/ast` のみ。`go/types`・`go/packages` は使わない） | 対象コードのビルド不要・依存取得不要。壊れかけ/古い Go のコードでも動く。Docker も軽量 |
| 外部依存 | **原則ゼロ**（標準ライブラリのみ。テスト含む） | 開発者フレンドリー・保守性・ビルドの速さ |
| 図の種類 | 型と依存関係の図（struct/interface） + パッケージ依存図（循環依存の警告つき） | php-class-diagram の核を踏襲 |
| 出力形式 | Mermaid を先に完成 → PlantUML を後続フェーズで追加 | Mermaid は GitHub でネイティブ表示され開発者フレンドリー |
| interface 実装検出 | メソッドシグネチャ照合のヒューリスティック（解析対象内の型同士のみ） | 純構文解析の制約下での現実解 |
| 改行コード | 出力・golden ファイルとも `\n` 固定 | golden テストの安定 |
| CLI パース | 標準 `flag` パッケージ | 依存ゼロ方針 |

## 3. アーキテクチャ

php-class-diagram の「言語解析層 / 図モデル層」2 層分離を踏襲し、レンダラを差し替え可能にする。

```
入力(dir) → [gocode] 構文解析 → 言語モデル → [diagram] 中間表現(IR)構築 → [render] テキスト生成 → stdout
```

```
diagoram/
├── cmd/diagoram/main.go        # エントリポイント（薄く保つ）
├── internal/
│   ├── cli/                    # オプション定義・パース・実行オーケストレーション
│   ├── gocode/                 # go/parser でソース → 言語モデル（Package, Struct, Interface, ...）
│   ├── diagram/                # 中間表現: パッケージツリー, Entry, Relation(Edge), フィルタ
│   └── render/
│       ├── mermaid/            # Mermaid レンダラ
│       └── plantuml/           # PlantUML レンダラ（Phase 6）
├── testdata/fixtures/          # 題材 Go コード + 期待出力(golden)
├── Dockerfile
└── README.md
```

- **gocode（言語モデル）**: Go の言語的事実のみを表す。図の概念を持ち込まない。
  - `Package`（ディレクトリ単位）, `Struct`（フィールド・メソッド・埋め込み）, `Interface`（メソッドセット・埋め込み）, `TypeRef`（型参照。パッケージ修飾・スライス/マップ/ポインタ情報を保持）
  - メソッドはレシーバ型で struct に紐付ける。可視性は識別子の大文字/小文字（exported / unexported）
- **diagram（IR）**: ディレクトリ階層を再帰的な **Package ツリー** にマッピング。エッジは種類別の型で表現:
  - `Dependency`（フィールド型・メソッド引数/戻り値の型参照）→ `..>`
  - `Embedding`（struct/interface の埋め込み）→ `--|>` 系
  - `Implementation`（interface 実装のヒューリスティック検出）→ `..|>`
  - `PackageDep`（パッケージ間依存。**相互依存は赤太線で警告** = php-class-diagram の看板機能）
- **render**: `Renderer` インターフェース 1 つに対し mermaid / plantuml の 2 実装。IR より先を知らない。

## 4. CLI 仕様（最終形。フェーズごとに段階実装）

```
diagoram [options] <dir>

  --class-diagram            型と依存関係を図で出力（デフォルト）
  --package-diagram          パッケージ依存図を出力
  --format=mermaid|plantuml  出力形式（デフォルト mermaid。plantuml は Phase 6）
  --include='glob'           対象ファイルパターン（複数指定可。デフォルト *.go）
  --exclude='glob'           除外パターン（複数指定可。デフォルトで *_test.go を除外）
  --hide-unexported          unexported なフィールド・メソッドを隠す
  --disable-fields           型と依存関係の図にフィールドを描かない
  --disable-methods          型と依存関係の図にメソッドを描かない
  --rel-target='A,B'         指定した型の周辺だけを抽出
  --rel-target-depth=N       抽出時に辿る依存の深さ（デフォルト 1）
  --summary                  解析対象の型一覧をプレーンテキストで出力
  -h, --help / -v, --version
```

## 5. フェーズ一覧

| フェーズ | 内容 | 完了時の姿 | 状態 |
|---|---|---|---|
| [1](01-phase1-foundation.md) | プロジェクト基盤 | CLI 骨格が動き、golden テスト基盤と CI がある | **完了** |
| [2](02-phase2-analyzer.md) | 解析器 gocode | fixture の Go コードを言語モデルに変換できる | **完了** |
| [3](03-phase3-mermaid-class-diagram.md) | IR + Mermaidで型と依存関係を図示 | **E2E で動く MVP**。`diagoram <dir>` が型と依存関係を出す | **完了 (v0.1.0)** |
| [4](04-phase4-package-diagram.md) | パッケージ依存図 | 循環依存の赤線警告つきパッケージ図が出る | **完了 (v0.2.0)** |
| [5](05-phase5-filters-and-implements.md) | フィルタ + interface 実装検出 + summary | 大規模プロジェクトで実用になる | **完了 (v0.3.0)** |
| [6](06-phase6-plantuml.md) | PlantUML レンダラ | `--format=plantuml` が動く | **完了 (v0.4.0)** |
| [7](07-phase7-docker-release.md) | Docker・README・ドッグフーディング | `docker run` で誰でも使える。README に自分自身の図 | **完了 (v1.0.0)** |

各フェーズ末尾でタグを打つ（Phase 3 = v0.1.0, 4 = v0.2.0, 5 = v0.3.0, 6 = v0.4.0, 7 = v1.0.0）。

## 6. 開発の進め方（実行プロトコル）

### TDD スタイル
1. フェーズファイルの仕様から **先に失敗するテストを書く**（fixture + 期待値、または table test）
2. テストを通す最小実装
3. リファクタリング（`gofmt` / `go vet` を常にクリーンに）

### golden テスト規約
- fixture は `testdata/fixtures/<ケース名>/` に小さな題材コード（php-class-diagram に倣い Product/Name/Price のようなミニドメイン）を置く
- 期待出力は同ディレクトリの `expected-class.mmd` 等
- テストヘルパに `-update` フラグを用意し、`go test ./... -update` で golden を再生成できるようにする
- 出力は決定的であること（map 走査順に依存しない。必ずソートする）

### オーケストレーションのルール
- **オーケストレーター（上位モデル）は実装しない**。フェーズファイルのタスク単位でサブエージェント（下位モデル）に委譲する
- 委譲プロンプトには必ず含める: 対象フェーズファイルのパス、TDD で進めること、`go test ./...` と `go vet ./...` が通ること、依存ゼロ方針
- オーケストレーターはタスク完了ごとに検収する: テスト実行結果の確認 → コードレビュー → フェーズファイルのチェックボックス更新
- フェーズ完了時に E2E で実際にコマンドを叩いて出力を目視確認し、コミットする

### タスクの Definition of Done
- [ ] テストが先にあり、実装後に `go test ./...` 全緑
- [ ] `go vet ./...` / `gofmt -l .` がクリーン
- [ ] 外部依存を増やしていない
- [ ] 公開 API（exported な型・関数）に doc コメントがある

## 7. 開発者フレンドリー原則（最優先の価値）

- **迷ったらシンプルな方**を選ぶ。抽象化は 2 つ目の実装が現れるまで導入しない（Renderer は例外。Phase 6 で 2 実装目が確定しているため）
- エラーメッセージは「何が起きたか + どうすればよいか」を含める
- `diagoram <dir>` だけで意味のある出力が出る（設定ファイル不要）
- ヘルプ（`-h`）だけで使い方が完結する
- README は例（入力コード → 出力図）を中心に書く

## 8. 参考: php-class-diagram から学んだこと

- 2 層分離（言語モデル / 図モデル）、再帰的 Package ツリー、Arrow の種類別型、行配列で出力を組み立てる方式は流用
- 循環依存の赤太線警告・rel-target フィルタ・外部パッケージの薄色表示は価値が高いので踏襲
- Go では PHPDoc 型推測のような複雑さは不要。逆に Go 固有の概念（埋め込み・暗黙の interface 実装・exported/unexported・レシーバ）のモデル化が本質的な仕事
- ソースは [smeghead/php-class-diagram](https://github.com/smeghead/php-class-diagram)（実装で迷ったら参照可）
