import { config } from '@vue/test-utils'
import PrimeVueConfig from 'openvue/config'
import ConfirmationService from 'openvue/confirmationservice'

// OpenVue's form components (InputText/AutoComplete/MultiSelect) read
// $primevue.config, which exists only once its config plugin is installed
// (main.ts does app.use); theme: 'none' pulls in no visual theme.
//
// ConfirmationService (spec 41): useConfirm() throws without it — installed
// globally here so every component test gets it for free, rather than each
// test that mounts a useConfirm() consumer registering it itself.
config.global.plugins = [
  [PrimeVueConfig, { theme: 'none' }],
  ConfirmationService,
]

// @tanstack/vue-virtual measures its scroll container via offsetWidth/
// offsetHeight, which happy-dom always reports as 0 (no real layout engine
// — the same limitation spec 10 named for its own scrolling e2e case).
// Stubbing a real size is the standard workaround for testing a virtualized
// list under happy-dom/jsdom; it's the minimal mock every such suite needs,
// not a library-behaviour assumption. Tests that need a different size
// (e.g. to size a shift-click range) override it themselves.
Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 800 })
Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, value: 600 })
