package router

import (
	"context"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// ColorProfileEnvVar is the environment variable consumers can set to override
// the auto-detected terminal color profile.
//
// This is the primary remedy for washed-out or mis-quantized colors over SSH.
// SSH does not forward COLORTERM unless the client's SendEnv and the server's
// AcceptEnv are both configured, so a remote process running in a TrueColor
// terminal usually falls back to ANSI256 and downsamples 24-bit theme colors to
// the nearest 256-color palette entry (e.g. a dark slate background becomes a
// brighter, saturated blue).
//
// Accepted values (case-insensitive): truecolor, 24bit, ansi256, 256, ansi,
// 16, ascii, none, notty. Any other value is ignored (auto-detect is used).
const ColorProfileEnvVar = "TUI_BASE_COLOR_PROFILE"

// ForcedColorProfile returns the color profile requested via the
// TUI_BASE_COLOR_PROFILE environment variable and whether one was set.
//
// It is exported so pages (e.g. the inspector) can surface whether an override
// is active, and so consumers can apply the same logic to their own programs.
func ForcedColorProfile() (colorprofile.Profile, bool) {
	return forcedColorProfileForEnvVar(ColorProfileEnvVar)
}

// forcedColorProfileForEnvVar is the internal implementation that accepts any
// env var name, allowing the router to use app-specific names.
func forcedColorProfileForEnvVar(envVar string) (colorprofile.Profile, bool) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(envVar))) {
	case "truecolor", "24bit":
		return colorprofile.TrueColor, true
	case "ansi256", "256":
		return colorprofile.ANSI256, true
	case "ansi", "16":
		return colorprofile.ANSI, true
	case "ascii", "none":
		return colorprofile.Ascii, true
	case "notty":
		return colorprofile.NoTTY, true
	}
	return 0, false
}

// EffectiveColorProfile returns the color profile the program actually renders
// with: the forced profile from TUI_BASE_COLOR_PROFILE if set, otherwise the
// auto-detected profile for stdout.
//
// This mirrors exactly what Bubble Tea's renderer uses to downsample SGR color
// sequences. Callers MUST convert any color they emit OUTSIDE of SGR — most
// importantly tea.View.BackgroundColor / ForegroundColor, which Bubble Tea sends
// as an OSC sequence that the color-profile writer passes through unchanged.
//
// Without this conversion, over a downsampling profile (e.g. ANSI256 on SSH)
// the OSC-set terminal background stays exact 24-bit while every lipgloss
// Background() cell is quantized to the nearest palette entry — producing two
// visibly different shades of the "same" color on screen.
func EffectiveColorProfile() colorprofile.Profile {
	if p, ok := ForcedColorProfile(); ok {
		return p
	}
	return colorprofile.Detect(os.Stdout, os.Environ())
}

// effectiveColorProfileForEnvVar is like EffectiveColorProfile but reads from
// the provided env var name instead of the default ColorProfileEnvVar constant.
// Used internally by the router to honor app-specific color profile overrides.
func effectiveColorProfileForEnvVar(envVar string) colorprofile.Profile {
	if p, ok := forcedColorProfileForEnvVar(envVar); ok {
		return p
	}
	return colorprofile.Detect(os.Stdout, os.Environ())
}

// NewProgram constructs a tea.Program for a tui-base model with the framework's
// standard options applied. Today that means honoring TUI_BASE_COLOR_PROFILE so
// users can force a specific color profile (most commonly TrueColor over SSH,
// where COLORTERM is typically stripped and 24-bit theme colors get quantized
// to a washed-out ANSI256 approximation).
//
// Additional tea.ProgramOption values are appended after the defaults, so
// callers can still customize or override behavior. Every app built on tui-base
// should construct its program through this helper for consistent color
// handling across local, WSL, and SSH sessions.
//
// It reads the default TUI_BASE_COLOR_PROFILE variable; apps with a branded
// app-specific env var should use [NewProgramWithEnvVar] with
// m.ColorProfileEnvVar() instead so the program and router agree on the name.
func NewProgram(m tea.Model, opts ...tea.ProgramOption) *tea.Program {
	return NewProgramWithEnvVar(m, ColorProfileEnvVar, opts...)
}

// NewProgramWithEnvVar constructs a tea.Program that honors the given
// colorProfileEnvVar for color-profile overrides. Call this instead of
// NewProgram when using a RouterModel so the program and router agree on which
// environment variable controls the color profile:
//
//	m := router.NewWithOptions(router.Options{AppName: "My App"})
//	p := router.NewProgramWithEnvVar(m, m.ColorProfileEnvVar())
func NewProgramWithEnvVar(
	m tea.Model,
	colorProfileEnvVar string,
	opts ...tea.ProgramOption,
) *tea.Program {
	var base []tea.ProgramOption
	if profile, ok := forcedColorProfileForEnvVar(colorProfileEnvVar); ok {
		base = append(base, tea.WithColorProfile(profile))
	}
	base = append(base, opts...)
	return tea.NewProgram(m, base...)
}

// NewProgramWithContext is NewProgramWithEnvVar bound to a context: when ctx
// is canceled (e.g. by signal.NotifyContext on SIGINT/SIGTERM) the program
// shuts down cleanly and restores the terminal, so consumers embedding
// tui-base in services get graceful-shutdown behavior without wiring
// tea.WithContext themselves:
//
//	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
//	defer stop()
//	p := router.NewProgramWithContext(ctx, m, m.ColorProfileEnvVar())
func NewProgramWithContext(
	ctx context.Context,
	m tea.Model,
	colorProfileEnvVar string,
	opts ...tea.ProgramOption,
) *tea.Program {
	withCtx := append([]tea.ProgramOption{tea.WithContext(ctx)}, opts...)
	return NewProgramWithEnvVar(m, colorProfileEnvVar, withCtx...)
}
