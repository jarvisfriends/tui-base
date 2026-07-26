// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package common

// appVersion is injected at build time via -ldflags:
//
//	go build -ldflags="-X github.com/jarvisfriends/tui-base/common.appVersion=v1.0.0" .
//
// When no version is injected (e.g. `go run .` or a plain `go build`),
// the value stays "Dev Build".
var appVersion = "Dev Build"

// AppVersion returns the application version string. Release builds set this
// to the latest git tag (e.g. "v1.2.3"); all other builds return "Dev Build".
func AppVersion() string { return appVersion }
