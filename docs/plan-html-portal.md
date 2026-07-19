# HTML ポータル出力機能 実装計画

計測（解析）結果を 1 コマンドでまとめて生成し、`index.html` を開くだけで
図・レポート・要約をすぐに閲覧できる「ポータルページ」を追加する。

## 決定済みの要件

1. 新フラグ `--html=<dir>` で「定番セット」を一括生成する
   - クラス図・パッケージ依存図（Mermaid でブラウザ描画 + PlantUML ソース同梱）、Markdown レポート、構造要約
   - それらへリンクする `index.html`（ポータル）
2. **ポータルでの図の描画は Mermaid に一本化**する（psap の方針に準拠）。
   PlantUML は `.puml` ソースとして出力・リンクし、SVG 化は従来の Docker 運用（docs/outputs.md 案内済み）に委ねる。
   ローカル PlantUML 検出による事前 SVG 化は初回スコープ外（フォローアップ参照）
3. `mermaid.min.js` は `go:embed` で同梱しオフライン完結。外部 CDN・外部 Go 依存は追加しない
   （go.mod の依存ゼロ方針を維持）
4. 既存のフィルタ・表示系フラグ（`--include` 等）はポータルモードでも尊重する
5. 解析対象のコードを外部へ送信しない。この保証はユニットテストで担保する（§7）

## 1. CLI インターフェース

### 新フラグ

- `--html=<dir>`: ポータル出力先ディレクトリ。無ければ `os.MkdirAll`、既存ファイルは上書き
  （上書きのみ・無関係ファイルは削除しない）

### 併用/排他ルール（`Run` の既存検証ブロックに追記）

| 組み合わせ | 扱い |
|---|---|
| `--html` + `--summary` / `--report` / `--class-diagram` / `--package-diagram` | エラー（ポータルは全部入りのため矛盾） |
| `--html` + `--format` | harmless に無視（既存 `--summary` と同じ流儀で usage に明記） |
| `--html` + フィルタ/表示系すべて | 尊重（既存パイプラインをそのまま通す） |

エラーメッセージ例（既存文体に合わせる）:

```
Error: --html and --summary cannot be used together. --html already generates a summary page.
```

### usage 追記（`--summary` の前あたり）

```
  --html=<dir>        Write an HTML portal to <dir> instead of printing to
                      stdout: class and package diagrams (Mermaid, rendered
                      offline in the browser, with PlantUML sources included),
                      a Markdown report, a structural summary, and an
                      index.html linking them. Cannot be combined with
                      --class-diagram, --package-diagram, --summary, or
                      --report. --format is ignored (harmless).
```

`Options` に `HTMLDir string` を追加。

## 2. 新パッケージ `internal/portal`

```
internal/portal/
├── portal.go          // Generate(): 生成物一式の書き出しオーケストレーション
├── page.go            // html/template によるページ描画（index/図/テキスト）
├── assets.go          // go:embed 宣言（templates/, assets/）
├── templates/
│   ├── index.html.tmpl    // ポータル（カード一覧 + メタ情報）
│   ├── mermaid.html.tmpl  // Mermaid 図ページ（図ソースをテキストノードで埋め込み）
│   ├── puml.html.tmpl     // PlantUML ソース表示ページ（<pre> + SVG 化手順の案内）
│   ├── report.html.tmpl   // report.md を marked.js でクライアントサイド描画
│   └── text.html.tmpl     // summary を <pre> 表示
├── assets/
│   ├── style.css
│   └── vendor/
│       ├── mermaid.min.js       // vendored
│       ├── mermaid.version.txt  // バージョン・取得元 URL・sha256
│       ├── MERMAID-LICENSE      // MIT ライセンス全文
│       ├── marked.min.js        // vendored（Markdown → HTML、約 40KB）
│       ├── marked.version.txt
│       └── MARKED-LICENSE       // MIT ライセンス全文
└── portal_test.go
```

### 主要 API

```go
// portal.go
type Artifacts struct {
    ClassMermaid, PackageMermaid   string // レンダリング済み .mmd テキスト
    ClassPlantUML, PackagePlantUML string // レンダリング済み .puml テキスト
    Summary                        string
    ReportMarkdown                 string
}

type Meta struct {
    Dir, ModulePath, Version string // index.html ヘッダ表示用
}

type Result struct {
    IndexPath string
}

func Generate(outDir string, a Artifacts, meta Meta) (*Result, error)
```

責務分割: レンダリング（mermaid/plantuml テキスト・summary・report の生成）は
呼び出し側 = `internal/cli` が行い、portal は「ファイル群と HTML を書くだけ」。
これにより portal は diagram/render に依存せず、文字列注入だけで単体テストできる。

### 出力ディレクトリレイアウト

```
<dir>/
├── index.html
├── assets/{mermaid.min.js, marked.min.js, style.css}
├── class-diagram.mmd / package-diagram.mmd          (ソース)
├── class-diagram.html / package-diagram.html        (Mermaid ブラウザレンダリング)
├── class-diagram.puml / package-diagram.puml        (ソース)
├── class-diagram-puml.html / package-diagram-puml.html  (ソース表示 + SVG 化手順案内)
├── report.md / report.html                          (marked.js で Markdown 描画 + 埋め込み Mermaid も描画)
└── summary.txt / summary.html
```

## 3. HTML 設計上の要点

- **file:// で完結させるため、Mermaid ソースは fetch せず生成時にテキストノードとして
  `<pre class="mermaid">` へ埋め込む**（file:// の fetch は CORS で失敗する。最重要の設計制約）。
  `.mmd` はコピー/再利用用に別途書き出す
- **Mermaid 初期化は `mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', maxTextSize: 900000 })`
  とし、`mermaid.run()` を明示呼び出しする**。diagoram は第三者リポジトリを解析するツールであり、
  型名・コメント経由の HTML 注入経路を作らないため `strict` を採用（psap と同方針）。
  デフォルト maxTextSize（5 万文字）は大規模図で超えるため引き上げる
- **maxEdges（Mermaid flowchart 既定 500）対策は Go 側で事前判定する**: パッケージ図のエッジ数を
  Generate 前に数え、しきい値超過時は描画を行わず「グラフが大きいため描画をスキップした。
  `--exclude-dir` / `--include-dir` / `--rel-target` で対象を絞るか、ソースを外部ビューアで
  利用する」旨のメッセージ + ソース表示にフォールバックする。クラス図（classDiagram）も
  同様にソースサイズ・要素数で事前判定する。クライアント側にも `mermaid.parseError` /
  `run()` の reject ハンドラで `<pre>` ソース表示への切替を入れ、二段構えとする
- CSP メタタグ: 外部参照をブラウザ側でも遮断する `<meta http-equiv="Content-Security-Policy" ...>`
  の付与を検討する。ただし本ポータルは複数ページ + 共有 `assets/` 構成で、file:// における
  `script-src 'self'` の挙動はブラウザ差があるため、**具体値は実装時（ステップ 2）に実ブラウザで
  確定**する。外部送信なしの主保証はユニットテスト（§7）に置く
- `html/template` の自動エスケープと Mermaid の HTML エンティティ解釈の相性は、
  ジェネリクス記法（`List~T~`）を含む fixture の実ブラウザ確認をステップ 2 の検証項目にする
- index.html は 6 枚のカード（Class/Mermaid, Package/Mermaid, Class/PlantUML source,
  Package/PlantUML source, Report, Summary）+ ヘッダに Meta（dir, module, version）。
  CSS は 100 行程度、外部 CDN・Web フォントなし。文言は英語
- タイムスタンプは出力に含めない（golden テストの決定性のため）

## 4. JS アセットの vendoring（mermaid.min.js / marked.min.js）

- mermaid 入手元: `https://cdn.jsdelivr.net/npm/mermaid@<ver>/dist/mermaid.min.js`
  （UMD 単一ファイル。ESM 版はチャンク分割されるため不可）。11.x にピン留め
- サイズ約 2.5〜3 MB。まず素の embed で実装し、問題になれば gzip embed + `compress/gzip` 展開（約 900 KB）へ切替
  （assets.go に閉じているので差し替えは局所的）
- marked 入手元: `https://cdn.jsdelivr.net/npm/marked@<ver>/marked.min.js`（UMD 単一ファイル、約 40KB、MIT）。
  report.html で `marked.parse()` により Markdown をクライアントサイド描画し、
  変換後の `pre > code.language-mermaid` を `<pre class="mermaid">` に差し替えてから `mermaid.run()` を呼ぶ。
  これによりレポート埋め込みの Mermaid 図もポータル上でそのまま閲覧できる。
  Go 側に Markdown 変換ライブラリは追加しない（go.mod の依存ゼロ方針を維持）
- 両方ともリポジトリにコミット。`*.version.txt` にバージョン・URL・sha256 を記録、
  `.gitattributes` に `internal/portal/assets/vendor/* linguist-vendored` を追加、MIT ライセンス全文を同梱

## 5. PlantUML の扱い

- ポータルでは **描画しない**。`.puml` ソースを出力し、`*-puml.html`（ソース表示ページ）から
  閲覧・コピーできるようにする
- `*-puml.html` と docs には、既存の SVG 化手順（`plantuml/plantuml` Docker イメージ）への
  案内を記載する（docs/outputs.md の該当節へのリンク）
- ローカル PlantUML（PATH / plantuml.jar / Docker）検出による事前 SVG 化は、
  需要が確認できた時点で `--plantuml=auto|off|<command>` フラグとして後付けする（フォローアップ参照）。
  portal.Generate の入力は文字列注入型なので、SVG を追加する拡張は Artifacts へのフィールド追加で閉じる

## 6. `internal/cli.Run` からの呼び出し（変更最小化）

変更は cli.go（小 diff）+ 新規 `internal/cli/portal.go` の 2 点。

1. `parseArgs`: フラグ登録 1 行 + Options フィールド 1 つ
2. usage 定数に 1 項目追記
3. `Run` の検証ブロックに排他チェック 4 つを追加
4. `Run` の `modulePath` 取得直後に分岐挿入:

```go
if opts.HTMLDir != "" {
    return runPortal(opts, pkgs, warnings, modulePath, parseOptions, stdout, stderr)
}
```

既存の出力分岐コードは一切変更しない。

5. `runPortal`（下請けは全部既存関数）:
   - class 系: `diagram.BuildWithModulePath` → `FilterUnexported` / `FilterAliases` / `FilterByRelTarget`
     （`Run` 内と同じ適用順。約 15 行は意図的に複製し、共通化は後回し）
   - package 系: `diagram.BuildPackageGraph(pkgs, modulePath, opts.ShowExternal)`
   - `mermaid.New()` / `plantuml.New()` で 4 テキスト生成
   - `diagram.Summary(...)`、`markdownReport(...)` を再利用
   - `portal.Generate(opts.HTMLDir, artifacts, meta)`
   - 成功時: `fmt.Fprintf(stdout, "Portal written to %s\n", result.IndexPath)`

## 7. テスト戦略

1. **portal 単体（portal_test.go）**: 固定の小さな Artifacts を `t.TempDir()` へ Generate。
   - ファイル一式の存在、index.html のリンク検証
   - HTML は `testutil.Golden` で golden 比較（タイムスタンプ非出力が効く）
   - **外部 URL 不在テスト**: 生成された全 HTML に `https://` / `http://` の参照
     （コメント・ライセンス表記を除く）が含まれないことを検証。
     「解析対象コードを外部へ送信しない」保証の中核テスト（psap と同方針）
   - maxEdges 超過時のフォールバック（スキップメッセージ + ソース表示）の分岐テスト
2. **cli E2E（cli_test.go に TestRunE2E_HTML 追加）**: `Run([]string{"--html", tmp, fixtures + "/basic"}, ...)`
   で exit 0、生成された `class-diagram.mmd` 等が既存 golden（`expected-class.mmd` / `.puml` /
   `expected-summary.txt`）と完全一致することを確認。排他エラーは既存 TestRun のテーブルに 4 行追加
3. **CI**: 外部ツール依存ゼロなので追加のセットアップ不要で決定的

## 8. 実装ステップ（独立性の高い 4 ステップ）

| # | 内容 | 依存 | 完了条件 |
|---|---|---|---|
| 1 | mermaid.min.js / marked.min.js の vendoring（取得・version.txt・LICENSE・.gitattributes） | なし | sha256 記録済みでコミットされている |
| 2 | internal/portal コア（Generate / templates / embed / style.css / maxEdges 事前判定）+ portal_test.go | 1 | `go test ./internal/portal` green、file:// で Mermaid 描画・strict 動作・CSP 値を実ブラウザ確認 |
| 3 | CLI 配線（Options / parseArgs / usage / 検証 / Run 分岐 + runPortal） | 2 | `go run ./cmd/diagoram --html=/tmp/p .` でポータル一式が生成される |
| 4 | E2E テスト + ドキュメント（README・docs/outputs.md・docs/options.md） | 3 | `go test ./...` green |

各ステップは単独コミット可能。

## 9. リスクと未決事項

**リスク**

- Mermaid の HTML エスケープ相性（バージョン依存）と `securityLevel: 'strict'` でのラベル表示
  → ステップ 2 で実データ（ジェネリクス記法含む fixture）の実ブラウザ検証を必須化
- file:// 上の CSP（`script-src 'self'` のブラウザ差）→ 実装時に確定。主保証は外部 URL 不在テスト
- runPortal と Run のフィルタ適用ロジック約 15 行の重複 → 将来 `buildClassDiagram()` として共通化
- バイナリ/リポジトリ +3MB → 問題になれば gzip embed へ切替

**決定済み（レビュー反映）**

- ポータル描画は Mermaid 一本化。PlantUML はソース出力 + 既存 Docker 運用の案内に留める
  （psap の方針に準拠。実装コスト・環境依存を削減し、どの環境でも決定的に同じポータルが出る）
- `securityLevel: 'strict'` + `startOnLoad: false` + テキストノード埋め込み（HTML 注入経路を作らない）
- maxEdges / maxTextSize は Go 側の事前判定 + クライアント側フォールバックの二段構え
- report.md の HTML 化: vendored marked.min.js によるクライアントサイド描画を採用
  （Go 依存ゼロを維持しつつ、レポート内の Mermaid 図も描画される。§4 参照）
- 出力ディレクトリが非空の場合: 「上書きのみ・削除しない」を既定とする

**フォローアップ（今回スコープ外・後で対応可能）**

- ローカル PlantUML 検出による事前 SVG 化（`--plantuml=auto|off|<command>`）: 需要が確認できたら追加。
  Artifacts へのフィールド追加で閉じる設計にしてある
- dogfooding のポータル公開: 本機能マージ後に GitHub Actions ワークフローを 1 本追加し、
  `go run ./cmd/diagoram --html=_site --exclude-dir=tmp .` → `actions/deploy-pages` で
  GitHub Pages に自動公開できる（リポジトリ設定で Pages を有効化するだけ。追加実装は不要）。
  今回は README にコマンド例のみ記載
