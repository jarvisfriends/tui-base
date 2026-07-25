# tui-base example apps

This binary is one of tui-base's reference applications, distributed as a
standalone release asset — run it directly, no Go toolchain required. Each
one is a small, complete Bubble Tea v2 app built on the `tui-base` framework,
demonstrating a different integration pattern you can copy into your own
project.

```bash
./minimal      # smallest possible tui-base app
./multipage    # custom pages + graceful shutdown
./dashboard    # themed widgets on a single page
./tui-base     # the full reference app (every feature wired together)
```

Every app is fully interactive (mouse + keyboard); quit with `q`, `ctrl+c`,
or `esc` depending on the page. `ctrl+d` opens the built-in
[inspector](https://github.com/jarvisfriends/inspector) overlay in any of
them.

## Apps in this release

Each app ships as its own archive per OS/architecture (this file is included
in all of them), so you only download the one you need.

| binary | what it demos |
| --- | --- |
| `tui-base` | The full reference app: multi-page routing, theming, notifications, and the Ctrl+D inspector. |
| `minimal` | The smallest possible tui-base app — the built-in Home/Settings pages, theming, notifications, and the inspector in one call. |
| `multipage` | Several custom pages with their own settings sections, plus graceful SIGINT/SIGTERM shutdown via `RunContext`. |
| `dashboard` | A single custom page with themed widgets: a `bubbles` table styled via `theme.TableStyles`, live status bar segments, and a key-press-triggered notification. |

Full source and the Go framework these apps are built from live at
<https://github.com/jarvisfriends/tui-base>.
