import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './style.css'
import { useThemeStore } from './stores/theme'

const app = createApp(App).use(createPinia()).use(router)
useThemeStore().bindSystem()
app.mount('#app')
