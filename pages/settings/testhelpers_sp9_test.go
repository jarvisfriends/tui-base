package settings

// selectItemForTest points the overview cursor at m.items[idx] regardless of
// collapse state: SP-9 collapses framework categories by default and parks
// the cursor on the first header, so tests that drive a specific item expand
// everything and clear the header cursor first.
func selectItemForTest(m *SettingsModel, idx int) {
	m.ExpandAllCategories()
	m.headerCursor = -1
	m.cursor = idx
}
