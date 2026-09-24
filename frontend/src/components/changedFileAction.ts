// ChangedFileAction is a changed-file row action. It names both the row
// button's icon and its changesView.* label, so a caller never passes
// display text and a label can never disagree with its icon.
export type ChangedFileAction = 'stage' | 'unstage' | 'discard' | 'markResolved'
