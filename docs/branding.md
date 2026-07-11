# Branding: app icon & Windows Terminal

This guide covers two things tui-base does to make an app easier to find and to
run with full fidelity on Windows:

1. A real app **icon** embedded in the compiled binary — and how to swap in your
   own without vendoring anything.
2. Automatic relaunch into **Windows Terminal** when the app is started under
   the legacy console, so the Charm v2 rendering features actually light up.

## App icon

### How it works

The icon a compiled binary shows in Explorer, the taskbar, and pinned shortcuts
comes from a Windows **resource object** (`.syso`) that the Go linker embeds
automatically. tui-base generates that resource from a single source of truth:

```
assets/icon.svg                      the master vector (edit this)
  └─ tools/genicon ──────────────────▶ assets/icon.ico            (multi-size icon)
                                       cmd/tui-base/resource_windows_amd64.syso
                                       cmd/tui-base/resource_windows_arm64.syso
```

The `.syso` files are named `resource_windows_<arch>.syso`, so the toolchain
links them only into Windows builds of the matching architecture and ignores
them completely on Linux and macOS. They are committed to the repo, so a plain
`go build` ships the icon with no extra steps.

The generator ([`tools/genicon`](../tools/genicon)) is a standalone Go module.
Keeping it separate means its SVG rasterizer and resource writer never enter the
tui-base library's dependency graph — apps that import tui-base pull in none of
it.

### Regenerate the tui-base icon

After editing `assets/icon.svg`:

```bash
go -C tools/genicon generate .   # from the repo root
```

Commit the regenerated `assets/icon.ico` and `cmd/tui-base/resource_windows_*.syso`.
This is deliberately *not* wired into `go generate ./...`, so the CI drift check
and the release build never depend on the icon toolchain.

### Brand your own app

An app built on tui-base points the published generator at its own artwork. No
vendoring, no separate install:

```bash
go run github.com/jarvisfriends/tui-base/tools/genicon@latest \
    -svg assets/app.svg \
    -ico assets/app.ico \
    -syso ./cmd/myapp \
    -name "My App" \
    -desc "My App" \
    -version 1.4.0
```

That writes `resource_windows_amd64.syso` and `resource_windows_arm64.syso` into
your `./cmd/myapp` package; the next `go build` of that command embeds your icon.
Commit those `.syso` files.

Flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `-svg` | `assets/icon.svg` | Source SVG to rasterize. |
| `-ico` | `assets/icon.ico` | Output multi-resolution `.ico`. |
| `-syso` | *(empty: skip)* | Directory to write `resource_windows_<arch>.syso` into — usually your `main` package. |
| `-sizes` | `256,128,64,48,32,16` | Icon sizes (px) packed into the `.ico`. |
| `-supersample` | `4` | Render each size at N× then downscale (anti-aliasing quality). |
| `-png` / `-png-size` | *(empty)* / `256` | Also write a standalone PNG (good for Windows Terminal tab icons). |
| `-arch` | `amd64,arm64` | GOARCH values to emit `.syso` files for. |
| `-name` / `-desc` / `-version` | `TUI Base` / `-name` / `0.0.0.0` | Product metadata recorded in the Windows resource. |
| `-manifest` | *(empty)* | Optional application manifest (`.xml`) to embed. |

The rasterizer ([resvg](https://github.com/linebender/resvg) compiled to
WebAssembly, run in-process via
[resvg-go](https://github.com/kanrichan/resvg-go)/wazero — no CGO) covers
static SVG very completely: fills, strokes, paths, gradients, patterns,
filters, and `<text>`. Scripting and animation are out of scope. Start from
`assets/icon.svg` and restyle it.

Each size is rendered at `-supersample`× (default 4×) and downscaled with a
Catmull-Rom filter. This matters: rasterizing thin strokes directly at 16–48 px
aliases badly, and those small sizes are exactly what Windows shows in Explorer
and the taskbar. Keep the artwork legible when small — a busy design still
crowds a 16 px cell no matter how it is sampled, so favor bold, few shapes.

> **Scope:** the embedded resource sets the icon for the *binary* (Explorer,
> taskbar, shortcuts). The glyph a terminal shows on its own **tab** is a
> separate thing — see [Windows Terminal tab icon](#windows-terminal-tab-icon).

## Windows Terminal tab icon

The tab icon is **not** a per-tab setting a running app (or a `wt new-tab`
argument) can set — Windows Terminal takes it from the **profile**. So there is
no in-band way to set it; the supported path is to register a profile with the
icon, then launch under that profile.

### 1. Register a profile fragment (the icon lives here)

A **profile fragment** makes the app appear in Windows Terminal's new-tab
dropdown with its own name and icon. It is the documented, persistent branding
path — and it directly serves the "easy to find" goal.

```go
tuibase.InstallWindowsTerminalProfile(tuibase.WindowsTerminalProfile{
    AppName:  "My App",
    IconData: embeddedPNGBytes, // written next to the fragment and referenced
})
// later: tuibase.UninstallWindowsTerminalProfile("My App")
```

This writes `%LOCALAPPDATA%\Microsoft\Windows Terminal\Fragments\<AppName>\`
(a `.json` profile plus the icon). Call it from an installer or a setup flag,
**not** on every launch. The reference app exposes it as:

```
tui-base -install-terminal-profile
tui-base -uninstall-terminal-profile
```

After installing, open Windows Terminal's new-tab dropdown (the ⌄ next to the
`+`) to see the branded entry.

Both APIs are Windows-only and return an error elsewhere.

### 2. Make the auto-relaunch open under that profile

Set `TerminalRelaunchConfig.ProfileName` to the installed profile's name. When
tui-base relaunches into Windows Terminal (see below) and that profile is
installed, it launches with `wt --profile <name>`, so the tab shows the
profile's icon and colors. If the profile is not installed, the flag is skipped
and the relaunch still works under the default profile.

```go
router.MaybeRelaunchInWindowsTerminal(router.TerminalRelaunchConfig{
    AppName:     "My App",
    ProfileName: "My App", // matches the installed profile
})
```

The reference app wires this up: `cmd/tui-base` embeds the generated
`tabicon.png`, installs it with the fragment on `-install-terminal-profile`, and
passes `ProfileName` on relaunch — so after a one-time install, relaunched tabs
carry the icon.

> Windows Terminal has no per-tab icon argument; `wt new-tab` does **not**
> accept `--icon`. The icon can only come from a profile, which is why branding
> a relaunched tab means launching under a profile.

## Windows Terminal auto-relaunch

### Why

The Charm v2 stack renders with truecolor, mouse reporting, and rich styling.
The legacy Windows console host (conhost) silently drops those; Windows Terminal
supports them. Windows lets a user pick the default terminal, but that setting
(the `DelegationConsole` / `DelegationTerminal` registry values) is known to be
reset on some machines — which quietly drops apps back into conhost and degrades
the UI.

To make the good experience the default, tui-base detects when it started under
conhost and relaunches itself inside Windows Terminal.

tui-base also exposes the underlying setting itself: the `winterm` package
reads and writes the delegation registry values (`winterm.Detect` /
`winterm.Set`), and the built-in settings page surfaces it as the "Default
Terminal" item, so users can repair a reset default without leaving the app.
The relaunch is the per-session guard; `winterm` is the persistent fix.

### Behavior

`tuibase.Run` / `tuibase.RunContext` perform the check automatically. It
relaunches **only** when *all* of these hold:

- the OS is Windows;
- `wt.exe` is installed (on `PATH`);
- the session is interactive (both stdin and stdout are terminals);
- its console is an actual classic conhost window (window class
  `ConsoleWindowClass`). Every ConPTY-backed host reports `PseudoConsoleWindow`
  instead — including Windows Terminal reached via the **default-terminal
  delegation** (a double-clicked exe when WT is the default terminal), which
  sets no environment markers at all, so this window check is the only way to
  detect it and not open a duplicate window;
- it is **not** already inside a known-good terminal by environment — Windows
  Terminal (`WT_SESSION`), VS Code, WezTerm, ConEmu/Cmder, Alacritty, or an SSH
  session are all detected and left alone; and
- it has not been disabled (see below).

When it relaunches, it opens a new Windows Terminal window running the same
executable with the same arguments, and the original process exits. Otherwise it
does nothing and the app runs in place. Relaunch failures are swallowed — the app
simply continues in the current console.

### Opting out

Per app:

```go
tuibase.Run(tuibase.Options{
    AppName:                 "My App",
    DisableTerminalRelaunch: true,
})
```

Per run, without recompiling — set the environment variable to a truthy value
(`1`, `true`, `yes`, `on`):

```
TUI_BASE_NO_WT_RELAUNCH=1
```

### Relaunching as early as possible

`Run`/`RunContext` already handle this, so most apps need nothing extra. If your
`main` does meaningful setup *before* calling `Run` (and you would rather not run
that setup twice in the throwaway conhost process), call the helper first:

```go
func main() {
    tuibase.EnsureWindowsTerminal() // relaunches and exits if under conhost; no-op otherwise
    // ... your setup, then tuibase.Run(...) ...
}
```

Apps that build the program by hand (via the `router` package) can call the
lower-level primitive directly:

```go
if relaunched, _ := router.MaybeRelaunchInWindowsTerminal(
    router.TerminalRelaunchConfig{AppName: "My App"},
); relaunched {
    return
}
```
