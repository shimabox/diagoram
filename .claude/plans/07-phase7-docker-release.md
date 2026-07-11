# Phase 7: Docker・ドキュメント・リリース

**目的**: 「docker で実行できてユーザーの環境に依存しない」「開発者フレンドリー」を仕上げる。
**完了時の姿**: README を読めば 5 分で使い始められ、`docker run` だけで図が出る。**v1.0.0** タグ。

## 前提
- Phase 6 完了

## タスク

### 7-1. Docker
- [x] マルチステージ Dockerfile:
  - build ステージ: `golang:1.24-alpine` で `CGO_ENABLED=0` ビルド
  - 実行ステージ: `scratch` または `gcr.io/distroless/static`。**イメージは数 MB 台**を目指す（純構文解析＋依存ゼロの利点をイメージサイズで示す）
  - `ENTRYPOINT ["/diagoram"]`
- [x] 実行例をテスト（手動確認でよい）:
  ```
  docker run --rm -v "$PWD:/work" ghcr.io/shimabox/diagoram /work
  ```
- [x] （任意）`compose.yaml`: 開発用。php-class-diagram 同様に必要なら
- [x] 判断メモ: php-class-diagram は PlantUML(Java) 同梱の重いイメージだったが、diagoram は Mermaid がテキストのまま GitHub で描画できるため**同梱しない**。PlantUML 画像化したい人向けの手順（plantuml/plantuml イメージへのパイプ）を README に書くことで代替

### 7-2. README（例中心・日英どちらでもよいがまず日本語 + 英語サマリ）
- [x] 構成: バッジ / 1 行説明 / Why / Install（go install・docker・binary）/ Quick Start（入力コード → 出力図の実例。Mermaid は README 内で実際にレンダリングされる形で貼る）/ 全オプション表 / PlantUML 画像化手順 / License
- [x] **ドッグフーディング**: diagoram 自身のクラス図・パッケージ図を README に掲載（php-class-diagram の伝統を踏襲）
- [x] 図の更新スクリプト `update-dogfood.sh`（自分自身に diagoram をかけて README 用の図を再生成）

### 7-3. リリース整備
- [x] GitHub Actions: タグ push で `go build`（linux/mac/windows × amd64/arm64）してリリース添付 + GHCR へ Docker イメージ push（goreleaser を使うなら「リリース用途の dev 依存」として許容。ツール本体の依存ゼロは維持）
- [x] `-ldflags "-X github.com/shimabox/diagoram/internal/cli.version=..."` でバージョン埋め込みが効いていることの確認
- [x] CHANGELOG.md（v0.1.0 からの分をまとめる）

### 7-4. 最終品質チェック（オーケストレーターが実施）
- [x] 著名な中規模 OSS Go プロジェクトに対して実行し、出力が実用に耐えるか確認（描画が破綻しないか、rel-target で救えるか）
- [x] ヘルプ・エラーメッセージの総点検（「何が起きたか + どうすればよいか」になっているか）
- [x] 00-overview の「開発者フレンドリー原則」を README・CLI が満たしているかの突き合わせ
- [x] v1.0.0 タグ・GitHub リリース

## 受け入れ基準
- クリーンな環境（コンテナ）で README の手順だけで図が出せる
- Docker イメージが 10MB 未満
- README の Mermaid 図が GitHub 上で実際にレンダリングされている

## 将来課題（v1.0 後の候補。着手時は新しいフェーズファイルを起こす）
- SCC による間接的な循環依存検出
- SVG リンク（図からソースへジャンプ）
- iota 定数グループの可視化（division 図相当）
- named type（`type UserID int` 等）の解析対象化
- `--format=dot`（Graphviz）

## 実装時の決定事項・最終品質チェック結果（Phase 7 完了時に記録）
- Docker: scratch ベースで **2.69MB**（目標 10MB を大きくクリア）。CA 証明書やシェルは不要（ネットワークアクセスなし）。オーケストレーターが独立に再ビルド・動作確認済み（-v でのバージョン埋め込み、-v マウントでの解析）
- compose.yaml は作らない判断（単一コンテナ CLI は docker run 一発で足りる）
- README: 掲載図はすべて実出力（実装担当が diff で検証）。ドッグフードのクラス図は全体ではなく internal/gocode に限定（可読性優先）。DOGFOOD マーカー間を update-dogfood.sh が差し替える
- release.yml: goreleaser 不使用。素の go build マトリクス + gh release create + buildx で GHCR push。test ジョブが build/docker をゲート。sha256 チェックサム添付
- 最終品質チェック（7-4）: fsnotify v1.9.0（著名 OSS）で class/summary を実行し、実 mermaid パーサで構文 OK を確認。ヘルプは全オプションに「何に効くか・何と併用不可か」を明記済み
- **既知の限界（将来課題）**: ビルドタグで分かれたプラットフォーム別ファイル（fsnotify の backend_*.go 等）は同名型が複数回リストされる（summary で `watch` が 3 回等）。Mermaid 出力は valid だが、GOOS 別のファイルフィルタや同名型のマージは未対応
- GitHub への push・GHCR 公開・Releases はユーザーのタイミング（remote 未設定）。release.yml はタグ push で発火する状態で配置済み
