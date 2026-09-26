import type { PrimeVueLocaleOptions } from 'openvue/config'
import type { Translate } from '@/i18n'

// OpenVue fills its own "{0}" into a count message.
const COUNT = { count: '{0}' }

// openvueLocale is OpenVue's locale option from the app's catalogue: the
// texts its components show themselves (a close button's label, a list's
// empty message), for the components the app uses. OpenVue merges it into
// its English defaults.
export function openvueLocale(t: Translate): Partial<PrimeVueLocaleOptions> {
  return {
    accept: t('openvue.accept'),
    reject: t('openvue.reject'),
    emptyFilterMessage: t('openvue.emptyFilterMessage'),
    emptyMessage: t('openvue.emptyMessage'),
    emptySearchMessage: t('openvue.emptySearchMessage'),
    emptySelectionMessage: t('openvue.emptySelectionMessage'),
    searchMessage: t('openvue.searchMessage', COUNT),
    selectionMessage: t('openvue.selectionMessage', COUNT),
    aria: {
      close: t('openvue.close'),
      listLabel: t('openvue.listLabel'),
      selectAll: t('openvue.allSelected'),
      unselectAll: t('openvue.noneSelected'),
    },
  }
}
