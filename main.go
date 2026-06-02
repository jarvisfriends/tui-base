package main

import (
	"fmt"
	"os"

	"github.com/jarvisfriends/tui-base/router"
)

func main() {
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
