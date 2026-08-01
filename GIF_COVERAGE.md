# GIF & Component Coverage: bubbles, bubbletea, tui-base, dash

Audit of demo GIFs and component coverage across the four repos, as of
2026-07-06.

**TL;DR**

| Repo | Items | Has GIF | Missing | Notes |
|---|---|---|---|---|
| [bubbles](bubbles/) | 14 components | 12 (all remote-hosted) | 2 | **Zero GIFs live in the repo** — README embeds images from `stuff.charm.sh` / `vhs.charm.sh` |
| [bubbletea](bubbletea/) | 63 examples | 36 (committed in-repo) | 27 | Tapes for the 36 existing GIFs live upstream in [vhs/examples/bubbletea](https://github.com/charmbracelet/vhs/tree/main/examples/bubbletea) |
| [tui-base](tui-base/) | ~14 visual features | 0 | all | One tape exists ([tools/demo.tape](tui-base/tools/demo.tape)) but its output GIF isn't committed |
| [dash](dash/) | 8 example tabs | 0 | all | The tabs are purpose-built demo pages — ideal GIF subjects |

Legend: ✅ GIF exists · ❌ needs one · ➖ skip (nothing visual to show)

---

## Part 1 — Bubbles (components)

All existing GIFs are hosted on Charm's servers and referenced by URL from the
README. The fork contains no image assets, so if upstream ever moves/removes
them the README breaks. **Recommendation:** record local GIFs with
[VHS](https://github.com/charmbracelet/vhs), commit them (or push to your own
hosting), and repoint the README.

### Text entry

| Component | GIF | Action | Priority |
|---|---|---|---|
| [textinput](bubbles/textinput/) | ✅ remote | Re-record locally | Med |
| [textarea](bubbles/textarea/) | ✅ remote | Re-record locally | Med |
| [cursor](bubbles/cursor/) | ❌ (not in README) | ➖ skip — internal building block rendered inside textinput/textarea; those GIFs already show it | — |

### Data display & navigation

| Component | GIF | Action | Priority |
|---|---|---|---|
| [list](bubbles/list/) | ✅ remote | Re-record locally | Med |
| [table](bubbles/table/) | ✅ remote | Re-record locally | Med |
| [viewport](bubbles/viewport/) | ✅ remote | Re-record locally | Med |
| [paginator](bubbles/paginator/) | ✅ remote | Re-record locally | Med |
| [filepicker](bubbles/filepicker/) | ✅ remote (vhs.charm.sh) | Re-record locally | Med |

### Feedback & time

| Component | GIF | Action | Priority |
|---|---|---|---|
| [spinner](bubbles/spinner/) | ✅ remote | Re-record locally | Med |
| [progress](bubbles/progress/) | ✅ remote | Re-record locally | Med |
| [timer](bubbles/timer/) | ✅ remote | Re-record locally | Low |
| [stopwatch](bubbles/stopwatch/) | ✅ remote | Re-record locally | Low |

### Chrome & helpers

| Component | GIF | Action | Priority |
|---|---|---|---|
| [help](bubbles/help/) | ✅ remote | Re-record locally | Med |
| [key](bubbles/key/) | ❌ (not in README) | ➖ skip — keybinding definitions have no visual of their own; the help GIF is its showcase | — |

---

## Part 2 — Bubbletea (examples)

GIFs are committed next to each example and embedded in
[examples/README.md](bubbletea/examples/README.md). **The VHS tapes that made
them live upstream in
[charmbracelet/vhs/examples/bubbletea](https://github.com/charmbracelet/vhs/tree/main/examples/bubbletea)**
(36 tapes, matching the 36 committed GIFs — `credit-card-form.tape` there
corresponds to our renamed `isbn-form`, which is also the only tape committed
in our fork). The 27 GIF-less examples are v2-era additions with **no
upstream tape either**, so those need tapes written from scratch. The vhs
repo's parent [examples/](https://github.com/charmbracelet/vhs/tree/main/examples)
directory has more tape references (gum, glow, slides, split panes, …) worth
mining for VHS techniques.

### Getting started / app structure

| Example | GIF | In README | Action | Priority |
|---|---|---|---|---|
| [simple](bubbletea/examples/simple/) | ✅ | ✅ | — | — |
| [fullscreen](bubbletea/examples/fullscreen/) | ✅ | ✅ | — | — |
| [altscreen-toggle](bubbletea/examples/altscreen-toggle/) | ✅ | ✅ | — | — |
| [result](bubbletea/examples/result/) | ✅ | ✅ | — | — |
| [views](bubbletea/examples/views/) | ✅ | ✅ | — | — |
| [composable-views](bubbletea/examples/composable-views/) | ✅ | ✅ | — | — |
| [tabs](bubbletea/examples/tabs/) | ✅ | ✅ | — | — |

### Text input & forms

| Example | GIF | In README | Action | Priority |
|---|---|---|---|---|
| [textinput](bubbletea/examples/textinput/) | ✅ | ✅ | — | — |
| [textinputs](bubbletea/examples/textinputs/) | ✅ | ✅ | — | — |
| [textarea](bubbletea/examples/textarea/) | ✅ | ✅ | — | — |
| [dynamic-textarea](bubbletea/examples/dynamic-textarea/) | ❌ | ❌ | Write tape + add README entry | **High** |
| [autocomplete](bubbletea/examples/autocomplete/) | ❌ | ❌ | Write tape + add README entry | **High** |
| [isbn-form](bubbletea/examples/isbn-form/) | ✅ | ✅ | — (only in-repo `.tape`) | — |
| [split-editors](bubbletea/examples/split-editors/) | ✅ | ✅ | — | — |
| [chat](bubbletea/examples/chat/) | ✅ | ✅ | — | — |
| [cursor-style](bubbletea/examples/cursor-style/) | ❌ | ❌ | Write tape + add README entry | Med |

### Lists, tables & navigation

| Example | GIF | In README | Action | Priority |
|---|---|---|---|---|
| [list-simple](bubbletea/examples/list-simple/) | ✅ | ✅ | — | — |
| [list-default](bubbletea/examples/list-default/) | ✅ | ✅ | — | — |
| [list-fancy](bubbletea/examples/list-fancy/) | ✅ | ✅ | — | — |
| [paginator](bubbletea/examples/paginator/) | ✅ | ✅ | — | — |
| [pager](bubbletea/examples/pager/) | ✅ | ✅ | — | — |
| [table](bubbletea/examples/table/) | ✅ | ✅ | — | — |
| [table-resize](bubbletea/examples/table-resize/) | ❌ | ❌ | Write tape + add README entry | **High** |
| [file-picker](bubbletea/examples/file-picker/) | ❌ | ❌ | Write tape + add README entry | **High** |
| [glamour](bubbletea/examples/glamour/) | ✅ | ✅ | — | — |

### Progress & feedback

| Example | GIF | In README | Action | Priority |
|---|---|---|---|---|
| [spinner](bubbletea/examples/spinner/) | ✅ | ✅ | — | — |
| [spinners](bubbletea/examples/spinners/) | ✅ | ✅ | — | — |
| [progress-static](bubbletea/examples/progress-static/) | ✅ | ✅ | — | — |
| [progress-animated](bubbletea/examples/progress-animated/) | ✅ | ✅ | — | — |
| [progress-bar](bubbletea/examples/progress-bar/) | ❌ | ❌ | Write tape + add README entry | **High** |
| [progress-download](bubbletea/examples/progress-download/) | ❌ | ✅ (code link only) | Write tape — needs a canned/local download so it's reproducible | Med |
| [timer](bubbletea/examples/timer/) | ✅ | ✅ | — | — |
| [stopwatch](bubbletea/examples/stopwatch/) | ✅ | ✅ | — | — |
| [help](bubbletea/examples/help/) | ✅ | ✅ | — | — |

### Async, IO & process control

| Example | GIF | In README | Action | Priority |
|---|---|---|---|---|
| [http](bubbletea/examples/http/) | ✅ | ✅ | — | — |
| [debounce](bubbletea/examples/debounce/) | ✅ | ✅ | — | — |
| [realtime](bubbletea/examples/realtime/) | ✅ | ✅ | — | — |
| [send-msg](bubbletea/examples/send-msg/) | ✅ | ✅ | — | — |
| [sequence](bubbletea/examples/sequence/) | ✅ | ✅ | — | — |
| [exec](bubbletea/examples/exec/) | ✅ | ✅ | — | — |
| [pipe](bubbletea/examples/pipe/) | ✅ | ✅ | — | — |
| [tui-daemon-combo](bubbletea/examples/tui-daemon-combo/) | ✅ | ✅ | — | — |
| [package-manager](bubbletea/examples/package-manager/) | ✅ | ✅ | — | — |
| [prevent-quit](bubbletea/examples/prevent-quit/) | ❌ | ❌ | Write tape + add README entry | Med |
| [suspend](bubbletea/examples/suspend/) | ❌ | ❌ | ➖ skip — ctrl+z drops out of the TUI; a recording shows mostly shell | — |

### Mouse & keyboard

| Example | GIF | In README | Action | Priority |
|---|---|---|---|---|
| [mouse](bubbletea/examples/mouse/) | ❌ | ✅ (code link only) | Write tape — VHS can't move a pointer, but the event log is still demoable | Med |
| [clickable](bubbletea/examples/clickable/) | ❌ | ❌ | Write tape + add README entry | Med |
| [print-key](bubbletea/examples/print-key/) | ❌ | ❌ | ➖ skip — plain key-event echo, low visual value | — |
| [keyboard-enhancements](bubbletea/examples/keyboard-enhancements/) | ❌ | ❌ | Write tape + add README entry (key-release events are visually interesting) | Med |
| [focus-blur](bubbletea/examples/focus-blur/) | ❌ | ❌ | ➖ skip — focus changes happen outside the terminal content VHS records | — |

### Terminal capabilities & eye candy

| Example | GIF | In README | Action | Priority |
|---|---|---|---|---|
| [doom-fire](bubbletea/examples/doom-fire/) | ❌ | ❌ | Write tape + add README entry — great showcase | **High** |
| [eyes](bubbletea/examples/eyes/) | ❌ | ❌ | Write tape + add README entry | **High** |
| [space](bubbletea/examples/space/) | ❌ | ❌ | Write tape + add README entry | **High** |
| [splash](bubbletea/examples/splash/) | ❌ | ❌ | Write tape + add README entry | **High** |
| [canvas](bubbletea/examples/canvas/) | ❌ | ❌ | Write tape + add README entry | Med |
| [cellbuffer](bubbletea/examples/cellbuffer/) | ❌ | ❌ | Write tape + add README entry | Med |
| [colorprofile](bubbletea/examples/colorprofile/) | ❌ | ❌ | Write tape + add README entry | Med |
| [set-terminal-color](bubbletea/examples/set-terminal-color/) | ❌ | ❌ | Write tape + add README entry | Low |
| [capability](bubbletea/examples/capability/) | ❌ | ❌ | ➖ skip — terminfo query tool, output is a text dump | — |
| [query-term](bubbletea/examples/query-term/) | ❌ | ❌ | ➖ skip — raw ANSI query/response, diagnostic in nature | — |
| [set-window-title](bubbletea/examples/set-window-title/) | ❌ | ❌ | ➖ skip — the title bar isn't part of what VHS records | — |
| [window-size](bubbletea/examples/window-size/) | ❌ | ❌ | ➖ skip — prints dimensions on resize; VHS runs at fixed size | — |
| [vanish](bubbletea/examples/vanish/) | ❌ | ❌ | Low value (screen-clear demo), record only if convenient | Low |

**Missing-GIF totals:** 15 to record (7 high, 6 med, 2 low), 8 skipped as
non-visual, plus `mouse` and `progress-download` which are README-listed but
GIF-less. None of the 15 have upstream tapes — all need new `.tape` files.

---

## Part 3 — tui-base (the bubbles we built)

[tui-base](tui-base/) is the app framework: it wraps the stock bubbles in a
router/page architecture and adds components Charm doesn't ship. **No GIFs
exist anywhere in the repo.** [tools/demo.tape](tui-base/tools/demo.tape)
records a whole-app tour (`dist/tui-base-demo.gif`) but the output isn't
committed and the README embeds nothing.

### Custom components (no stock-bubble equivalent)

| Component | What it is | GIF | Priority |
|---|---|---|---|
| [datepicker](tui-base/datepicker/) | Calendar month/year/day picker (forked bubble-datepicker) | ❌ | **High** |
| [timepicker](tui-base/timepicker/) | Duration/time field picker | ❌ | **High** |
| [navigation](tui-base/navigation/) | Three switchable nav chromes: tabs, sidebar, topnav | ❌ | **High** |
| [status](tui-base/status/) | Status bar + notification history panel + info modal | ❌ | **High** |
| [notifications](tui-base/notifications/) | Toast/notification queue feeding the status bar | ❌ | Med (shown inside the status GIF) |
| [overlay](tui-base/overlay/) + [router](tui-base/router/) overlay stack | Modal overlay compositing | ❌ | Med |
| [pages/inspector](tui-base/pages/inspector/) | Ctrl+D live debug/inspector overlay | ❌ | **High** — unique selling point |
| [pages/settings](tui-base/pages/settings/) | Settings page: theme picker, key recorder, file/dir/multi-file pickers | ❌ | **High** |
| [keys](tui-base/keys/) | `AppKeyMap`: central bindings, runtime-customizable, feeds help | ➖ | shown via settings key-recorder GIF |
| [theme](tui-base/theme/) | Theme presets + styles for bubbles/huh | ❌ | Med — theme-cycling makes a good GIF |
| [table](tui-base/table/) | Stock table wrapped with `help.KeyMap` + sizing contract | ❌ | Low (dash tabs show tables in context) |
| [gate](tui-base/gate/), [geom](tui-base/geom/), [envpath](tui-base/envpath/), [logging](tui-base/logging/), [config](tui-base/config/) | Non-visual plumbing | ➖ | — |

### tui-base GIF checklist

| Recording | Source | Status |
|---|---|---|
| Whole-app tour (nav, settings, inspector, quit) | `tools/demo.tape` exists — run it, commit `dist/tui-base-demo.gif`, embed in README | ❌ tape ✅ / gif ❌ |
| [examples/minimal](tui-base/examples/minimal/) | New tape | ❌ |
| [examples/multipage](tui-base/examples/multipage/) | New tape | ❌ |
| [examples/dashboard](tui-base/examples/dashboard/) | New tape | ❌ |
| Per-component close-ups (datepicker, timepicker, nav modes, status bar, inspector, settings pickers) | New tapes — or crop these from the dash tab GIFs below to avoid duplicate tape maintenance | ❌ |

---

## Part 4 — dash (coverage tracker)

[dash](dash/) is the flagship consumer: its `examples/` package is a set of
router pages ("tabs") that demo every bubble **with the boilerplate
deduplicated into tui-base**. Goal from the roadmap: let a user pull any
bubble into a dashboard at runtime, using the bubbletea examples as the
implementation reference — minus their copy-pasted fluff.

### The dedup principle

Every bubbletea example re-implements the same scaffolding. In dash, that
fluff is provided once:

| Fluff in bubbletea examples | Replaced by |
|---|---|
| `case "q", "ctrl+c": return m, tea.Quit` and hardcoded letters in every `Update` | [tui-base/keys](tui-base/keys/keys.go) `AppKeyMap` — `key.Matches(msg, keys.Quit)`, runtime-rebindable via Settings (`ApplyCustomizations`), auto-feeds help |
| Hand-rolled help footer / `help.New()` wiring | tui-base [status bar](tui-base/status/status_bar.go) renders ShortHelp/FullHelp from the same `AppKeyMap` |
| `tea.WindowSizeMsg` plumbing in every model | [page.Base](tui-base/page/page.go) `SetSize` contract propagated by the router |
| Alt-screen setup, mouse enabling, background colors | `pageView` helper in [dash/examples/scaffold.go](dash/examples/scaffold.go) + router program options |
| Tab/view switching state machines (`views`, `composable-views`, `tabs` examples) | [tui-base/navigation](tui-base/navigation/) + router page registry |
| Per-example lipgloss styles | [tui-base/theme](tui-base/theme/) — every dash string renders through the active theme |
| Manual mouse hit-testing (`clickable` example) | `zoneTracker` / zoned-join helpers in [scaffold.go](dash/examples/scaffold.go) + router mouse routing |
| Focus cycling between widgets (`split-editors`, `textinputs`) | `paneCycler` in scaffold.go (`[`/`]` or click) |

**Porting rule:** when lifting a bubbletea example into dash, keep only its
*component wiring* (Init cmd, Update messages, View render); everything else
above already exists. Never compare `msg.String()` against a hardcoded letter
— bind through `AppKeyMap` so it stays rebindable at runtime.

### Bubble coverage in dash tabs

Current tabs: Intro · Gallery · Data · Inputs · Status · Pickers · Monitor · Game.

| Bubble | In dash? | Where | Reference example(s) |
|---|---|---|---|
| spinner | ✅ | Status (preset browser = `spinners`), Gallery | spinner, spinners |
| progress | ✅ | Status, Inputs (char budget), Gallery gauges | progress-static/animated |
| table | ✅ | Data, Monitor (proc table), Gallery | table |
| textinput | ✅ | Inputs (echo modes, validation, suggestions), Data (filter) | textinput, textinputs, autocomplete |
| textarea | ✅ | Inputs (resizable via Width field) | textarea, dynamic-textarea |
| list | ✅ | Data, Status (spinner presets), Gallery (procs) | list-simple/default |
| paginator | ✅ | Data (table pager), Gallery | paginator |
| viewport | ✅ | Every tab (page scroll), Intro | pager |
| filepicker | ✅ | Pickers (with preview pane) | file-picker |
| timer | ✅ | Status (countdown), Pickers (armed by timepicker) | timer |
| stopwatch | ✅ | Status, Gallery | stopwatch |
| help + key | ✅ | Everywhere via tui-base keys/status bar; Status tab shows it explicitly | help |
| cursor | ➖ | Inside textinput/textarea | — |
| tui-base datepicker | ✅ | Pickers, Gallery | — |
| tui-base timepicker | ✅ | Pickers, Gallery | — |

**Every stock bubble is already represented.** What's left is *patterns*, not
bubbles:

### Patterns left to include

| Status | Pattern | Reference example | Notes |
|---|---|---|---|
| ⬜ | Async IO driving a widget | http, progress-download | Fetch something real, feed a progress bar/table; Monitor's tick loop covers `realtime` but not request/response |
| ⬜ | Markdown rendering | glamour | Natural fit for Intro tab or a docs viewer widget |
| ⬜ | Debounced input | debounce | Data tab's filter fires per keystroke — debouncing it is a one-file demo |
| ⬜ | Custom list delegate | list-fancy | Data/Status use default delegates; one styled delegate would complete the list story |
| ⬜ | Static lipgloss table that reflows | table-resize | Roadmap W-1 (lipgloss/v2/table widget) |
| ⬜ | huh form flow | isbn-form | Roadmap CH-1 (settings-form-driven chat); creator/form.go is a start |
| ⬜ | Chat / external messages | chat, send-msg | Roadmap CH-1 multicast chat page |
| ⬜ | Editor handoff | exec | "Edit widget config in $EDITOR" would be a genuinely useful demo |
| ⬜ | Unsaved-changes guard | prevent-quit | Router-level `tea.WithFilter` when the creator has unsaved edits |
| ⬜ | Key-release / enhanced keyboard | keyboard-enhancements | Surface in settings key-recorder, which already captures keys |
| ➖ | App chrome (tabs/views/fullscreen/altscreen/window-size/focus) | tabs, views, composable-views, fullscreen, altscreen-toggle, window-size, focus-blur | Handled by tui-base router/navigation — nothing to port |
| ➖ | Mouse basics | mouse, clickable | Handled by scaffold zoneTracker + router routing |
| ➖ | Canvas/animation | canvas, cellbuffer, doom-fire, eyes, space, splash | Game tab is dash's canvas demo; port more only as eye candy |
| ➖ | Shell interop | pipe, tui-daemon-combo, package-manager | App-level patterns that conflict with a router app (tea.Println needs inline mode); revisit if dash grows a headless mode |
| ➖ | Terminal diagnostics | capability, query-term, colorprofile, cursor-style, set-* | tui-base inspector/theme territory, not dashboard widgets |

### dash GIF checklist

No GIFs or tapes exist in the repo. Each tab is a self-contained demo — one
tape per tab, embedded in the README:

| Recording | Shows | Status | Priority |
|---|---|---|---|
| Intro tab | Guided tour, themed rendering, live widgets | ❌ | **High** |
| Gallery tab | Widgets, gauges, pie/sankey, pickers, procs | ❌ | **High** |
| Data tab | Filter → list/table/pager cross-wiring | ❌ | **High** |
| Inputs tab | textinput variants, autocomplete, dynamic textarea | ❌ | **High** |
| Status tab | Spinner presets, progress, stopwatch, countdown, notifications | ❌ | **High** |
| Pickers tab | Calendar, duration→timer, filepicker+preview | ❌ | **High** |
| Monitor tab | Live CPU/mem/GPU/net/procs | ❌ | Med (timing-dependent, less deterministic) |
| Game tab | Shape-network water sim | ❌ | Med — best eye-candy, hardest to script |
| Theme cycle | Same tab under 3–4 theme presets | ❌ | Med |

---

## Part 5 — When to use which

### The stack, top to bottom

- **dash**: you want a dashboard, or a worked example of assembling an app.
- **tui-base**: you're building your own multi-page app — get router,
  navigation, theming, settings, notifications, inspector for free and write
  only pages.
- **bubbles**: you need one widget inside any bubbletea program.
- **bubbletea examples**: reference code for a wiring pattern — copy the
  pattern, not the boilerplate (see the dedup table in Part 4).

### Bubbles component vs. bubbletea example

A **bubble** is a reusable `Model` you import and embed. An **example** is
reference code you read. When both exist for a concept (spinner, table,
paginator, timer…), the example is a thin demo *of* the bubble. Rule of
thumb: **need the widget → import the bubble; need the wiring → read the
example; building a whole app → start from tui-base and skip both kinds of
boilerplate.**

### Stock bubble vs. tui-base version

| Need | Stock choice | tui-base choice | Pick tui-base when… |
|---|---|---|---|
| Keybindings | bubbles/key, hand-wired | [keys.AppKeyMap](tui-base/keys/keys.go) | keys must be user-rebindable at runtime and appear in help automatically |
| Help display | bubbles/help footer | [status bar](tui-base/status/) | you also want notifications, history, and info modals in the same chrome |
| Tabs/pages | `tabs` example pattern | [navigation](tui-base/navigation/) + router | more than one page, or you want tabs/sidebar/topnav switchable |
| Table | bubbles/table | [tui-base/table](tui-base/table/table.go) wrapper | you're inside the tui-base Component contract (SetSize + help.KeyMap) |
| File picking | bubbles/filepicker | [settings pickers](tui-base/pages/settings/) (file/dir/multi) | picking feeds persisted settings; raw filepicker for one-off picks (see dash Pickers tab) |
| Dates/times | — (none exist) | [datepicker](tui-base/datepicker/) / [timepicker](tui-base/timepicker/) | always — there is no stock equivalent |
| Modals | hand-rolled overlay | [overlay](tui-base/overlay/) + router stack | anything beyond a single hardcoded modal |
| Styling | inline lipgloss | [theme](tui-base/theme/) | users can change themes; also themes huh forms and bubbles |

### Within the stock bubbles

**Lists & paging** — `paginator`: you render your own items, need only page
logic. `list`: full browser (filtering, status, pagination) — heavier,
opinionated. `viewport`: not a list; scrolls pre-rendered text (docs, logs).
Examples ladder: `list-simple` → `list-default` → `list-fancy`.

**Text entry** — `textinput`: single line. `textarea`: multi-line;
`dynamic-textarea` = auto-grow, `split-editors` = several at once,
`autocomplete` = suggestions overlay. `cursor`: only when building your own
input widget.

**Tables** — bubbles `table`: interactive row selection. lipgloss table
(`table-resize` example): static rendering that reflows — use when nothing
needs selecting.

**Progress & activity** — `spinner`: unknown duration. `progress`: known
fraction; static vs animated vs hand-rolled (`progress-bar`) vs IO-driven
(`progress-download`).

**Time** — `timer` counts down, `stopwatch` counts up.

**Async patterns** — `http`/`progress-download`: one-shot IO commands.
`debounce`: coalesce bursty input. `realtime`/`send-msg`: goroutines pushing
messages in. `sequence`: ordered commands. `exec`: hand terminal to another
program. `pipe`: stdin/stdout interop. `tui-daemon-combo`: TUI + headless in
one binary. `prevent-quit`: `tea.WithFilter` interception.

**Rendering & capabilities** — `cellbuffer`/`canvas`: cell-level drawing
mechanics; `doom-fire`/`eyes`/`space`/`splash`: pure showcase;
`colorprofile`/`capability`/`query-term`/`set-terminal-color`: what the
terminal supports.

---

## Part 6 — Generating the missing GIFs

All recordings should be [VHS](https://github.com/charmbracelet/vhs) tapes so
they're reproducible and reviewable:

1. **Reuse upstream tapes** for the 36 bubbletea examples that have them —
   copy from [vhs/examples/bubbletea](https://github.com/charmbracelet/vhs/tree/main/examples/bubbletea)
   into each example dir (rename `credit-card-form.tape` → `isbn-form.tape`),
   so the fork can regenerate its own GIFs.
2. **Write new tapes** for the 15 recordable GIF-less examples (no upstream
   tape exists). Match existing conventions: output `<name>.gif` beside
   `main.go`, ~750px wide.
3. **bubbles**: add a `tapes/` dir driving the corresponding bubbletea
   examples (that's what upstream does), commit the GIFs, repoint the README
   away from `stuff.charm.sh`.
4. **tui-base**: run the existing `tools/demo.tape`, commit the GIF, embed in
   README; add per-example tapes under `examples/`.
5. **dash**: one tape per tab (launch `go run .`, navigate to the tab via
   tui-base keys — `tab` advances pages, so tapes stay valid as tabs are
   added). Crop tui-base component close-ups from these instead of
   maintaining duplicate tapes.
