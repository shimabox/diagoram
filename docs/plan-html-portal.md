# HTML ポータル出力機能 実装計画

計測（解析）結果を 1 コマンドでまとめて生成し、`index.html` を開くだけで
Mermaid / PlantUML の図・レポート・要約をすぐに閲覧できる「ポータルページ」を追加する。

## 決定済みの要件

1. 新フラグ `--html=<dir>` で「定番セット」を一括生成する
   - クラス図（Mermaid / PlantUML）、パッケージ依存図（Mermaid / PlantUML）、Markdown レポート、構造要約
   - それらへリンクする `index.html`（ポータル）
2. Mermaid はブラウザレンダリング。`mermaid.min.js` は `go:embed` で同梱しオフライン完結
3. PlantUML はローカル環境（PATH の `plantuml` / `plantuml.jar` + java / Docker イメージ `plantuml/plantuml`）を検出して事前 SVG 化。
   検出できなければ SVG をスキップし、その旨をポータルに表示 + `.puml` ソースは残す（graceful degradation）
4. 既存のフィルタ・表示系フラグ（`--include` 等）はポータルモードでも尊重する
5. 外部 CDN・外部 Go 依存は追加しない（go.mod の依存ゼロ方針を維持）

## 1. CLI インターフェース

### 新フラグ

- `--html=<dir>`: ポータル出力先ディレクトリ。無ければ `os.MkdirAll`、既存ファイルは上書き（無関係ファイルは消さない）
- `--plantuml=auto|off|<command>`（default `auto`）: PlantUML 実行環境の制御。`off` で SVG 生成を明示スキップ（CI の決定性確保にも必須）。`--html` なしなら harmless に無視

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
                      offline in the browser; PlantUML sources, plus SVG when
                      a local PlantUML is available), a Markdown report, a
                      structural summary, and an index.html linking them.
                      Cannot be combined with --class-diagram,
                      --package-diagram, --summary, or --report. --format is
                      ignored (harmless).
  --plantuml=auto|off|<command>
                      How to render PlantUML SVGs for --html (default
                      "auto": try plantuml on PATH, then $PLANTUML_JAR with
                      java, then the plantuml/plantuml Docker image; "off"
                      skips SVG rendering). Ignored without --html.
```

`Options` に `HTMLDir string` / `PlantUML string` を追加。

## 2. 新パッケージ `internal/portal`

```
internal/portal/
├── portal.go          // Generate(): 生成物一式の書き出しオーケストレーション
├── page.go            // html/template によるページ描画（index/図/テキスト）
├── plantuml.go        // PlantUML Runner の検出・実行
├── assets.go          // go:embed 宣言（templates/, assets/）
├── templates/
│   ├── index.html.tmpl    // ポータル（カード一覧 + ステータス）
│   ├── mermaid.html.tmpl  // Mermaid 図ページ（図ソースをインライン埋め込み）
│   ├── svg.html.tmpl      // PlantUML SVG 閲覧ページ
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
├── portal_test.go
└── plantuml_test.go
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
    IndexPath      string
    PlantUMLStatus string // "plantuml on PATH" / "not found; SVG skipped" 等
}

func Generate(outDir string, a Artifacts, meta Meta, runner Runner) (*Result, error)

// plantuml.go
type Runner interface {
    Describe() string
    RenderSVG(ctx context.Context, pumlSource string) ([]byte, error)
}
func DetectRunner(mode string) (Runner, string, error) // mode = "auto"|"off"|path
```

責務分割: レンダリング（mermaid/plantuml テキスト・summary・report の生成）は
呼び出し側 = `internal/cli` が行い、portal は「ファイル群と HTML を書くだけ」。
これにより portal は diagram/render に依存せず、文字列注入だけで単体テストできる。

### 出力ディレクトリレイアウト

```
<dir>/
├── index.html
├── assets/{mermaid.min.js, style.css}
├── class-diagram.mmd / package-diagram.mmd          (ソース)
├── class-diagram.html / package-diagram.html        (Mermaid ブラウザレンダリング)
├── class-diagram.puml / package-diagram.puml        (ソース)
├── class-diagram.svg / package-diagram.svg          (PlantUML 検出時のみ)
├── class-diagram-plantuml.html / package-diagram-plantuml.html
├── report.md / report.html                          (marked.js で Markdown 描画 + 埋め込み Mermaid も描画)
└── summary.txt / summary.html
```

## 3. HTML 設計上の要点

- **file:// で完結させるため、Mermaid ソースは fetch せず生成時に `<pre class="mermaid">` へインライン埋め込む**
  （file:// の fetch は CORS で失敗する。最重要の設計制約）。`.mmd` はコピー/再利用用に別途書き出す
- Mermaid 初期化は `mermaid.initialize({ startOnLoad: true, maxTextSize: 900000, securityLevel: "loose" })` 程度を直書き
  （デフォルト maxTextSize 5 万文字は大規模図で超えるため必ず引き上げる）
- `html/template` の自動エスケープと Mermaid の HTML エンティティ解釈の相性は、
  ジェネリクス記法（`List~T~`）を含む fixture の実ブラウザ確認をステップ 2 の検証項目にする
- index.html は 6 枚のカード（Class/Mermaid, Package/Mermaid, Class/PlantUML, Package/PlantUML, Report, Summary）
  + ヘッダに Meta（dir, module, version, PlantUML status）。CSS は 100 行程度、外部 CDN・Web フォントなし。文言は英語
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

## 5. PlantUML 検出・実行（plantuml.go）

`mode == "auto"` 時の検出順:

1. `exec.LookPath("plantuml")` → `plantuml -pipe -tsvg`
2. `$PLANTUML_JAR` + `exec.LookPath("java")` → `java -jar <jar> -pipe -tsvg`
3. `docker` があり、かつ `docker image inspect plantuml/plantuml` が成功する場合のみ
   → `docker run --rm -i plantuml/plantuml -pipe -tsvg`（暗黙 pull でハングさせない）

実行設計:

- 図 1 枚ごとに `exec.CommandContext` + timeout 60 秒。stderr はキャプチャしステータス文言に含める
- 失敗・タイムアウトで Generate 全体は失敗させない。SVG をスキップしページに理由を表示、
  `.puml` へのリンクは常に残す。cli 側は stderr に `Warning:` 行（既存 warnings と同じ体裁）
- 成功条件はまず「exit 0 + 非空 stdout」（PlantUML のバージョン差リスクは後述）
- `LookPath` 等は差し替え可能にしてテスト容易性を確保

## 6. `internal/cli.Run` からの呼び出し（変更最小化）

変更は cli.go（小 diff）+ 新規 `internal/cli/portal.go` の 2 点。

1. `parseArgs`: フラグ登録 2 行 + Options フィールド 2 つ
2. usage 定数に 2 項目追記
3. `Run` の検証ブロックに排他チェック 4 つ + `--plantuml` 値検証
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
   - `portal.DetectRunner` → `portal.Generate`
   - 成功時: `Portal written to <path>` + PlantUML status を stdout へ 1 行ずつ

## 7. テスト戦略（PlantUML 非依存で CI が通る）

1. **portal 単体**: 固定 Artifacts を `t.TempDir()` へ Generate。ファイル一式の存在、index.html のリンク・
   ステータス検証。HTML は `testutil.Golden` で golden 比較（タイムスタンプ非出力が効く）。
   `runner=nil` と固定 SVG フェイク Runner の両パス
2. **plantuml 検出**: `t.TempDir()` に stub シェルスクリプトを置き `t.Setenv("PATH", ...)` で
   検出・実行・タイムアウトを検証。Windows は `t.Skip`
3. **cli E2E**: `Run([]string{"--html", tmp, "--plantuml=off", fixtures + "/basic"}, ...)` で exit 0、
   生成された `class-diagram.mmd` 等が既存 golden（`expected-class.mmd` / `.puml` / `expected-summary.txt`）と
   完全一致することを確認。排他エラーは既存 TestRun のテーブルに 4 行追加
4. **CI**: テストは `--plantuml=off` 明示で決定的。SVG バイナリは golden 化しない

## 8. 実装ステップ（独立性の高い 5 ステップ）

| # | 内容 | 依存 | 完了条件 |
|---|---|---|---|
| 1 | mermaid.min.js / marked.min.js の vendoring（取得・version.txt・LICENSE・.gitattributes） | なし | sha256 記録済みでコミットされている |
| 2 | internal/portal コア（Generate / templates / embed / style.css、Runner は IF 定義のみ）+ portal_test.go | 1 | `go test ./internal/portal` green、file:// で Mermaid 描画を実ブラウザ確認 |
| 3 | PlantUML Runner（DetectRunner / 各 Runner / timeout）+ plantuml_test.go | 2 | 検出 3 経路 + off + timeout のテスト green |
| 4 | CLI 配線（Options / parseArgs / usage / 検証 / Run 分岐 + runPortal） | 2, 3 | `go run ./cmd/diagoram --html=/tmp/p .` でポータル一式が生成される |
| 5 | E2E テスト + ドキュメント（README・docs/outputs.md・docs/options.md） | 4 | plantuml 未インストール環境で `go test ./...` green |

2 と 3 は並行作業可。各ステップは単独コミット可能。

## 9. リスクと未決事項

**リスク**

- Mermaid の HTML エスケープ相性（バージョン依存）→ ステップ 2 で実ブラウザ検証を必須化
- 超巨大図の描画限界 → `mermaid.parseError` ハンドラで `<pre>` ソース表示へフォールバック
- PlantUML の exit code / stderr のバージョン差 → 拾えないケースは Warning に落ちるだけで致命的ではない
- runPortal と Run のフィルタ適用ロジック約 15 行の重複 → 将来 `buildClassDiagram()` として共通化
- バイナリ/リポジトリ +3MB → 問題になれば gzip embed へ切替

**決定済み（レビュー回答反映）**

- report.md の HTML 化: vendored marked.min.js によるクライアントサイド描画を採用
  （Go 依存ゼロを維持しつつ、レポート内の Mermaid 図も描画される。§4 参照）
- 出力ディレクトリが非空の場合: 「上書きのみ・削除しない」を既定とする

**未決事項**

- `--plantuml` の最終形（`auto|off|<command>` で足りるか、`docker` 明示指定を許すか）

**フォローアップ（今回スコープ外・後で対応可能）**

- dogfooding のポータル公開: 本機能マージ後に GitHub Actions ワークフローを 1 本追加し、
  `go run ./cmd/diagoram --html=_site --exclude-dir=tmp .` → `actions/deploy-pages` で
  GitHub Pages に自動公開できる（リポジトリ設定で Pages を有効化するだけ。追加実装は不要）。
  今回は README にコマンド例のみ記載
