import { useConfirm } from 'openvue/useconfirm'

// useConfirmAction is the one confirmation for every hard action: a
// Confirm/Cancel pair with a danger-styled confirm button. Callers supply
// the header and message.
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
