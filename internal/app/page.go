package app

import "github.com/wailsapp/wails/v3/pkg/application"

// pageKey is the context key of the browser page a call comes from. The
// server-mode build (the real-backend e2e) serves the app to browser
// pages, which are no windows of the registry's; the registry learns from
// a page's calls which worktree it shows (Registry.showInPage).
type pageKey struct{}

// pageMiddleware puts the calling page into each request's context under
// pageKey. nil in the desktop build: every window there is the registry's.
var pageMiddleware application.Middleware
