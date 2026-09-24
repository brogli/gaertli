package main

import _ "embed"

//go:embed adapted/lazygit.LICENSE
var lazygitLicence string

// adaptedCode is code in this repo adapted from other projects. Its notices
// ship alongside the dependencies'.
var adaptedCode = []Dependency{{
	Name:    "github.com/jesseduffield/lazygit",
	Version: "(adapted: commit-graph lanes in internal/graph, the partial-patch builder in internal/services)",
	Licence: "MIT",
	Text:    lazygitLicence,
}}

//go:embed adapted/vue-devtools-api-6.LICENSE
var vueDevtoolsAPI6Licence string

// noLicenceFileText names packages whose published files carry no licence
// file at all, so loadTexts falls back to this embedded text instead of
// failing. @vue/devtools-api's 6.x line (a vue-i18n dependency) ships only
// package.json and dist/; its declared licence, author and repository
// fields match vuejs/vue-devtools, whose LICENSE is copied here verbatim.
var noLicenceFileText = map[string]string{
	"@vue/devtools-api": vueDevtoolsAPI6Licence,
}
