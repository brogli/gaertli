import { afterEach } from 'vitest'
import { config, enableAutoUnmount } from '@vue/test-utils'
import PrimeVueConfig from 'openvue/config'
import ConfirmationService from 'openvue/confirmationservice'
import ToastService from 'openvue/toastservice'
import { i18n } from '@/i18n'
import { openvueLocale } from '@/lib/openvueLocale'
import { openvuePassThrough } from '@/lib/openvuePassThrough'

// Components with a window-level listener (onKeyStroke's shortcuts) would
// otherwise keep firing into later tests in the same file: every mount
// needs an unmount.
enableAutoUnmount(afterEach)

// OpenVue's form components read $primevue.config, which exists only once
// its config plugin is installed; theme: 'none' pulls in no visual theme,
// locale is the app's own.
// useConfirm() and useToast() throw without their services. The production
// i18n instance keeps every mount asserting English text; a test proving
// text comes from the catalogue installs its own instance on that mount.
config.global.plugins = [
  [PrimeVueConfig, { theme: 'none', locale: openvueLocale(i18n.global.t), pt: openvuePassThrough }],
  ConfirmationService,
  ToastService,
  i18n,
]

// @tanstack/vue-virtual measures its scroll container via offsetWidth/
// offsetHeight, which happy-dom always reports as 0: it has no layout
// engine, so every element gets a real size here; tests needing a
// different size override it themselves.
Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 800 })
Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, value: 600 })
