import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVueConfig from 'openvue/config'
import ConfirmationService from 'openvue/confirmationservice'
import Aura from '@openvue/themes/aura'
import App from './App.vue'
import './style.css'

// PrimeVueConfig themes OpenVue's components with Aura; without it they
// render unstyled and the form components crash reading $primevue.config.
// darkModeSelector is the class the settings store toggles on <html>.
// ConfirmationService backs useConfirm() and App.vue's <ConfirmDialog>.
createApp(App)
  .use(createPinia())
  .use(PrimeVueConfig, { theme: { preset: Aura, options: { darkModeSelector: '.nebegit-dark', cssLayer: false } } })
  .use(ConfirmationService)
  .mount('#app')
