import { useConfirm } from 'openvue/useconfirm'
import { i18n } from '../i18n'

// useConfirmAction is the one confirmation for every hard action: a
// Confirm/Cancel pair with a danger-styled confirm button. Callers supply
// the header and message. i18n.global.t, not useI18n(): called by
// composables and tests outside a component's setup too.
export function useConfirmAction() {
  const confirm = useConfirm()

  function confirmAction(header: string, message: string, onAccept: () => void): void {
    confirm.require({
      header,
      message,
      acceptLabel: i18n.global.t('common.confirm'),
      rejectLabel: i18n.global.t('common.cancel'),
      acceptProps: { severity: 'danger' },
      accept: onAccept,
    })
  }

  return { confirmAction }
}
