import { createApp, h } from 'vue'
import { NConfigProvider, NMessageProvider } from 'naive-ui'
import './styles.css'

const Root = {
  setup() {
    return () => h(NConfigProvider, null, {
      default: () => h(NMessageProvider, null, {
        default: () => h('main', { class: 'admin-bootstrap' }, [
          h('h1', '盘达权限管理'),
          h('p', '管理网页正在加载。'),
        ]),
      }),
    })
  },
}

createApp(Root).mount('#app')
