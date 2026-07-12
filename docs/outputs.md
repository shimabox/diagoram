# 出力形式と読み方

diagoramはGoソースを `go/parser` と `go/ast` で解析します。対象プロジェクトのビルドや依存パッケージの取得は行いません。

## 解析レポート

```sh
diagoram --report --public-api --max-members=20 . > analysis.md
```

生成AIやレビュー担当者へ渡すためのMarkdownです。

- 解析対象のディレクトリとGo module
- GOOS、GOARCH、build tag
- 適用した絞り込み条件
- 型と依存関係の構造要約
- 依存が生じた理由を付けたMermaidクラス図
- 解析できなかったファイルの警告

レポート本文は英語で出力されます。diagoram自身は生成AIを呼び出さず、ソースコードから得た事実を一定の形式で出力します。設計評価や改善案は、このレポートを渡した生成AIやレビュー担当者が作成します。

大きなコードベースでは、必要な範囲に絞ると扱いやすくなります。

```sh
diagoram --report --public-api --include-dir=api --max-members=20 . > api-analysis.md
diagoram --report --rel-target=Order --rel-target-depth=2 . > order-analysis.md
```

## クラス図

引数に指定したディレクトリ以下を解析し、Mermaidのクラス図を標準出力へ出します。

```sh
diagoram ./shop
```

struct、interface、名前付きのscalar・array・slice・map・function型、type aliasを表示します。フィールド、メソッド、関数の引数と戻り値から型同士の依存関係も検出します。

| 記法 | 意味 |
|---|---|
| `..>` | フィールド、メソッド引数、戻り値などによる依存 |
| `--\|>` | structまたはinterfaceの埋め込み |
| `..\|>` | メソッドシグネチャから推定したinterface実装 |

実装関係は構文情報から推定します。解析対象内の型だけが照合対象です。

依存が生じた理由を図へ表示できます。

```sh
diagoram --show-edge-reasons .
```

## パッケージ依存図

ディレクトリ構造とimport関係を表示します。2つのパッケージが互いを直接importしている場合は、赤い太線で示します。

```sh
diagoram --package-diagram .
```

標準ライブラリや他moduleへの依存も含める場合は `--show-external` を付けます。

```sh
diagoram --package-diagram --show-external .
```

## 構造要約

図を使わず、パッケージごとの型、メンバー数、依存関係をテキストで確認できます。

```sh
diagoram --summary --public-api .
```

## PlantUML

クラス図とパッケージ依存図はPlantUMLでも出力できます。

```sh
diagoram --format=plantuml . > diagram.puml
docker run --rm -v "$PWD:/work" plantuml/plantuml -tsvg /work/diagram.puml
```

`--report` は常にMermaid図を含むため、`--format` の指定は反映されません。

## 解析時の警告

構文エラーなどで解析できないファイルは標準エラー出力へ警告し、残りのファイルの解析を続けます。解析レポートでは同じ警告をDiagnosticsにも収録します。

build constraintがあるファイルの警告には、その条件も表示します。
