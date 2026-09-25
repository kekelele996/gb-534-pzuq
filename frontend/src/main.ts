import { createApp } from 'vue'
import { createPinia } from 'pinia'
import {
  ElAlert, ElButton, ElDatePicker, ElDialog, ElDrawer, ElDropdown, ElDropdownItem,
  ElDropdownMenu, ElForm, ElFormItem, ElInput, ElInputNumber, ElLoading, ElOption,
  ElPagination, ElSelect, ElSkeleton, ElSwitch, ElTable, ElTableColumn, ElTooltip,
} from 'element-plus'
import 'element-plus/dist/index.css'
import App from './App.vue'
import router from './router'
import './styles.css'

const app = createApp(App).use(createPinia()).use(router).use(ElLoading)
const components = [
  ElAlert, ElButton, ElDatePicker, ElDialog, ElDrawer, ElDropdown, ElDropdownItem,
  ElDropdownMenu, ElForm, ElFormItem, ElInput, ElInputNumber, ElOption,
  ElPagination, ElSelect, ElSkeleton, ElSwitch, ElTable, ElTableColumn, ElTooltip,
]
for (const component of components) app.component(component.name!, component)
app.mount('#app')
