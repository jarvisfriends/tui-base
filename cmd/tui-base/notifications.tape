# VHS demo of snap/notifications + snap/status hosted by the reference app
# (the snap ROADMAP wants their demo in a router+pages host, not a synthetic
# example): toasts of all three severities, TTL expiry, the status bar's
# notification count, full help, and hiding/showing the bar.
# Regenerate from the repo root:
#   go -C tools/rendertapes run .
Set WindowBar Colorful
Output cmd/tui-base/notifications.gif
Set FontSize 20
Set Width 1600
Set Height 1000
Set TypingSpeed 60ms

Type "./cmd/tui-base/demo-bin"
Enter
Sleep 3s

# The inspector's test keys fire real notifications through the router:
# i = info (5s TTL), w = warning (10s), e = error (15s).
Ctrl+D
Sleep 1.5s
Type "i"
Sleep 600ms
Type "w"
Sleep 600ms
Type "e"
Sleep 1s
Escape
Sleep 1s

# Three toasts stacked bottom-right; the status bar counts them. Watch the
# info toast expire first, then the warning.
Sleep 4s
Sleep 5s

# Ctrl+N opens the notification history panel (I-12); arrows move the
# cursor, d dismisses everything, Ctrl+N closes it again.
Ctrl+N
Sleep 2s
Down
Sleep 1s
Type "d"
Sleep 1.5s
Ctrl+N
Sleep 1s

# Status bar: Ctrl+H expands the full key help, Ctrl+J hides/shows the bar.
Ctrl+H
Sleep 2s
Ctrl+H
Sleep 1s
Ctrl+J
Sleep 1.5s
Ctrl+J
Sleep 1.5s

# Quit.
Type "q"
Sleep 1s
