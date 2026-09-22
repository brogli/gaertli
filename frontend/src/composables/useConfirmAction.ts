import { useConfirm } from 'openvue/useconfirm'

// useConfirmAction is QA-SAFE-01/FR-UX-03's one shared confirmation rule,
// first used by ResetDialog's hard reset (spec 41), reused unchanged by
// specs 71 (force push), 72 (discard), 91/92 (remove worktree, delete
// unmerged branch): one Confirm/Cancel button pair, one danger-styled
// confirm button. Callers supply only their own header and message text.
export function useConfirmAction() {
  const confirm = useConfirm()

  function confirmAction(header: string, message: string, onAccept: () => void): void {
    confirm.require({
      header,
      message,
      acceptLabel: 'Confirm',
      rejectLabel: 'Cancel',
      acceptProps: { severity: 'danger' },
      accept: onAccept,
    })
  }

  return { confirmAction }
}
