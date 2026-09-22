import { useConfirm } from 'openvue/useconfirm'

// useConfirmAction is QA-SAFE-01/FR-UX-03's one shared confirmation rule
// for every hard action: one Confirm/Cancel pair, a danger-styled confirm
// button. Callers supply only their own header and message text.
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
