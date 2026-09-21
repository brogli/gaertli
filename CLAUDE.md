# nebegit

Desktop git GUI: Go core + Wails v3 shell + Vue 3. Developed spec-driven, test-driven, hands-off by Claude sessions.

Read in this order: `docs/requirements.md` (what; index of `docs/requirements/`), `docs/architecture.md` (how; index of `docs/architecture/`), then `HANDOVER.md` in the plans dir (session protocol, roadmap, specs; path in `CLAUDE.local.md`). Load only the topic files a task needs.

Rules:
- No feature code without an approved spec in the plans dir. Specs are self-approved after the template checklist; critical decisions (listed in HANDOVER.md) go to the user.
- TDD: failing test first. Test names carry requirement IDs: Go `TestFR_LOG_01_...`, Vitest `it('FR-LOG-01 ...')`.
- `docs/` holds only requirements and architecture. Keep them current. Everything throwaway (specs, research notes, task lists, logs) lives in the plans dir.
- Frontend tests: Vitest component tests first; Playwright only where a component test cannot cover it, and the spec says why.
