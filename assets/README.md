# Brand assets

| File | Role |
| --- | --- |
| `icon.svg` | Master vector for the app mark. **Edit this**, then regenerate. |
| `icon.ico` | Generated multi-resolution Windows icon (256/128/64/48/32/16 px). Do not edit by hand. |

The icon that a compiled binary shows in Explorer, the taskbar, and shortcuts
comes from a Windows resource (`cmd/tui-base/resource_windows_*.syso`) that is
generated from `icon.ico`. Everything downstream of `icon.svg` is produced by
the [`tools/genicon`](../tools/genicon) generator.

## Regenerate after editing `icon.svg`

```bash
go -C tools/genicon generate .   # from the repo root
# or:  cd tools/genicon && go generate .
```

That rasterizes `icon.svg`, rewrites `icon.ico`, and re-emits the `.syso`
resources under `cmd/tui-base`. Commit the regenerated `.ico` and `.syso` files
so a plain `go build` still ships the icon.

Generation is intentionally **not** part of the repo-root `go generate ./...`,
so the CI drift check and release build never depend on the SVG toolchain — the
committed `.ico`/`.syso` are the source of truth.

## Brand your own app

See [docs/branding.md](../docs/branding.md). In short, an app built on tui-base
points the published generator at its own artwork — no vendoring required:

```bash
go run github.com/jarvisfriends/tui-base/tools/genicon@latest \
    -svg assets/app.svg -ico assets/app.ico -syso ./cmd/app \
    -name "My App" -desc "My App" -version 1.4.0
```

The rasterizer ([oksvg](https://github.com/srwiley/oksvg)) supports a practical
subset of SVG — solid fills, rounded rectangles, simple paths, and linear or
radial gradients. Avoid CSS, filters, and patterns.
