import { createI18n, useI18n as useI18nBase } from 'vue-i18n'
import en from '@/locales/en.json'

type MessageSchema = typeof en

// Module augmentation, so a raw `t()` typed through Composer (as this
// file's own Translate, below, does) infers this schema.
declare module 'vue-i18n' {
  // eslint-disable-next-line @typescript-eslint/no-empty-object-type -- declaration merging needs this exact shape
  export interface DefineLocaleMessage extends MessageSchema {}
}

// Composition API needs legacy: false, not yet v11's default.
export const i18n = createI18n({
  legacy: false,
  locale: 'en',
  fallbackLocale: 'en',
  messages: { en },
})

// useI18n's own bare call picks its untyped overload, not the one reading
// DefineLocaleMessage; every component imports this wrapper instead. t()
// still accepts any string, so a misspelt key is no type error:
// src/__tests__/i18n.spec.ts catches it.
export function useI18n(): TypedComposer {
  return useI18nBase<[MessageSchema], 'en'>()
}

type TypedComposer = ReturnType<typeof useI18nBase<[MessageSchema], 'en'>>

// Translate is useI18n's t, for the lib/ and menus/ helpers that take it as
// an argument.
export type Translate = ReturnType<typeof useI18n>['t']
