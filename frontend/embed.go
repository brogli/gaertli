// Package frontend embeds the built Vite output for the shell to serve.
package frontend

import "embed"

// Dist is the production build written by `vite build` to frontend/dist.
//
//go:embed all:dist
var Dist embed.FS
