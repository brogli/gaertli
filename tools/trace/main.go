package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var cfg Config
	flag.StringVar(&cfg.FunctionalPath, "functional", "docs/requirements/functional.md", "functional requirements")
	flag.StringVar(&cfg.QualityPath, "quality", "docs/requirements/quality.md", "quality requirements")
	flag.StringVar(&cfg.TestingPath, "testing", "docs/architecture/testing.md", "manual checklist")
	flag.StringVar(&cfg.GoRoot, "go", ".", "root of the Go test walk")
	flag.StringVar(&cfg.FrontendRoot, "frontend", "frontend", "root of the Vitest and Playwright test walk")
	flag.StringVar(&cfg.PendingPath, "pending", "tools/trace/pending.txt", "allow-list of IDs whose tests are still owed")
	flag.Parse()

	code, err := run(cfg, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "trace:", err)
		os.Exit(2)
	}
	os.Exit(code)
}
