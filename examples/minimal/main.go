// Command minimal is the smallest possible tui-base app: the built-in Home
// and Settings pages, theming, notifications, and the Ctrl+D inspector — one
// import, one call.
//
//	go run ./examples/minimal
package main

import (
	"fmt"
	"os"

	tuibase "github.com/jarvisfriends/tui-base"
)

func main() {
	if err := tuibase.Run(tuibase.Options{AppName: "Minimal"}); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
