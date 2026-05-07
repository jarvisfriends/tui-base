package main

import (
	"fmt"
	"os"

	"github.com/jarvisfriends/tui-base/router"
)

func main() {
	m := router.New()
	// router.NewProgram applies tui-base's standard program options (e.g.
	// honoring TUI_BASE_COLOR_PROFILE so colors stay consistent over SSH).
	p := router.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program: " + err.Error())
		os.Exit(1)
	}
}
