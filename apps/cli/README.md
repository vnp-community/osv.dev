# apps/cli — OSV.dev CLI Tools

Tất cả CLI binaries và background workers của OSV.dev.

## Commands

| Binary | Mô tả | Nguồn gốc |
|--------|--------|-----------|
| `exporter` | Export vulnerability data | `go/cmd/exporter/` |
| `importer` | Import từ sources | `go/cmd/importer/` |
| `worker` | Background processing worker | `go/cmd/worker/` |
| `gitter` | Git repository sync | `go/cmd/gitter/` |
| `generatesitemap` | Generate sitemap.xml | `go/cmd/generatesitemap/` |
| `recordchecker` | Validate OSV records | `go/cmd/recordchecker/` |
| `relations` | Compute alias/related relations | `go/cmd/relations/` |
| `osv-linter-worker` | OSV schema linting worker | `go/cmd/osv-linter-worker/` |
| `custommetrics` | Custom GCP metrics | `go/cmd/custommetrics/` |
| `extract_versions` | Extract version information | `go/cmd/extract_versions/` |
| `first_package_finder` | Find first affected package | `go/cmd/first_package_finder/` |

## Build

```bash
# Build tất cả
make build

# Build 1 binary
go build -o bin/importer ./cmd/importer/...

# Build tất cả + đặt vào bin/
make all
```

## Dependencies

- Shared library: `services/pkg` (via replace directive)
- Legacy code: `go/` module (transitional, xem TODO trong go.mod)

## Migration Status

> ⚠️ Các binaries này đang trong quá trình migration từ `go/cmd/` sang đây.
> Hiện tại vẫn depend vào `go/internal/*` packages.
> Sau khi tất cả internal packages được port sang `services/`, dependency vào `go/` sẽ được remove.
