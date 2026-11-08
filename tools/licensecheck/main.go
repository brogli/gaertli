package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const ownModule = "github.com/brogli/gaertli"

func main() {
	out := flag.String("out", "THIRD-PARTY-NOTICES.txt", "where to write the notices")
	flag.Parse()

	deps, err := collect()
	if err != nil {
		fmt.Fprintln(os.Stderr, "licensecheck:", err)
		os.Exit(2)
	}
	if violations := Check(deps); len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, "licensecheck: %s %s has licence %q, not in the allow-list\n", v.Name, v.Version, v.Licence)
		}
		os.Exit(1)
	}
	if err := os.WriteFile(*out, []byte(Notices(deps)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "licensecheck:", err)
		os.Exit(2)
	}
}

// collect lists the Go packages linked into both binaries, the frontend's
// production dependencies (the ones bundled into the app), and the code
// adapted from other projects.
func collect() ([]Dependency, error) {
	dir, err := os.MkdirTemp("", "licensecheck")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	tmpl := filepath.Join(dir, "report.tmpl")
	if err := os.WriteFile(tmpl, []byte(goReportTemplate), 0o600); err != nil {
		return nil, err
	}

	goReport, err := output("go", "tool", "go-licenses", "report", "--template", tmpl, "./cmd/gaertli", "./cmd/gaertli-core")
	if err != nil {
		return nil, err
	}
	goDeps, err := parseGoLicenses(goReport, ownModule)
	if err != nil {
		return nil, err
	}
	jsReport, err := output("pnpm", "--dir", "frontend", "licenses", "list", "--prod", "--json")
	if err != nil {
		return nil, err
	}
	jsDeps, err := parsePnpmLicenses(jsReport)
	if err != nil {
		return nil, err
	}
	deps, err := loadTexts(append(goDeps, jsDeps...))
	if err != nil {
		return nil, err
	}
	return append(append(deps, adaptedCode...), themeCredits...), nil
}

// output runs a command and returns its stdout; its stderr (go-licenses
// prints warnings there) is shown only when the command fails.
func output(name string, args ...string) ([]byte, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, fmt.Errorf("%s: %w\n%s", name, err, ee.Stderr)
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return out, nil
}
