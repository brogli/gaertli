import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVueConfig from 'openvue/config'
import ConfirmationService from 'openvue/confirmationservice'
import App from './App.vue'
import './style.css'

// PrimeVueConfig is required wiring, not theming: OpenVue's form components
// (InputText, AutoComplete, MultiSelect) read $primevue.config during render
// and crash without it. theme: 'none' loads no CSS theme.
//
// ConfirmationService backs useConfirm()/<ConfirmDialog> in App.vue, the
// shared confirmation for destructive actions.
createApp(App)
  .use(createPinia())
  .use(PrimeVueConfig, { theme: 'none' })
  .use(ConfirmationService)
  .mount('#app')
