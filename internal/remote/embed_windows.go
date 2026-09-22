//go:build windows

package remote

import _ "embed"

// embeddedCore is the linux/amd64 nebegit-core binary this Windows build
// ships, so the app stays one download. Taskfile.yml's build:core:embed
// cross-compiles it into nebegit-core-linux here, since go:embed cannot
// reach outside its own package directory — run `task build:core:embed`
// before a bare `go build` on Windows.
//
//go:embed nebegit-core-linux
var embeddedCore []byte
