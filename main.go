// Command tui-base runs the reference application showcasing the framework:
// multi-page routing, theming, notifications, and the Ctrl+D inspector.
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"

	"github.com/jarvisfriends/tui-base/router"
)

const appName = "TUI Base"

// tabIconPNG is the icon shown on the Windows Terminal tab. It is generated
// from assets/icon.svg by tools/genicon (see gen.go) and embedded so the app
// can brand its tab without shipping a separate file.
//
//go:embed tabicon.png
var tabIconPNG []byte

func main() {
	install := flag.Bool(
		"install-terminal-profile",
		false,
		"register a Windows Terminal profile (branded name + icon in the new-tab dropdown), then exit",
	)
	uninstall := flag.Bool("uninstall-terminal-profile", false,
		"remove the Windows Terminal profile installed by -install-terminal-profile, then exit")
	flag.Parse()

	switch {
	case *install:
		path, err := router.InstallWindowsTerminalProfile(router.WindowsTerminalProfile{
			AppName:  appName,
			IconData: tabIconPNG,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "install failed: "+err.Error())
			os.Exit(1)
		}
		fmt.Println("Installed Windows Terminal profile: " + path)
		return
	case *uninstall:
		if err := router.UninstallWindowsTerminalProfile(appName); err != nil {
			fmt.Fprintln(os.Stderr, "uninstall failed: "+err.Error())
			os.Exit(1)
		}
		fmt.Println("Removed Windows Terminal profile for " + appName)
		return
	}

	// On the legacy Windows console, relaunch into Windows Terminal (which
	// supports the truecolor/mouse/styling features this stack renders with)
	// and let this process exit. No-op everywhere else. When the profile has
	// been installed (-install-terminal-profile), the tab opens under it and
	// shows the branded icon; otherwise it opens under the default profile.
	if relaunched, _ := router.MaybeRelaunchInWindowsTerminal(
		router.TerminalRelaunchConfig{AppName: appName, ProfileName: appName},
	); relaunched {
		return
	}

	m := router.New()
	// router.NewProgramWithEnvVar applies tui-base's standard program options
	// and honors the app-specific color-profile env var (TUI_BASE_COLOR_PROFILE
	// for the default "TUI Base" name) so colors stay consistent over SSH.
	p := router.NewProgramWithEnvVar(m, m.ColorProfileEnvVar())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program: " + err.Error())
		os.Exit(1)
	}
}
