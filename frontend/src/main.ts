import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVueConfig from 'openvue/config'
import App from './App.vue'
import './style.css'

// PrimeVueConfig (spec 22): required wiring, not theming — OpenVue's own
// "form" components (InputText, AutoComplete, MultiSelect, first used by
// FilterBar) read $primevue.config during render and crash without it
// (confirmed empirically); Tree/ContextMenu (spec 11/22) don't need it.
// theme: 'none' skips loading any CSS theme, keeping the "no ThemeProvider
// wired" stance (RefsSidebar.vue) — this only provides the behavioural
// config object those components require to exist at all.
createApp(App).use(createPinia()).use(PrimeVueConfig, { theme: 'none' }).mount('#app')
