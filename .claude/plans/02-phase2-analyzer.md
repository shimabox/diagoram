# Phase 2: 解析器 gocode（Go ソース → 言語モデル）

**目的**: `go/parser` + `go/ast` だけで Go ソースを解析し、図に必要な言語的事実を構造体に落とす。
**完了時の姿**: fixture ディレクトリを与えると `[]gocode.Package` が返り、その内容がテストで検証されている。

## 前提
- Phase 1 完了（golden ヘルパ・CI がある）
- **`go/types` / `golang.org/x/tools` は使わない**（00-overview の確定判断）

## 言語モデル（`internal/gocode`）

```go
// 解析のエントリポイント
func Parse(rootDir string, opt ParseOptions) ([]*Package, []Warning, error)
// Warning = 構文エラー等でスキップしたファイルの報告。error は rootDir 不読等の致命的問題のみ

type ParseOptions struct {
    Includes []string // glob。デフォルト ["*.go"]
    Excludes []string // glob。デフォルトで "*_test.go" を除外
}

type Package struct {
    Dir        string       // rootDir からの相対パス（"." がルート）
    Name       string       // package 節の名前
    Imports    []Import     // パッケージ内全ファイルの import の和集合（重複除去）
    Structs    []*Struct
    Interfaces []*Interface
}

type Import struct{ Alias, Path string }

type Struct struct {
    Name    string
    Doc     string      // doc コメント 1 行目（図の要約表示用）
    Fields  []Field
    Embeds  []TypeRef   // 埋め込みフィールド
    Methods []Method    // レシーバ照合で紐付け（値/ポインタレシーバは区別しない）
}

type Interface struct {
    Name    string
    Doc     string
    Methods []Method
    Embeds  []TypeRef   // 埋め込み interface
}

type Field struct {
    Name     string
    Type     TypeRef
    Exported bool
}

type Method struct {
    Name     string
    Params   []TypeRef
    Results  []TypeRef
    Exported bool
}

// 型参照。依存エッジの材料になる
type TypeRef struct {
    PkgName string // 修飾子（例: "model" ← model.User）。同一パッケージ内なら ""
    Name    string // 型名。プリミティブもそのまま入る（"int", "string", ...）
    IsSlice bool   // []T / [N]T
    IsMap   bool   // map[K]V（V を主参照とし、K は別の TypeRef として展開してよい）
    IsPtr   bool   // *T
    String  string // 表示用の原文に近い表記（例: "[]*model.User"）
}
```

### 解析ルール
- ディレクトリを再帰走査し、**ディレクトリ = 1 パッケージ**として集約（Go の規約通り）。`vendor/`, `testdata/`, `.` 始まりのディレクトリはスキップ
- 1 ファイルずつ `parser.ParseFile`（`parser.ParseComments` 付き）。**構文エラーのファイルは警告を出してスキップ**し、解析全体は止めない（壊れかけのコードでも動く、という価値）
- 抽出対象:
  - `type X struct{...}` → Struct（フィールド、埋め込み、タグは無視）
  - `type X interface{...}` → Interface（メソッド、埋め込み）
  - `func (r X) M(...)` / `func (r *X) M(...)` → 同一パッケージの Struct にメソッドとして紐付け
  - その他の型宣言（`type X int` 等の named type、エイリアス）は **Phase 2 ではスコープ外**（必要になったら拡張）
- ジェネリクス: 型パラメータは**メソッド/フィールドの表示文字列にはそのまま出すが、依存解決には使わない**（過剰実装しない）
- 匿名 struct・関数型フィールドは `String` 表記のみ保持し、依存の対象にしない
- Exported 判定は `ast.IsExported`

## タスク（TDD 順）

### 2-1. fixture 整備
- [x] `testdata/fixtures/basic/`: 単一パッケージ。Product/Name/Price のミニドメイン（php-class-diagram に倣う）。struct・フィールド・メソッド・exported/unexported を網羅
- [x] `testdata/fixtures/multi-package/`: `product/`, `product/attribute/`, `config/` のようなサブパッケージ構成。パッケージ跨ぎの型参照（`attribute.Color` 等）を含む
- [x] `testdata/fixtures/interfaces/`: interface 定義、埋め込み interface、struct の埋め込み、それを満たす struct
- [x] `testdata/fixtures/edge-cases/`: ポインタ・スライス・マップ・ジェネリクス・匿名struct・構文エラーファイル（1 個混ぜる）

### 2-2. ファイル探索（TDD）
- [x] テスト先行 → 実装: include/exclude glob、`vendor/` 等のスキップ、`*_test.go` デフォルト除外

### 2-3. 宣言抽出（TDD）
- [x] テスト先行 → 実装: fixture `basic` を Parse し、Struct/Field/Method/Exported が期待通りか table test で検証
- [x] interface・埋め込み（fixture `interfaces`）
- [x] レシーバによるメソッド紐付け

### 2-4. 型参照と import（TDD）
- [x] TypeRef の分解（`*T`, `[]T`, `map[K]V`, `pkg.T` の組み合わせ）を細かい table test で検証
- [x] Import の収集（alias 対応）
- [x] fixture `multi-package` でパッケージ修飾つき参照が取れること

### 2-5. 堅牢性（TDD）
- [x] 構文エラーファイルをスキップして続行し、stderr 向けの警告リストが返ること（fixture `edge-cases`）

### 2-6. フェーズ完了処理
- [x] オーケストレーターがコードレビュー（モデルの過不足、依存ゼロ、doc コメント）
- [x] コミット

## 受け入れ基準
- 4 つの fixture すべてで Parse 結果がテストで固定されている
- 結果が決定的（ファイル・宣言はパス順/出現順でソート）
- `go test ./...` / `go vet` / `gofmt` クリーン、外部依存ゼロ

## スコープ外
- 図・IR への変換（Phase 3）
- interface 実装の検出（Phase 5。ここでは材料=メソッドセットを揃えるだけ）
- 定数・関数（トップレベル func）の抽出

## 実装時の決定事項（Phase 2 完了時に記録）
- `Parse` は `([]*Package, []Warning, error)` を返す。構文エラーファイルは Warning に積んで続行、error は致命的問題のみ
- `ParseOptions.Excludes` をユーザーが指定した場合、デフォルトの `*_test.go` 除外は**置き換え**られる（マージしない。Includes と同じセマンティクス）
- `map[K]V` はキー K を分解しない（値 V のみ主参照）。可変長引数 `...T` はスライス扱い
- 匿名struct/関数型/チャネル等は Name="" とし String に原文表記のみ保持
- 注意: fixture の `edge-cases/broken.go`（意図的な構文エラー）により `gofmt -l .` は exit 2 になる（GitHub Actions は bash -e なのでステップが落ちる）。CI の gofmt チェックは `gofmt -l cmd internal` と実ソースに限定している
