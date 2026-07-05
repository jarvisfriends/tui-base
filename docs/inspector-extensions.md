# Extending the Inspector (developers only)

The Ctrl+D inspector accepts custom tabs through
`router.RegisterInspectorTab` / `inspector.MetricsProvider`. It is a
**developer diagnostics surface**: use it for app-internal metrics, queue
depths, cache hit rates — anything you want visible while debugging a running
app. User-facing screens belong in pages, not inspector tabs.

## The contract

```go
type MetricsProvider interface {
    TabName() string                          // short label + registry key
    BuildRows(c *theme.AppStyle) []string     // styled content lines
    RefreshInterval() time.Duration           // <= 0: no forced refresh
    Start()                                   // collection on  (idempotent)
    Stop()                                    // collection off (idempotent)
}
```

- **Start/Stop**: providers run only while the inspector is on screen. `Start`
  fires when the inspector opens (or when you register on an open inspector);
  `Stop` fires when it closes or the tab is removed/replaced. Both must be
  safe to call repeatedly.
- **BuildRows** runs on the UI goroutine on every render of your tab — only
  format state you have already collected; never block.
- **RefreshInterval** forces a re-render of your tab at that cadence (checked
  on the inspector's stats tick, so effective granularity is the runtime tick
  interval). Return 0 if your rows only change when other messages redraw the
  inspector anyway.

## Example

```go
type queueDepthTab struct {
    stop chan struct{}
    depth atomic.Int64
}

func (q *queueDepthTab) TabName() string                { return "Queue" }
func (q *queueDepthTab) RefreshInterval() time.Duration { return time.Second }

func (q *queueDepthTab) BuildRows(c *theme.AppStyle) []string {
    return []string{
        c.Styles.Subtitle.Bold(true).Render("Work Queue"),
        c.Styles.Item.Render("Depth  ") + c.Styles.TextOnBg.Render(strconv.FormatInt(q.depth.Load(), 10)),
    }
}

func (q *queueDepthTab) Start() {
    if q.stop != nil {
        return
    }
    q.stop = make(chan struct{})
    go q.poll(q.stop)
}

func (q *queueDepthTab) Stop() {
    if q.stop != nil {
        close(q.stop)
        q.stop = nil
    }
}
```

Register it once after constructing the router:

```go
m := router.NewWithOptions(opts)
m.RegisterInspectorTab(&queueDepthTab{})
```

The tab appears after the built-ins, participates in every tab-switching
affordance (number keys, ←/→, tab/shift+tab, horizontal wheel, clicks), and
scrolls in the section viewport when its rows overflow.
