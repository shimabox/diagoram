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

- [ ] 5-1. fixture 追加/拡充: interface 実装ペア（実装している/していない/メソッド0個 interface/埋め込み展開）を含むケース
- [ ] 5-2. 実装検出のテスト先行（table test で判定ロジックを固定）→ 実装 → クラス図 golden 更新
- [ ] 5-3. rel-target フィルタのテスト先行(起点1つ/複数、depth 0..2、見つからない型のエラー) → 実装 → golden
- [ ] 5-4. 表示オプション 3 種のテスト → 実装 → golden
- [ ] 5-5. `--summary` のテスト先行（golden）→ 実装
- [ ] 5-6. ドッグフーディング: 手頃な OSS の Go プロジェクト 1 つ（例: 自分の別リポジトリ）に実行し、実用性を確認。気づきを本ファイル末尾に追記
- [ ] 5-7. コードレビュー → コミット → **v0.3.0** タグ

## 受け入れ基準
- 実装検出の判定基準がテストで文書化されている（どこまで厳密かが golden/table test から読み取れる）
- rel-target が IR レベルのグラフ探索で実装されている
- ヘルプ（`-h`）に全オプションの説明が揃っている

## スコープ外
- PlantUML（Phase 6）
