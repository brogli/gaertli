import { config } from '@vue/test-utils'
import PrimeVueConfig from 'openvue/config'

// OpenVue's form components (InputText/AutoComplete/MultiSelect) read
// $primevue.config, which exists only once its config plugin is installed
// (main.ts does app.use); theme: 'none' pulls in no visual theme.
config.global.plugins = [[PrimeVueConfig, { theme: 'none' }]]
