# Phase 6: PlantUML レンダラ

**目的**: `--format=plantuml` で PlantUML スクリプトを出力できるようにする。表現力（パッケージの正確なネスト・色）が必要な場面に応える。
**完了時の姿**: クラス図・パッケージ図の両方が PlantUML で出せる。完了時に **v0.4.0** タグ。

## 前提
- Phase 5 完了（IR・オプションが出揃っている）
- レンダラは IR だけを見る。**このフェーズで gocode / diagram 層に手を入れる必要が生じたら設計の匂い**なので、オーケストレーターに相談してから進める

## 設計

- `internal/render` の `Renderer` インターフェースに対する 2 実装目
- `--format=mermaid|plantuml`（デフォルト mermaid）。不正値は候補を示してエラー

### クラス図出力仕様（php-class-diagram 準拠。golden で固定）

```
@startuml class-diagram
  package product as product {
    class "Product" as product_Product {
      -name : Name
      -price : Price
      +Method1(param1) : string
    }
    interface "Stock" as product_Stock
  }
  product_Product ..> product_Name
  product_Product ..|> product_Stock
@enduml
```

- パッケージは `package X as alias { ... }` で**正確にネスト**（Mermaid でフラット化した階層がここでは本来の形で出る）
- struct → `class`、interface → `interface`
- Dependency `..>`、Embedding `--|>`（PlantUML では `<|--` 向き反転に注意。golden で固定）、Implementation `..|>`
- コレクション依存は `"1" ..> "*"` の多重度表記

### パッケージ図出力仕様

- `package` のネスト + `-->`
- **相互依存は `<-[#red,plain,thickness=4]->`**（php-class-diagram と同じ表現）

## タスク（TDD 順）

- [ ] 6-1. 既存 fixture すべてに `expected-class.puml` / `expected-package.puml` の golden を先に書く（オーケストレーターがレビューして確定）
- [ ] 6-2. クラス図レンダラ実装 → golden 全緑
- [ ] 6-3. パッケージ図レンダラ実装 → golden 全緑
- [ ] 6-4. `--format` フラグの CLI 統合テスト（不正値エラー含む）
- [ ] 6-5. 出力を実際に PlantUML でレンダリングして目視確認（Docker: `docker run --rm -i plantuml/plantuml -pipe -tsvg < out.puml > out.svg` などで確認し、確認手順を README 用にメモ）
- [ ] 6-6. コードレビュー → コミット → **v0.4.0** タグ

## 受け入れ基準
- mermaid / plantuml の両レンダラが同じ IR から生成されている（レンダラ追加で解析層に変更が入っていない）
- 全 fixture の .puml golden が実際に PlantUML で構文エラーなくレンダリングできる

## スコープ外
- 画像化の同梱（Phase 7 の Docker で扱う）
- SVG リンク（`--svg-topurl` 相当）は v1.0 後の将来課題
