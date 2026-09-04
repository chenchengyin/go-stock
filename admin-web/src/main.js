import { createApp, h } from 'vue'
import { NConfigProvider, NMessageProvider } from 'naive-ui'
import App from './App.vue'
import router from './router.js'
import './styles.css'

const Root = {
  setup() {
    return () => h(NConfigProvider, null, {
      default: () => h(NMessageProvider, null, {
        default: () => h(App),
      }),
    })
  },
}

createApp(Root).use(router).mount('#app')
