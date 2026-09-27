package main

// themeCredits are the MIT-licensed palettes behind the dark themes in
// frontend/src/theme/themes.ts; their notices ship alongside the
// dependencies'.
var themeCredits = []Dependency{
	{Name: "Gruvbox", Version: "morhetz/gruvbox", Licence: "MIT", Text: mitNotice("Copyright (c) morhetz")},
	{Name: "Catppuccin", Version: "catppuccin/palette", Licence: "MIT", Text: mitNotice("Copyright (c) 2021 Catppuccin")},
	{Name: "Nord", Version: "nordtheme/nord", Licence: "MIT", Text: mitNotice("Copyright (c) 2016-present Sven Greb")},
	{Name: "Dracula", Version: "dracula/dracula-theme", Licence: "MIT", Text: mitNotice("Copyright (c) 2023 Dracula Theme")},
}

// mitNotice is the standard MIT licence text under copyright.
func mitNotice(copyright string) string {
	return copyright + "\n\n" + mitLicenceBody
}

const mitLicenceBody = `Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.`
