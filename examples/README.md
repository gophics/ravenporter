# RavenPorter Examples

These examples are small Go programs you can run from the repository root.

Each example is self-contained and can be run from the repository root:

- `go run ./examples/quickstart-import` - start with a local file, a preset, and a few report fields.
- `go run ./examples/input-sources` - use the same import flow with `os.DirFS`, raw bytes, or an `io.Reader`.
- `go run ./examples/batch-import` - import a small asset tree from disk and through an `fs.FS` view of the same tree.
- `go run ./examples/asset-pipeline` - run a fuller workflow with a reusable profile, batch import, validation, JSON export, and cache cooking.
- `go run ./examples/profiles-and-overrides` - save a profile to TOML, load it back, override one setting, and import with it.
- `go run ./examples/inspect-and-export` - check the import report, run validation directly, and export JSON IR for downstream tooling.
- `go run ./examples/cache-roundtrip` - cook an `.rpcache` payload and open it again through `cache`.

The root `example_test.go` file stays as the short pkg.go.dev example surface. This directory is the fuller runnable catalog.
