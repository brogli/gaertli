# ADR-5: Log and graph are built in Go, the UI renders a window

- Go streams `git log`, builds commit records and graph rows, keeps them in memory (100k × a few hundred bytes fits QA-PERF-07).
- The UI's virtual list asks Go for rows `[from, to)`. Each row carries its lane data. The UI draws one small SVG/canvas strip per row.

