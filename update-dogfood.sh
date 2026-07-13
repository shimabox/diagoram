#!/usr/bin/env bash
# diagoramで自身のソースコードを解析し、README.mdに埋め込む
# dogfood図を再生成します。生成結果はREADMEのDOGFOODマーカー間へ
# 書き込みます。
#
# - クラス図はdiagoramの言語モデルを定義するinternal/gocodeを対象にします。
#   リポジトリ全体ではなく対象を絞ることで、図を読みやすく保ちます。
# - パッケージ依存図は、ローカル作業用のtmpを除いたリポジトリ全体を
#   対象にし、diagoram本体のパッケージ構成だけを表示します。
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

readme=README.md
class_out=$(mktemp)
package_out=$(mktemp)
trap 'rm -f "$class_out" "$package_out"' EXIT

go run ./cmd/diagoram internal/gocode >"$class_out"
go run ./cmd/diagoram --package-diagram --exclude-dir=tmp . >"$package_out"

splice() {
  local start="$1" end="$2" content_file="$3"
  if ! grep -qF "$start" "$readme" || ! grep -qF "$end" "$readme"; then
    echo "update-dogfood.sh: $readme にマーカー $start / $end が見つかりません" >&2
    exit 1
  fi
  awk -v start="$start" -v end="$end" -v content_file="$content_file" '
    $0 == start {
      print
      print "```mermaid"
      while ((getline line < content_file) > 0) print line
      close(content_file)
      print "```"
      skip = 1
      next
    }
    $0 == end { skip = 0 }
    !skip
  ' "$readme" >"$readme.tmp"
  mv "$readme.tmp" "$readme"
}

splice "<!-- DOGFOOD:CLASS:START -->" "<!-- DOGFOOD:CLASS:END -->" "$class_out"
splice "<!-- DOGFOOD:PACKAGE:START -->" "<!-- DOGFOOD:PACKAGE:END -->" "$package_out"

echo "README.md のdogfood図を更新しました。"
