<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Activity, FlaskConical, LockKeyhole } from 'lucide-vue-next'
import { errorMessage } from '../api/client'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const form = ref({ username: 'admin', password: 'admin123' })
const loading = ref(false)
const error = ref('')
async function submit() {
  loading.value = true; error.value = ''
  try { await auth.login(form.value); await router.push('/vessels') }
  catch (cause) { error.value = errorMessage(cause) }
  finally { loading.value = false }
}
</script>

<template>
  <main class="login-shell">
    <section class="login-context">
      <div class="login-brand"><FlaskConical :size="22" /><span>Kinetics Lab</span></div>
      <div class="login-title">
        <p class="eyebrow">BIOPROCESS EVIDENCE WORKBENCH</p>
        <h1>发酵动力学<br />偏差分析</h1>
        <p>把配方阶段、传感器时序与可重放的曲线证据放在同一条审阅链路中。</p>
      </div>
      <div class="signal-board" aria-hidden="true">
        <Activity :size="18" /><span v-for="height in [24,42,36,60,48,72,55,44,30,18]" :key="height" :style="{ height: `${height}px` }" />
      </div>
      <p class="login-boundary">OFFLINE ANALYSIS · NO EQUIPMENT CONTROL</p>
    </section>
    <section class="login-panel">
      <form class="login-form" @submit.prevent="submit">
        <LockKeyhole :size="24" />
        <div><p class="eyebrow">SECURE ACCESS</p><h2>进入工艺工作台</h2></div>
        <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
        <el-form label-position="top">
          <el-form-item label="账号"><el-input v-model="form.username" autocomplete="username" /></el-form-item>
          <el-form-item label="密码"><el-input v-model="form.password" type="password" show-password autocomplete="current-password" /></el-form-item>
        </el-form>
        <el-button native-type="submit" type="primary" :loading="loading">登录</el-button>
        <p class="credential-note">默认管理员：admin / admin123</p>
      </form>
    </section>
  </main>
</template>
