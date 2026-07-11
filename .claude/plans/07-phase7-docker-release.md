# Phase 7: Docker・ドキュメント・リリース

**目的**: 「docker で実行できてユーザーの環境に依存しない」「開発者フレンドリー」を仕上げる。
**完了時の姿**: README を読めば 5 分で使い始められ、`docker run` だけで図が出る。**v1.0.0** タグ。

## 前提
- Phase 6 完了

## タスク

### 7-1. Docker
- [ ] マルチステージ Dockerfile:
  - build ステージ: `golang:1.24-alpine` で `CGO_ENABLED=0` ビルド
  - 実行ステージ: `scratch` または `gcr.io/distroless/static`。**イメージは数 MB 台**を目指す（純構文解析＋依存ゼロの利点をイメージサイズで示す）
  - `ENTRYPOINT ["/diagoram"]`
- [ ] 実行例をテスト（手動確認でよい）:
  ```
  docker run --rm -v "$PWD:/work" ghcr.io/shimabox/diagoram /work
  ```
- [ ] （任意）`compose.yaml`: 開発用。php-class-diagram 同様に必要なら
- [ ] 判断メモ: php-class-diagram は PlantUML(Java) 同梱の重いイメージだったが、diagoram は Mermaid がテキストのまま GitHub で描画できるため**同梱しない**。PlantUML 画像化したい人向けの手順（plantuml/plantuml イメージへのパイプ）を README に書くことで代替

### 7-2. README（例中心・日英どちらでもよいがまず日本語 + 英語サマリ）
- [ ] 構成: バッジ / 1 行説明 / Why / Install（go install・docker・binary）/ Quick Start（入力コード → 出力図の実例。Mermaid は README 内で実際にレンダリングされる形で貼る）/ 全オプション表 / PlantUML 画像化手順 / License
- [ ] **ドッグフーディング**: diagoram 自身のクラス図・パッケージ図を README に掲載（php-class-diagram の伝統を踏襲）
- [ ] 図の更新スクリプト `update-dogfood.sh`（自分自身に diagoram をかけて README 用の図を再生成）

### 7-3. リリース整備
- [ ] GitHub Actions: タグ push で `go build`（linux/mac/windows × amd64/arm64）してリリース添付 + GHCR へ Docker イメージ push（goreleaser を使うなら「リリース用途の dev 依存」として許容。ツール本体の依存ゼロは維持）
- [ ] `-ldflags "-X github.com/shimabox/diagoram/internal/cli.version=..."` でバージョン埋め込みが効いていることの確認
- [ ] CHANGELOG.md（v0.1.0 からの分をまとめる）

### 7-4. 最終品質チェック（オーケストレーターが実施）
- [ ] 著名な中規模 OSS Go プロジェクトに対して実行し、出力が実用に耐えるか確認（描画が破綻しないか、rel-target で救えるか）
- [ ] ヘルプ・エラーメッセージの総点検（「何が起きたか + どうすればよいか」になっているか）
- [ ] 00-overview の「開発者フレンドリー原則」を README・CLI が満たしているかの突き合わせ
- [ ] v1.0.0 タグ・GitHub リリース

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
