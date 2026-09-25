import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVueConfig from 'openvue/config'
import ConfirmationService from 'openvue/confirmationservice'
import ToastService from 'openvue/toastservice'
import Aura from '@openvue/themes/aura'
import '@openvue/openicons/openicons.css'
import '@fontsource-variable/inter'
import '@fontsource-variable/jetbrains-mono'
import App from './App.vue'
import './style.css'
import { installErrorLogging } from './lib/errorLogging'
import { i18n } from './i18n'
import { openvueLocale } from './lib/openvueLocale'

const app = createApp(App)
installErrorLogging(app)

// PrimeVueConfig themes OpenVue's components with Aura; without it they
// render unstyled and the form components crash reading $primevue.config.
// darkModeSelector is the class the settings store toggles on <html>.
// locale gives its components' own texts from the catalogue.
// ConfirmationService backs useConfirm() and App.vue's <ConfirmDialog>.
// ToastService backs useToast() and App.vue's <NoticeArea>.
app
  .use(createPinia())
  .use(i18n)
  .use(PrimeVueConfig, {
    theme: { preset: Aura, options: { darkModeSelector: '.nebegit-dark', cssLayer: false } },
    locale: openvueLocale(i18n.global.t),
  })
  .use(ConfirmationService)
  .use(ToastService)
  .mount('#app')
