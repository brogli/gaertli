import { config } from '@vue/test-utils'
import PrimeVueConfig from 'openvue/config'
import ConfirmationService from 'openvue/confirmationservice'

// OpenVue's form components (InputText/AutoComplete/MultiSelect) read
// $primevue.config, which exists only once its config plugin is installed
// (main.ts does app.use); theme: 'none' pulls in no visual theme.
//
// ConfirmationService: useConfirm() throws without it, so it is installed
// globally rather than per test file.
config.global.plugins = [
  [PrimeVueConfig, { theme: 'none' }],
  ConfirmationService,
]

// @tanstack/vue-virtual measures its scroll container via offsetWidth/
// offsetHeight, which happy-dom always reports as 0: it has no layout
// engine. Stubbing a real size is the standard workaround; tests needing a
// different size override it themselves.
Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 800 })
Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, value: 600 })
