# Contributing

## Requirements

- Go `1.25.7` or newer
- GNU Make
- Node.js `22.12.0` or newer if you need to work on `docs/`

## Local Setup

1. Clone the repository.
2. Run `go test ./... -count=1` from the repository root.
3. If you are touching the docs site, run `npm ci` in `docs/`.

## Validation

Use the smallest command that covers your change while iterating:

- `make test` for the main Go module
- `make test-integration` for the corpus-backed tests in `testsuite/`
- `make docs-check` for Astro type/content validation
- `make docs-build` for a production docs build
- `make release-check` before opening or merging a release-facing change

## Repo Layout

- `ravenporter`, `cache`, `detect`, `emit`, `ir`, `process`, and `validate` hold the public package surface
- `internal/` contains decoder, pipeline, and processing internals
- `testsuite/` holds larger integration fixtures and golden coverage
- `docs/` contains the Starlight docs site
- `examples/` contains runnable end-to-end examples

## Change Expectations

- keep public docs and examples aligned with behavioral changes
- add or update tests when behavior changes
- prefer focused patches over broad refactors
- do not commit generated caches, local build output, or `docs/dist`
