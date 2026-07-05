# VHS demo tape for the tui-base reference app (X-5).
#
# Render with:  vhs tools/demo.tape   (https://github.com/charmbracelet/vhs)
# Produces dist/tui-base-demo.gif showing navigation, theming, notifications,
# and the Ctrl+D inspector.

Output dist/tui-base-demo.gif

Set FontSize 15
Set Width 1200
Set Height 720
Set Theme "Dracula"
Set TypingSpeed 60ms

Type "go run ./cmd/tui-base"
Enter
Sleep 3s

# Cycle pages with the app navigation keys.
Tab
Sleep 1.5s
Tab
Sleep 1.5s

# Open Settings (default binding) and browse a few rows.
Ctrl+G
Sleep 1.5s
Down
Down
Sleep 1s
Escape
Sleep 1s

# The Ctrl+D inspector: cycle a few tabs, then close.
Ctrl+D
Sleep 2s
Right
Sleep 1.5s
Right
Sleep 1.5s
Type "4"
Sleep 2s
Escape
Sleep 1s

# Quit.
Type "q"
Sleep 1s
