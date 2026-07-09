# File and Directory Pickers

How to embed huh/bubbles file pickers and tui-base's own `DirPicker` without
hitting their sizing traps.

## The collapsed-window trap (one visible row)

`charm.land/bubbles/filepicker` opens with a **collapsed one-row browse
window** (`maxIdx = 0`). Three things can expand it, and only one of them is
unconditional:

| Event | Effect on the window |
|---|---|
| `SetHeight(h)` (what `huh`'s `Height`/`WithHeight` call) | only *shrinks* the window — it never grows `maxIdx` |
| `readDirMsg` (a directory listing arriving) | grows the window to the height *at that moment* |
| `tea.WindowSizeMsg` | unconditionally resets the window to the current height |

On top of that, `huh` freezes each group's height from the fields' **first
render** — for a picker that's the collapsed pre-readDir list — and a zoomed
(picking) field gets that frozen height re-imposed on every render. The
combination produces the classic symptom: the overlay box fills the page but
the listing shows **one row at a time**, and it "heals itself" after the
user changes directory or resizes the terminal.

### How to stay out of that state

1. **Always dispatch the form's `Init()` command.** It carries the picker's
   `readDir`; swallowing it means no listing (or a stale window) until some
   later message fires it. `MultiFileEditor.startPicking` returns this
   command for exactly that reason.
2. **Send a synthetic `tea.WindowSizeMsg` right after `Init`,** sized to the
   space the form actually gets (`overlay.FormHeight`/`overlay.FormWidth`).
   This is what `tea.Program` would do on startup, and it's the only repair
   the picker honors unconditionally. `overlay.FormOverlayHost.Open` and
   `MultiFileEditor.startPicking` both do this — reuse them instead of
   hand-rolling a form host.
3. Set `.Picking(true)` and `.Height(...)` on the huh field at construction
   so the first render is already browse-mode; heights set later fight the
   group's frozen height.

If you host a picker form some other way, replicate steps 1–2; a picker that
"only shows one row until I scroll or go up a directory" means one of them
is missing.

## Directory-only picking: `DirPicker`

`pages/settings.DirPicker` is a purpose-built directory browser (no files
shown) with its own sizing (`Width`/`Height` fields — seed them from the
page before `Init`). Bindings: Enter/→ descends, ← ascends, Space selects
the highlighted folder, Ctrl+S selects the folder being browsed, Esc
cancels.

Pressing ← at a filesystem root switches to the **drive list** (`💾
Drives`), so the user can reach any mounted drive — navigation is never
limited to the directory the app started in.

## Choosing a config field type

| Need | Field constructor | Editor |
|---|---|---|
| One file (or file-or-dir) | `config.FilePickerField` / `FieldFilePicker` with `FileAllowed` | huh file picker |
| One directory | `FieldFilePicker` with `DirAllowed` only | `DirPicker` |
| Many files/paths | `config.MultiFilePickerField` | `MultiFileEditor` + huh picker per row |
| Many directories | `config.MultiDirPickerField` | `MultiFileEditor` + `DirPicker` per row |

Multi-path values are stored in the bound string as a semicolon-separated
list.
