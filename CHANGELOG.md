# Changelog

All notable changes to diagoram are documented in this file.

diagoram was developed privately before its first public release; the
pre-release milestones were folded into v0.1.0 below, and their history
lives in git.

## [v0.1.1] - 2026-07-26

Release-infrastructure updates; no changes to the CLI itself.

### Changed
- Docker image tags no longer carry the "v" prefix (`0.1.1`, not
  `v0.1.1`), following common registry convention. `latest` now only
  moves for stable `X.Y.Z` releases. The `v0.1.0` image remains
  available under both `v0.1.0` and `0.1.0`.
- Binaries and the Docker image are built with Go 1.26 (previously
  Go 1.24, which no longer receives security fixes).

### Added
- The release workflow can be run manually to verify the Docker build
  and tag derivation without pushing a git tag; manual runs never
  write release tags.

## [v0.1.0] - 2026-07-19

Initial public release.

### Added
- Static analysis of Go source with `go/parser`. No build or dependency
  fetching required; files with syntax errors are skipped with a
  warning.
- Types-and-relationships output (default) and package dependency
  diagram (`--package-diagram`), rendered as Mermaid (default) or
  PlantUML (`--format=plantuml`). Direct two-package import cycles are
  drawn as a red, bold, bidirectional arrow.
- `--report`: a Markdown analysis report (scope, settings, structural
  summary, diagram, diagnostics) meant to be read by people or handed
  to generative AI together with the code.
- `--summary`: a plain-text structural summary.
- `--html=<dir>`: a self-contained offline HTML portal with
  browser-rendered Mermaid diagrams (zoom/pan, copy buttons), PlantUML
  sources, the report, and the summary. No external CDN; a unit test
  guarantees the generated pages contain no external URLs. Bundled
  mermaid.js/marked.js ship with their MIT licenses.
- Filtering and display options: `--include`/`--exclude`(`-dir`),
  `--public-api`, `--rel-target`/`--rel-target-depth`,
  `--function`/`--method`/`--receiver`, `--hide-unexported`,
  `--max-members`, `--show-edge-reasons`, build-context options, and
  more (see `docs/options.md`).
- Multi-stage `Dockerfile` producing a `scratch`-based image, a release
  workflow building linux/darwin/windows binaries, and a dogfood portal
  published to GitHub Pages at https://shimabox.github.io/diagoram/.
