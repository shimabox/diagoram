# syntax=docker/dockerfile:1

# --- build stage ----------------------------------------------------------
# diagoram is pure go/parser + go/ast with zero external dependencies, so a
# static, from-scratch image is possible and keeps the image tiny (a few
# MB, not hundreds) -- see .claude/plans/07-phase7-docker-release.md.
FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

ARG VERSION=dev

RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w -X github.com/shimabox/diagoram/internal/cli.version=${VERSION}" \
    -o /out/diagoram ./cmd/diagoram

# --- run stage --------------------------------------------------------------
FROM scratch

COPY --from=build /out/diagoram /diagoram

WORKDIR /work

ENTRYPOINT ["/diagoram"]
CMD ["--help"]
