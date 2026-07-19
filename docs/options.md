# オプション一覧

```sh
diagoram [options] <dir>
```

## 出力

| オプション | 説明 |
|---|---|
| `--class-diagram` | 型と依存関係を図で出力。デフォルト |
| `--package-diagram` | パッケージ依存図を出力 |
| `--summary` | 型と依存関係をテキストで要約 |
| `--report` | 構造要約、Mermaid図、解析条件、警告をMarkdownで出力 |
| `--html=<dir>` | 型・パッケージ依存図、レポート、要約を`<dir>`へ一括生成し、`index.html`から閲覧できるHTMLポータルを作る（[出力形式と読み方](outputs.md#htmlポータル)参照） |
| `--format=mermaid\|plantuml` | 図の形式を指定。デフォルトは `mermaid` |
| `--show-external` | パッケージ依存図に外部パッケージを含める |

`--report` は `--summary`、`--package-diagram` と併用できません。`--class-diagram` と `--package-diagram` も併用できません。`--html` は `--class-diagram`、`--package-diagram`、`--summary`、`--report` のいずれとも併用できません(すでにポータルへ含まれるため)。`--format` は `--html` と併用した場合も無視されます(harmless)。

## 表示内容

| オプション | 説明 |
|---|---|
| `--public-api` | 非公開identifierとinternal・example・test・benchmark packageを除外 |
| `--hide-unexported` | unexportedの型、フィールド、メソッドを隠す |
| `--hide-aliases` | type aliasと接続edgeを隠す |
| `--show-constants` | 名前付き型に属する定数を表示 |
| `--show-functions` | package-level functionを表示 |
| `--disable-fields` | フィールドを表示しない |
| `--disable-methods` | メソッドを表示しない |
| `--disable-implements` | 推定したinterface実装関係を表示しない |
| `--show-edge-reasons` | edgeにfield、map-keyなどの発生理由を表示 |
| `--max-members=N` | fields、methods、constants、functionsを種類ごとに最大N件表示 |

`--max-members=0` は件数を制限しません。省略時は `0` です。

## 型とメンバーの絞り込み

| オプション | 説明 |
|---|---|
| `--rel-target='A,B'` | 指定した型から辿れる型だけを表示。複数指定可 |
| `--rel-target-depth=N` | `--rel-target` から辿る深さ。デフォルトは `1` |
| `--function='glob'` | 表示するpackage-level functionを名前で絞る。複数指定可 |
| `--method='glob'` | 表示するmethodを名前で絞る。複数指定可 |
| `--receiver='glob'` | methodをreceiverの基底型名で絞る。複数指定可 |

型名は `Product` または `attribute.Color` の形式で指定します。

```sh
diagoram --rel-target=Product --rel-target-depth=2 .
diagoram --public-api --function='New*' --method='Get*' .
diagoram --receiver='*Service' .
```

## ファイルとディレクトリの絞り込み

| オプション | 説明 |
|---|---|
| `--include='glob'` | 対象ファイルを指定。複数指定可。デフォルトは `*.go` |
| `--exclude='glob'` | 除外ファイルを指定。複数指定可。デフォルトは `*_test.go` |
| `--include-dir='glob'` | 対象directoryとその配下を相対パスで指定。複数指定可 |
| `--exclude-dir='glob'` | 除外directoryを相対パスまたは名前で指定。複数指定可 |
| `--exclude-generated` | 標準markerを持つ生成Goファイルを除外 |
| `--generated-only` | 標準markerを持つ生成Goファイルだけを解析 |

`--exclude` を一度でも指定すると、デフォルトの `*_test.go` は置き換えられます。

## Build context

デフォルトではbuild constraintが異なるファイルをまとめて解析します。特定の環境で選ばれるファイルだけを解析する場合はbuild contextを指定します。

| オプション | 説明 |
|---|---|
| `--build-context=union\|current` | source unionまたは現在のGo build contextを明示 |
| `--goos=GOOS` | 指定したGOOSのbuild constraintを適用 |
| `--goarch=GOARCH` | 指定したGOARCHのbuild constraintを適用 |
| `--build-tag=tag` | build tagを追加。複数指定可 |

```sh
diagoram --build-context=current .
diagoram --goos=linux --goarch=amd64 --build-tag=enterprise .
```

`--build-context=union` は `--goos`、`--goarch`、`--build-tag` と併用できません。

## その他

| オプション | 説明 |
|---|---|
| `-h`, `--help` | ヘルプを表示 |
| `-v`, `--version` | バージョンを表示 |
