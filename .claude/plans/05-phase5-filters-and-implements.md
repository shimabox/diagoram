# Phase 5: フィルタ・interface 実装検出・summary — 実プロジェクトで実用に

**目的**: 大きなプロジェクトに対して「注目したい場所だけ」を出せるようにし、Go らしい interface 実装関係も図に載せる。
**完了時の姿**: 実プロジェクトで `--rel-target` により注目型の周辺図が切り出せる。完了時に **v0.3.0** タグ。

## 前提
- Phase 4 完了

## 5A. interface 実装検出（ヒューリスティック）

純構文解析の制約下で `types.Implements` の代わりにメソッドシグネチャ照合を行う。

- 対象: 解析対象内の (Struct, Interface) 全ペア
- 判定: interface の**全メソッド**について、struct のメソッドセットに「名前が一致し、かつ引数/戻り値の型表記（`TypeRef.String` を正規化したもの）が一致」するものがあれば実装とみなす
  - 正規化: パッケージ修飾は「同一パッケージ内なら省略される」揺れがあるため、型名部分のみで比較する妥協を許す（過剰に厳密にしない。誤検出よりも見逃しを許容する方針でもよい — 実装時に fixture で振る舞いを決め、決定を本ファイルに追記する）
- **メソッド 0 個の interface（`interface{}` 系）は除外**（全型がマッチしてしまうため）
- 埋め込み interface はメソッドセットを展開してから比較
- エッジ種別 `Implementation` → Mermaid では `A ..|> B`
- クラス図が過密になる場合に備え `--disable-implements` フラグを用意

## 5B. rel-target フィルタ

php-class-diagram の `RelationsFilter` 相当。**IR（Entry/Edge）に対するグラフ探索**として実装する（文字列マッチではなく。ここは本家より素直にやれる）。

- `--rel-target='Product,Config'`: 型名（または `pkg.Type` 形式）で起点集合を指定
- `--rel-target-depth=N`（デフォルト 1）: 起点から依存エッジを from/to 両方向に N ホップ辿って到達した Entry のみ残す
- フィルタ後、残った Entry に接続しないエッジ・空になった PackageNode は出力しない
- 指定した型が見つからない場合は「候補一覧つきのエラー」を出す（開発者フレンドリー）

## 5C. 表示オプション

- `--hide-unexported`: unexported フィールド・メソッドを描かない
- `--disable-fields` / `--disable-methods`: 関係だけのシンプル図（php-class-diagram で推奨されていた使い方）
- `--summary`: 図ではなくプレーンテキストで解析結果の一覧を出す（「テキストでわかりやすい形」要件）

```
diagoram: 12 packages, 34 structs, 8 interfaces

product/
  Product (struct)  fields=3 methods=5  → Name, Price, attribute.Color
  Name    (struct)  fields=1 methods=1
  Stock   (interface) methods=2  ← 実装: Product
...
```

## タスク（TDD 順）

- [x] 5-1. fixture 追加/拡充: interface 実装ペア（実装している/していない/メソッド0個 interface/埋め込み展開）を含むケース
- [x] 5-2. 実装検出のテスト先行（table test で判定ロジックを固定）→ 実装 → クラス図 golden 更新
- [x] 5-3. rel-target フィルタのテスト先行(起点1つ/複数、depth 0..2、見つからない型のエラー) → 実装 → golden
- [x] 5-4. 表示オプション 3 種のテスト → 実装 → golden
- [x] 5-5. `--summary` のテスト先行（golden）→ 実装
- [x] 5-6. ドッグフーディング: 手頃な OSS の Go プロジェクト 1 つ（例: 自分の別リポジトリ）に実行し、実用性を確認。気づきを本ファイル末尾に追記
- [x] 5-7. コードレビュー → コミット → **v0.3.0** タグ

## 受け入れ基準
- 実装検出の判定基準がテストで文書化されている（どこまで厳密かが golden/table test から読み取れる）
- rel-target が IR レベルのグラフ探索で実装されている
- ヘルプ（`-h`）に全オプションの説明が揃っている

## スコープ外
- PlantUML（Phase 6）

## 実装時の決定事項（Phase 5 完了時に記録）

### 5A. interface 実装検出
- シグネチャ照合の正規化（`internal/diagram/implements.go` の `normalizeTypeRef`）: パッケージ修飾を**常に無視**し、ポインタ/スライス/マップの形状 + 型名（無名型は `TypeRef.String` にフォールバック）だけで比較する。誤検出よりも見逃しを許容する方針を徹底し、「同じ型かどうか揺れる」ケースは全て一致扱いにする単純なルールにした（`model.User` と別パッケージの `User` は区別しない）
- struct の埋め込みによるメソッド昇格は**1 段のみ**（埋め込みフィールドの埋め込み先はさらに辿らない）。interface の埋め込みは**再帰的に完全展開**する（サイクルガードあり）。非対称にした理由: struct 側は「無限再帰の危険」が明記されていた一方、interface 埋め込みは interface 同士の DAG なのでメモ化 + visiting セットで安全に完全展開でき、実務上も `Shape embeds Named` のような多段埋め込みを取りこぼす方が驚きが大きいため
- struct が埋め込みフィールドとして **interface** を埋め込んでいる場合も、その interface の（展開済み）メソッドセットを 1 段分だけ昇格対象に含めた（Go の実際のメソッドセット規則と一致）
- メソッド名が一致し引数/戻り値の型正規化文字列が一致すれば実装とみなす（`implementsAll`/`signaturesMatch`）。メソッド 0 個の interface は展開後の合計で判定し、除外する
- 対象は解析対象内の **(Struct, Interface) 全ペア**（`buildImplementationEdges`）。同一パッケージに限らず、import 関係が無いパッケージ同士でも検出する（`testdata/fixtures/implements/extras` の `Widget` がルートパッケージの `Describable` を実装する例で確認）
- 新規 fixture `testdata/fixtures/implements/`: 直接実装 (`Point`→`Named`)・1 段埋め込み昇格による実装 (`Circle`→`Named`)・メソッド 0 個 interface の除外 (`Empty`)・同名だがシグネチャ不一致で誤検出しないケース (`Labeled.Name() int` vs `Name() string`)・実装者がいない interface (`Sized`)・パッケージを跨いだ実装 (`extras.Widget`→`Describable`) を 1 箇所に収録
- 既存 fixture `interfaces` にも新規エッジが発生した（`Base ..|> Named` / `Circle ..|> Named` / `Circle ..|> Shape`）。fixture のコード自体のドキュメントコメントが「Circle satisfies Shape by embedding Base」と明記していた通りの検出結果であり、golden を更新した
- Mermaid では `A ..|> B`、`--disable-implements` で Implementation エッジのみ非表示にできる（IR 自体は常に完全なまま保持し、表示側でフィルタする設計。理由は次項参照）

### 5B. rel-target フィルタ
- `diagram.FilterByRelTarget(d, targets, depth)` として実装。エッジ集合から隣接リストを構築し、開始集合から**両方向**に BFS（`bfsReachable`）。`depth < 0` は 0 にクランプ
- 名前解決は「裸の型名」と「(パッケージディレクトリの最終セグメント).型名」の両方を index 化（`buildRelTargetIndex`）。同名の型が複数パッケージに存在する場合はエラーにせず全て起点に加える（`--rel-target` はスコープを絞る道具であり、曖昧性より使い勝手を優先）
- 見つからない型は `*RelTargetNotFoundError`（`Missing`/`Candidates` を持つ）を返し、CLI は `Error: no type named "X" found. Available types: ...` の形で全既知型名を提示する
- フィルタ後は Entry を持たない PackageNode・両端が生き残っていない Edge を再帰的に除去した新しい `*Diagram` を返す（`rebuildFiltered`/`filterNode`）。元の `Diagram` と `Entry` ポインタは変更しない（`Entry` は Build 後は不変という既存方針を踏襲）
- CLI 統合: `--rel-target` はクラス図・`--summary` の両方に効く（`diagram.Build` の直後、Summary/Render の前にフィルタ）。`--package-diagram` と同時指定してもエラーにはせず無害に無視する（`--show-external` が `--class-diagram` 時に無害に無視されるのと同じ非対称の前例に合わせた）

### 5C. 表示オプション
- `render.Options` に `HideUnexported`/`DisableFields`/`DisableMethods`/`DisableImplements` を追加し、**IR は変更せずレンダリング時にのみフィルタ**する設計にした（`render.go` のもとのドキュメントコメントが「表示オプションは後続フェーズで render.Options に足す」と明記していた設計意図に合わせた）。`internal/diagram` に `ExportedFields`/`ExportedMethods`（`visibility.go`）を切り出し、mermaid レンダラと `Summary` の両方が同じ「unexported の定義」を共有する
- `diagram.Summary` は `internal/render` に依存できない（`render` が `diagram` に依存する既存の層構造のため）ので、同じ 4 フラグを持つ `diagram.SummaryOptions` を別途定義し、CLI 側で同じ値を両方にコピーする
- `--summary` の出力仕様は仕様書の例をそのまま踏襲しつつ、行の細かい空白は golden で確定（`internal/diagram/summary.go`）:
  - ブロック見出しはパッケージディレクトリパス + `/`（ルートパッケージは `.`）。ブロックを持つパッケージ数だけを「N packages」としてカウントする
  - 型名と `(struct)`/`(interface)` ラベルは `text/tabwriter`（標準ライブラリ、依存ゼロ方針を維持）でブロックごとに揃え、詳細列（`fields=`/`methods=`/`→`/`← implements:`）はタブ区切りにせず 1 セルの自由テキストとして連結する
  - 出力先を Mermaid 由来の "→"/"←" 矢印は残しつつ、仕様書の日本語ラベル「実装」はコードベース全体が英語であることに合わせて `implements` に翻訳した
  - outgoing 依存（Dependency + Embedding）だけを `→` で表示し、Implementation エッジは interface 側の `← implements: ...`（構造体一覧）としてのみ表示する（struct 側には implements 情報を出さない）。仕様書の例（`product/` ブロック）はこの決定と整合する
  - `--disable-fields`/`--disable-methods` は該当セグメントを 0 表示にするのではなく**丸ごと省略**する
- golden: `basic`/`multi-package`/`implements` の 3 fixture に `expected-summary.txt` を追加。`multi-package` が仕様書の `product/` 例に対応する実データ

### ドッグフーディング（diagoram 自身に対する実行確認）
- `go run ./cmd/diagoram --summary .`: 6 packages, 26 structs, 2 interfaces を正しく集計。`internal/render/mermaid.Renderer` の行に `← implements: mermaid.Renderer`（`internal/render.Renderer` interface 側の行）が現れ、パッケージを跨いだ実装検出が実プロジェクトでも機能することを確認
- `go run ./cmd/diagoram --rel-target=Diagram --rel-target-depth=1 .`: `internal/diagram.Diagram` 周辺だけを含む小さなクラス図が出力され、`internal_render_mermaid_Renderer ..|> internal_render_Renderer` の Implementation エッジも含まれることを確認
- `go run ./cmd/diagoram --rel-target=NoSuchType .`: 候補一覧つきエラーで exit 1 になることを確認
- 気づき: `render.Renderer`（interface, 同一パッケージ内で `Options` を裸で参照）と `mermaid.Renderer`（`render.Options` と修飾して参照）は正規化により正しく実装関係として検出された。これは「パッケージ修飾を無視する」という 5A の決定が実プロジェクトで狙い通り機能した具体例

### 外部プロジェクトでのドッグフーディング（オーケストレーター実施、2026-07-11）
- 対象: `~/shimabox/github/Mizu-go`（11 packages, 38 structs, 6 interfaces の実プロジェクト）
- `--summary`: パッケージ跨ぎの実装検出が実データで機能（`core.Particle ← implements: particle.atom, particle.h2o`、`core.Random ← implements: mathRandom, seededRandom`）。一覧は読みやすい
- `--rel-target=Particle --rel-target-depth=1`: Particle interface 周辺（Factory 等）だけの小さな図が切り出せた
- 多値戻り値メソッド（`+Layout(int, int) int, int`）は fixture 未収録パターンだったが、実 mermaid パーサ（mermaid.parse）でクラス図・パッケージ図・rel-target 出力すべて構文 OK を確認済み
- 気づき（将来課題候補）: 多値戻り値の fixture ケースを edge-cases に足しておくと golden で保護できる
