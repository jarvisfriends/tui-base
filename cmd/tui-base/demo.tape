# VHS tour of the tui-base reference app (X-5): pages, settings, inspector.
# Regenerate from the repo root:
#   go -C tools/rendertapes run .
# (rendertapes cross-compiles the linux demo binary and runs this tape in the
# official vhs container; gifs are build artifacts, not diffable sources)
Set WindowBar Colorful
Output cmd/tui-base/demo.gif
Set FontSize 20
Set Width 1600
Set Height 1000
Set TypingSpeed 60ms

Type "./cmd/tui-base/demo-bin"
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
