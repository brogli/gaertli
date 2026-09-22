//go:build windows

package remote

import _ "embed"

// embeddedCore is the linux/amd64 nebegit-core binary this Windows build
// ships (ADR-3, QA-INST-01: one download), cross-compiled into
// nebegit-core-linux by Taskfile.yml's build:core:embed before this
// package compiles (go:embed cannot reach outside its own package
// directory, Research used) — run `task build:core:embed` before a bare
// `go build` on Windows.
//
//go:embed nebegit-core-linux
var embeddedCore []byte
