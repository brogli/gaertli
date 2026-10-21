# Frontend

- **A standard Vue project.** The layout, tooling and idioms follow the official Vue scaffold, so
  the project reads like any Vue project and can follow the scaffold's changes (QA-MNT-05).
- **Typed throughout.** The compiler should catch what it can: no escape hatches from the type
  system in app code, and outside data stays untyped until checked.
- **One component library, used as it comes.** Every control is an OpenVue component, its look
  changed only through the theme's design tokens and its behaviour relied on, never worked around
  (QA-USE-04). The app's own styles lay out views and draw only what the library has no component
  for.
- **Utility styling.** Views style with Tailwind utilities that read the library's tokens, so the
  app and its components share one theme.
- **One icon set**, the library's own, for every icon; no glyphs, emoji or hand-drawn icons.
- **No text in code.** Every UI text, error messages included, comes from the message catalogue
  (FR-UX-11).
- **Repo content is text.** Messages and file contents are rendered as text, never as HTML
  (QA-SEC-04).
