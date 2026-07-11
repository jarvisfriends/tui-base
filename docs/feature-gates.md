# Feature gates

Named boolean flags (package [`gate`](https://github.com/jarvisfriends/snap) in the snap repo) that show or hide functionality
at **startup** (defaults + environment overrides) and at **runtime** (the
settings page), with changes visible immediately. Gate values are
deliberately **never persisted** to the settings file — every launch starts
from the registered defaults plus environment overrides, so a development
toggle can't silently bleed into a user's saved config.

## Lifecycle

1. **Register** gates before building the router and pass the registry in:

   ```go
   gates := gate.NewGateRegistry()
   gates.Register(gate.FeatureGate{
       Name:        "Experimental Dashboard",
       Default:     false,
       Description: "Unfinished dashboard page",
   })
   tuibase.Run(tuibase.Options{AppName: "My App", Gates: gates})
   ```

   The router always ensures a registry exists (creating one when the app
   passes none) and registers its own built-in gates — currently
   `inspector.AccessibilityTabGate` ("Inspector Accessibility Tab",
   default **off**), which controls the Accessibility tab in the Ctrl+D
   inspector.

2. **Startup overrides** come from the environment, applied by the router
   using the app name: `<APPNAME>_GATE_<GATENAME>` (uppercased,
   non-alphanumerics become underscores). For the reference app:

   ```
   TUI_BASE_GATE_INSPECTOR_ACCESSIBILITY_TAB=1
   ```

3. **Runtime toggling**: every registered gate appears in the settings page's
   **Feature Flags** section. Committing a change updates the shared registry
   and broadcasts `settings.GatesChangedMsg` — the router re-derives
   gate-dependent UI immediately (e.g. the inspector shows/hides its
   Accessibility tab on the spot, snapping to the Runtime tab if the hidden
   tab was active). No restart, no file write.

## Reacting to gate changes in an app

Pages hold the same `*gate.GateRegistry` pointer, so checking
`gates.Value("My Gate")` in `View`/render code picks up flips automatically on
the next frame. For structural changes (rebuilding a list of tabs or pages),
handle the broadcast:

```go
case settings.GatesChangedMsg:
    // msg.Values is a snapshot; the registry is already updated.
    m.rebuildGatedUI()
```

## Semantics

- `Value(name)` for an **unregistered** name returns `true` (absent gate =
  feature enabled). Register a gate with `Default: false` to hide something.
- `Register` panics on duplicate names; `Has(name)` lets framework code
  register built-ins only when the app has not already defined them.
- The registry is safe for concurrent reads; writes happen on the Bubble Tea
  update goroutine.
