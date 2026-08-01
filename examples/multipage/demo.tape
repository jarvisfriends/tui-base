# Publish after rendering: docker run --rm -v "$PWD:/vhs" ghcr.io/charmbracelet/vhs publish <gif>; put the URL in README (gifs are not committed).
# VHS demo of snap/navigation hosted by the multipage example (the snap
# ROADMAP wants the navigation demo in a router+pages host, not a synthetic
# example): Tab page cycling, sidebar focus with live arrow-key switching,
# and hiding/showing the nav chrome.
# Regenerate from the repo root:
#   go -C tools/rendertapes run .
Set WindowBar Colorful
Output examples/multipage/demo.gif
Set FontSize 20
Set Width 1600
Set Height 1000
Set TypingSpeed 60ms

Type "./examples/multipage/demo-bin"
Enter
Sleep 3s

# Tab / Shift+Tab cycle through Overview, Reports, Settings.
Tab
Sleep 1.5s
Tab
Sleep 1.5s
Shift+Tab
Sleep 1.5s

# Ctrl+B focuses the sidebar; Up/Down switch pages live while focus stays on
# the nav, Enter hands focus to the page.
Ctrl+B
Sleep 1s
Down
Sleep 1.2s
Down
Sleep 1.2s
Up
Sleep 1.2s
Enter
Sleep 1.5s

# Enter unfocused the sidebar, so Ctrl+B refocuses it, then hides the nav
# chrome entirely, then brings it back (unfocused).
Ctrl+B
Sleep 1s
Ctrl+B
Sleep 1.5s
Ctrl+B
Sleep 1.5s

# Quit.
Type "q"
Sleep 1s
