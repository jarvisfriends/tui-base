package main

import (
	"fmt"
)

// UTF-8 Powerline glyphs (Requires a Nerd Font or Powerline-patched font in your terminal)
const (
	TriRight       = "\ue0b0" // 
	TriLeft        = "\ue0b2" // 
	RoundRight     = "\ue0b4" // 
	RoundLeft      = "\ue0b6" // 
	SlantRight     = "\ue0bc" // 
	SlantLeft      = "\ue0ba" // 
	SlashBackRight = "\ue0b8" // 
	SlashBackLeft  = "\ue0be" // 
	FlameRight     = "\ue0c0" // 
	FlameLeft      = "\ue0c2" // 
	PixelsRight    = "\ue0c4" // 
	PixelsLeft     = "\ue0c6" // 
	HexagonRight   = "\ue0c8" // 
	HexagonLeft    = "\ue0ca" // 
)

// Standard ANSI color escape codes
const (
	Reset = "\033[0m"

	// Backgrounds
	BgBlack = "\033[40m"
	BgGreen = "\033[42m"
	BgBlue  = "\033[44m"

	// Foregrounds
	FgWhite = "\033[37m"
	FgGreen = "\033[32m"
	FgBlue  = "\033[34m"

	// Using 49 for background makes the outer caps blend into whatever
	// theme (dark/light) your specific terminal uses.
	BgDefault = "\033[49m"
	BgCyan    = "\033[46m" // Button background color

	FgCyan = "\033[36m" // Cap foreground (must match button background)
)

// DrawSegment prints a UI block and its trailing transition shape.
// bg, fg: Colors for the text block.
// nextBg: Background color of the upcoming segment (for the transition shape).
// shapeFg: Foreground color of the shape (usually matches 'bg').
func DrawSegment(text, bg, fg, nextBg, shapeFg, shape string) {
	// 1. Print the text block with its specific background and foreground
	fmt.Printf("%s%s %s ", bg, fg, text)

	// 2. Print the transition shape
	// The shape's foreground is the current block's background color.
	// The shape's background is the next block's background color.
	fmt.Printf("%s%s%s", nextBg, shapeFg, shape)
}

// ButtonStyle defines the left and right boundary characters for a button.
type ButtonStyle struct {
	Name  string
	Left  string
	Right string
}

// A collection of Nerd Font/Powerline transition glyphs
var styles = []ButtonStyle{
	{"Triangle (Hard)", TriLeft, TriRight},            //  / 
	{"Rounded (Soft)", RoundLeft, RoundRight},         //  / 
	{"Slant Forward", SlantLeft, SlantRight},          //  / 
	{"Slant Backward", SlashBackLeft, SlashBackRight}, //  / 
	{"Flame", FlameLeft, FlameRight},                  //  / 
	{"Pixels", PixelsLeft, PixelsRight},               //  / 
	{"Hexagon", HexagonLeft, HexagonRight},            //  / 
}

// DrawButton renders a standalone button using the 3-part anatomy.
func DrawButton(label string, style ButtonStyle) {
	// 1. Left Cap
	leftCap := fmt.Sprintf("%s%s%s", BgDefault, FgCyan, style.Left)

	// 2. Button Body (Padded with spaces for visual breathing room)
	body := fmt.Sprintf("%s%s %s ", BgCyan, FgWhite, label)

	// 3. Right Cap
	rightCap := fmt.Sprintf("%s%s%s", BgDefault, FgCyan, style.Right)

	// Print the style name, then the fully rendered button, then reset ANSI
	fmt.Printf("%-20s: %s%s%s%s\n\n", style.Name, leftCap, body, rightCap, Reset)
}

func main() {
	fmt.Println("--- Breadcrumb Trail (Triangles) ---")

	// Segment 1 (Blue), transitions into Segment 2 (Green)
	DrawSegment("HOME", BgBlue, FgWhite, BgGreen, FgBlue, TriRight)

	// Segment 2 (Green), transitions into Terminal Default (Black)
	DrawSegment("PLANTS", BgGreen, FgWhite, BgBlack, FgGreen, TriRight)

	fmt.Println(Reset, "\n")

	DrawSegment("HOME", BgBlue, FgWhite, BgGreen, FgBlue, RoundRight)
	DrawSegment("PLANTS", BgGreen, FgWhite, BgBlack, FgGreen, RoundRight)

	fmt.Println(Reset, "\n")

	DrawSegment("HOME", BgBlue, FgWhite, BgGreen, FgBlue, SlantRight)
	DrawSegment("PLANTS", BgGreen, FgWhite, BgBlack, FgGreen, SlantRight)

	fmt.Println("--- Standalone Button (Rounded) ---")

	// Print the left cap (Black background, Blue foreground)
	fmt.Printf("%s%s%s", BgBlack, FgBlue, RoundLeft)

	// Print the button body and right cap
	DrawSegment("ACTIVATE PUMP", BgBlue, FgWhite, BgBlack, FgBlue, RoundRight)

	fmt.Println(Reset, "\n")

	fmt.Println("--- UI Button Gallery ---\n")

	for _, style := range styles {
		DrawButton("SYSTEM STATUS", style)
	}
}
