import { shortSha } from '../lib/format'

// The types here have their own module: a type exported from a <script
// setup> block resolves to `any` under eslint's type-aware linting.

// StartPoint is where a new branch starts: a branch or tag by its full ref
// name, preselected in a list the user can pick another from, or a commit
// the caller fixed (from the log view, HEAD while detached), shown by its
// label. A full ref name keeps a branch and a tag of the same name apart,
// which a bare name as a start does not.
export type StartPoint = { ref: string } | { rev: string; label: string }

// WorktreeStart is a "New worktree from here…" prefill for
// CreateWorktreeDialog: an existing branch, or a new branch at a start point.
export type WorktreeStart =
  { newBranch: false; branch: string } | { newBranch: true; start: StartPoint }

// commitStart is the fixed start point at the commit sha.
export function commitStart(sha: string): StartPoint {
  return { rev: sha, label: shortSha(sha) }
}

// branchStart is the start point at local branch name. Without one (a
// detached HEAD) nothing is preselected, and git branches from HEAD.
export function branchStart(name: string | undefined): StartPoint {
  return { ref: name ? `refs/heads/${name}` : '' }
}

// startRev is the revision a start point sends to git.
export function startRev(start: StartPoint): string {
  return 'rev' in start ? start.rev : start.ref
}
