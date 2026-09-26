import '@/assets/main.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVueConfig from 'openvue/config'
import ConfirmationService from 'openvue/confirmationservice'
import ToastService from 'openvue/toastservice'
import '@openvue/openicons/openicons.css'
import '@fontsource-variable/inter'
import '@fontsource-variable/jetbrains-mono'
import App from './App.vue'
import { NebegitPreset } from '@/theme/preset'
import { installErrorLogging } from '@/lib/errorLogging'
import { i18n } from './i18n'
import { openvueLocale } from '@/lib/openvueLocale'

const app = createApp(App)
installErrorLogging(app)

// PrimeVueConfig themes OpenVue's components with the app's Aura preset;
// without it they render unstyled and the form components crash reading
// $primevue.config. darkModeSelector is the class the settings store
// toggles on <html>. OpenVue's styles go into their own CSS layer, between
// Tailwind's base and its utilities.
// locale gives its components' own texts from the catalogue.
// ConfirmationService backs useConfirm() and App.vue's <ConfirmDialog>.
// ToastService backs useToast() and App.vue's <NoticeArea>.
app
  .use(createPinia())
  .use(i18n)
  .use(PrimeVueConfig, {
    theme: {
      preset: NebegitPreset,
      options: {
        darkModeSelector: '.nebegit-dark',
        cssLayer: { name: 'openvue', order: 'theme, base, openvue, components, utilities' },
      },
    },
    locale: openvueLocale(i18n.global.t),
  })
  .use(ConfirmationService)
  .use(ToastService)
  .mount('#app')
