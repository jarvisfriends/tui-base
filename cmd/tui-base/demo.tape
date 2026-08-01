# Publish after rendering: docker run --rm -v "$PWD:/vhs" ghcr.io/charmbracelet/vhs publish <gif>; put the URL in README (gifs are not committed).
# VHS tour of the tui-base reference app: home-page component gallery,
# table interactions, settings, inspector, and page navigation.
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

# Home-page showcase shortcuts:
# t=theme, o=segmented pill shape, p=progress bar style,
# r=sparkline glyph style, f=effects tier.
Type "o"
Sleep 800ms
Type "o"
Sleep 800ms
Type "p"
Sleep 800ms
Type "p"
Sleep 800ms
Type "r"
Sleep 800ms
Type "t"
Sleep 900ms
Type "t"
Sleep 900ms
Type "f"
Sleep 900ms

# Show scrolling and table interactions.
PageDown
Sleep 1.2s
PageDown
Sleep 1.2s
Type "s"
Sleep 900ms
Down
Sleep 500ms
Down
Sleep 500ms
Up
Sleep 700ms
Type "/"
Sleep 600ms
Type "c"
Sleep 700ms
Escape
Sleep 900ms
PageUp
Sleep 1.2s

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

# Open Settings and browse a few rows.
Ctrl+G
Sleep 1.5s
Down
Sleep 800ms
Down
Sleep 800ms
Escape
Sleep 1s

# Cycle pages with the app navigation keys.
Tab
Sleep 1.2s
Tab
Sleep 1.2s
Shift+Tab
Sleep 1.2s

# Quit.
Type "q"
Sleep 1s
