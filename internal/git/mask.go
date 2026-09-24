package git

import "regexp"

// credentialInURL matches a URL's userinfo, wherever it appears in a
// command's arguments or in git's own output.
var credentialInURL = regexp.MustCompile(`://[^\s/@]+@`)

// MaskCredentials replaces every URL's userinfo in text with "***": a
// remote URL embedded in a git argument, or one echoed back in git's
// stderr.
func MaskCredentials(text string) string {
	return credentialInURL.ReplaceAllString(text, "://***@")
}
