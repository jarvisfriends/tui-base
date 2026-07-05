// Package navigation defines the Navigator interface and the built-in
// navigators: Sidebar (left-docked list), Tabs (top-docked bordered tab bar
// with overflow paging), and MinimalTopNav (compact top row with optional
// number prefixes).
//
// Navigators report their dock edge via Dock so the router can lay them out
// without concrete type switches; optional capabilities (Focusable,
// NumberLabeled) are detected the same way. All navigators support keyboard
// (NextPage/PreviousPage/Select), mouse clicks, and horizontal wheel
// scrolling with equivalent semantics (ADR-005).
package navigation
