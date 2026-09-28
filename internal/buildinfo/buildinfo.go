// Package buildinfo holds values stamped at release build time via -ldflags -X.
package buildinfo

// App is the application version; "dev" outside release builds.
var App = "dev"

// Commit is the short commit hash; empty outside release builds.
var Commit = ""
