# Changelog

All notable changes to diagoram are documented in this file.

## [v1.0.0] - 2026-07-11

### Added
- Multi-stage `Dockerfile` producing a `scratch`-based image (a few MB)
  with the version embedded via `-ldflags`.
- `README.md` rewritten around runnable examples, including
  dogfooded diagrams of diagoram's own source.
- `update-dogfood.sh` to regenerate the dogfooded diagrams in
  `README.md` from the current source tree.
- `.github/workflows/release.yml`: on a `v*` tag push, builds
  linux/darwin/windows amd64/arm64 binaries, attaches them to a GitHub
  Release, and pushes a Docker image to GHCR.
- `CHANGELOG.md` (this file).

## [v0.4.0] - 2026-07-11

### Added
- PlantUML renderer (`--format=plantuml`) for both the types-and-relationships output
  and the package dependency diagram, matching the Mermaid renderer's
  output feature-for-feature (fields, methods, visibility,
  dependency/embedding/implementation edges, mutual-dependency
  warning).

## [v0.3.0] - 2026-07-11

### Added
- `--rel-target='A,B'` / `--rel-target-depth=N`: scope the types-and-relationships output
  to only the types reachable from the given type names, for large
  codebases where a full diagram is too dense to read.
- Heuristic interface implementation detection (`..|>` edges) across
  every analyzed struct/interface pair, including via one level of
  struct embedding; excludes zero-method interfaces.
- `--disable-implements` to turn the above off.
- `--summary`: a plain-text listing of the analyzed types, independent
  of any diagram renderer.
- Display options: `--hide-unexported`, `--disable-fields`,
  `--disable-methods`.

## [v0.2.0] - 2026-07-11

### Added
- `--package-diagram`: a package dependency graph, with packages drawn
  as a nested tree matching the directory structure.
- Direct two-package import cycles are drawn as a red, bold,
  bidirectional arrow, so problematic coupling is visible at a glance.
- `--show-external` to also draw packages outside the analyzed
  directory (standard library, other modules) as light-colored nodes.

## [v0.1.0] - 2026-07-11

### Added
- First working end-to-end MVP: `diagoram <dir>` analyzes Go source
  with `go/parser`/`go/ast` and visualizes types and relationships in Mermaid
  (structs, interfaces, fields, methods, exported/unexported
  visibility, dependency and embedding edges) to stdout.
- `--include`/`--exclude` glob filters (default `*.go` / `*_test.go`).
- Golden-file test harness (`-update` flag) and CI (`go vet`, `gofmt
  -l`, `go test`).
