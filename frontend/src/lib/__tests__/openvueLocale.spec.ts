import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import { openvueLocale } from '../openvueLocale'
import en from '@/locales/en.json'

// A catalogue unlike OpenVue's English defaults, so a text taken from it
// cannot be mistaken for OpenVue's own.
const openvue = {
  accept: 'Ja',
  reject: 'Nein',
  emptyFilterMessage: 'Keine Treffer',
  emptyMessage: 'Keine Auswahl',
  emptySearchMessage: 'Nichts gefunden',
  emptySelectionMessage: 'Nichts gewählt',
  searchMessage: '{count} Treffer',
  selectionMessage: '{count} gewählt',
  close: 'Schliessen',
  listLabel: 'Liste',
  allSelected: 'Alle gewählt',
  noneSelected: 'Keine gewählt',
}

describe('openvueLocale', () => {
  it("FR-UX-11 OpenVue's built-in texts come from the catalogue, its own {0} placeholder kept", () => {
    const other = createI18n({ legacy: false, locale: 'en', messages: { en: { ...en, openvue } } })

    const locale = openvueLocale(other.global.t)

    expect(locale).toEqual({
      accept: 'Ja',
      reject: 'Nein',
      emptyFilterMessage: 'Keine Treffer',
      emptyMessage: 'Keine Auswahl',
      emptySearchMessage: 'Nichts gefunden',
      emptySelectionMessage: 'Nichts gewählt',
      searchMessage: '{0} Treffer',
      selectionMessage: '{0} gewählt',
      aria: {
        close: 'Schliessen',
        listLabel: 'Liste',
        selectAll: 'Alle gewählt',
        unselectAll: 'Keine gewählt',
      },
    })
  })
})
