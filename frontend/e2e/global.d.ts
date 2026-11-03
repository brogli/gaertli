// window.__e2eLogRowsCalls is set at runtime by src/testing/e2e-mock-bindings.ts
// (vite.config.ts swaps it in under --mode e2e), which declares the same
// augmentation for the app TS project. tsconfig.node.json type-checks
// e2e/**/*.ts as a separate project that never sees src/, so it needs its
// own copy of this ambient declaration.
interface Window {
  __e2eLogRowsCalls: Array<{ from: number; to: number }>
}
