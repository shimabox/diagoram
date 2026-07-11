#!/usr/bin/env bash
# Regenerates the "dogfooding" diagrams embedded in README.md by running
# diagoram against its own source tree, and splices the fresh output
# between the README's DOGFOOD marker comments.
#
# - Class diagram: internal/gocode, diagoram's own language model. It
#   is used (rather than the whole repo) so the dogfooded diagram stays
#   readable inline in the README instead of a ~40-class wall of text.
# - Package diagram: the whole repo, which is small enough to read as
#   one picture and shows diagoram's actual package layout.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

readme=README.md
class_out=$(mktemp)
package_out=$(mktemp)
trap 'rm -f "$class_out" "$package_out"' EXIT

go run ./cmd/diagoram internal/gocode >"$class_out"
go run ./cmd/diagoram --package-diagram . >"$package_out"

splice() {
  local start="$1" end="$2" content_file="$3"
  if ! grep -qF "$start" "$readme" || ! grep -qF "$end" "$readme"; then
    echo "update-dogfood.sh: markers $start / $end not found in $readme" >&2
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

echo "README.md dogfood diagrams updated."
