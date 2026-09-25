<script setup lang="ts">
import { Activity, BookOpen, Cylinder, FlaskConical, LogOut, ScanSearch, ScrollText } from 'lucide-vue-next'
import { useRouter } from 'vue-router'
import { useAuth } from '../../hooks/useAuth'

const router = useRouter()
const { auth, canAudit } = useAuth()
const links = [
  { to: '/vessels', label: '发酵罐', icon: Cylinder },
  { to: '/recipes', label: '配方版本', icon: BookOpen },
  { to: '/series', label: '时序工作台', icon: Activity },
  { to: '/analyses', label: '偏差分析', icon: ScanSearch },
]
function logout() { auth.logout(); router.push('/login') }
</script>

<template>
  <div class="app-shell">
    <aside class="app-sidebar">
      <div class="brand-block">
        <span class="brand-mark"><FlaskConical :size="20" /></span>
        <div><strong>Kinetics Lab</strong><span>发酵动力学工作台</span></div>
      </div>
      <nav aria-label="主导航">
        <RouterLink v-for="link in links" :key="link.to" :to="link.to" class="nav-link">
          <component :is="link.icon" :size="17" /><span>{{ link.label }}</span>
        </RouterLink>
        <RouterLink v-if="canAudit" to="/audit" class="nav-link"><ScrollText :size="17" /><span>审计中心</span></RouterLink>
      </nav>
      <div class="identity-block">
        <span>{{ auth.user?.display_name }}</span>
        <strong>{{ auth.user?.role }}</strong>
        <el-tooltip content="退出登录" placement="right">
          <el-button text circle aria-label="退出登录" @click="logout"><LogOut :size="17" /></el-button>
        </el-tooltip>
      </div>
    </aside>
    <main class="app-main">
      <div class="context-strip"><span>OFFLINE PROCESS EVIDENCE</span><strong>ALGORITHM phase-dtw-v1.0.0</strong></div>
      <div class="boundary-strip">
        <FlaskConical :size="17" />
        <strong>离线分析边界</strong>
        <span>结果仅用于工艺证据审阅，不连接或控制发酵罐、泵、阀门与加料系统。</span>
      </div>
      <slot />
    </main>
  </div>
</template>
