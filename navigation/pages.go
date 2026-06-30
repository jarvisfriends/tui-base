package navigation

const (
	// PageIDHome, PageIDInspector, PageIDSettings are the stable page ID strings
	// used when registering pages with any Navigator. Router and tests should use
	// these rather than bare string literals so renames stay in one place.
	PageIDHome      = "home"
	PageIDInspector = "debug"
	PageIDSettings  = "settings"

	pageIDHome      = PageIDHome
	pageIDInspector = PageIDInspector
	pageIDSettings  = PageIDSettings

	pageHome      = "Home"
	pageInspector = "Inspector"
	pageSettings  = "Settings"
)
