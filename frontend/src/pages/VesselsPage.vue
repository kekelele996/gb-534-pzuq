<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { FlaskConical, Plus, Power, RefreshCw, Search } from 'lucide-vue-next'
import AppShell from '../components/common/AppShell.vue'
import DeviationBadge from '../components/common/DeviationBadge.vue'
import PageHeader from '../components/common/PageHeader.vue'
import StateBadge from '../components/common/StateBadge.vue'
import { useAuth } from '../hooks/useAuth'
import { useVesselStore } from '../stores/fermentation-vessel'

const store = useVesselStore()
const { canWriteVessels } = useAuth()
const search = ref('')
const dialog = ref(false)
const saving = ref(false)
const form = reactive({
  vessel_code: '', name: '', working_volume_l: 1000, sensor_channels: 'ph,temperature,do,agitation',
  location: '', owner_team: '', commissioned_at: new Date().toISOString().slice(0, 10),
})
const totals = computed(() => ({
  vessels: store.items.length,
  recipes: store.items.reduce((sum, item) => sum + item.analysis_summary.recipe_count, 0),
  ready: store.items.reduce((sum, item) => sum + item.analysis_summary.ready_series_count, 0),
  attention: store.items.filter((item) => ['major', 'critical'].includes(item.analysis_summary.latest_deviation_level)).length,
}))
async function submit() {
  saving.value = true
  try {
    await store.create({
      ...form, sensor_channels: form.sensor_channels.split(',').map((item) => item.trim()).filter(Boolean),
      commissioned_at: new Date(form.commissioned_at).toISOString(),
    })
    dialog.value = false; ElMessage.success('发酵罐档案已创建')
  } catch (error) { ElMessage.error(error instanceof Error ? error.message : '创建失败') }
  finally { saving.value = false }
}
async function deactivate(id: number) {
  try { await store.deactivate(id); ElMessage.success('发酵罐已停用') }
  catch (error) { ElMessage.error(error instanceof Error ? error.message : '操作失败') }
}
onMounted(() => store.load())
</script>

<template>
  <AppShell>
    <div class="page-wrap">
      <PageHeader eyebrow="ASSET CONTEXT" title="发酵罐总览" description="汇总罐体通道配置、配方版本、时序质量与最近偏差等级。">
        <el-tooltip content="刷新数据"><el-button circle aria-label="刷新" @click="store.load(search)"><RefreshCw :size="17" /></el-button></el-tooltip>
        <el-button v-if="canWriteVessels" type="primary" @click="dialog = true"><Plus :size="16" />新建档案</el-button>
      </PageHeader>
      <section class="metric-band">
        <div><span>在册罐体</span><strong>{{ totals.vessels }}</strong></div>
        <div><span>配方版本</span><strong>{{ totals.recipes }}</strong></div>
        <div><span>就绪时序</span><strong>{{ totals.ready }}</strong></div>
        <div><span>高偏差罐体</span><strong class="severity-number">{{ totals.attention }}</strong></div>
      </section>
      <div class="toolbar">
        <el-input v-model="search" clearable placeholder="搜索编号、名称或位置" @keyup.enter="store.load(search)"><template #prefix><Search :size="15" /></template></el-input>
        <span>{{ store.items.length }} 条罐体记录</span>
      </div>
      <el-alert v-if="store.error" :title="store.error" type="error" :closable="false" show-icon />
      <el-skeleton v-if="store.loading" :rows="7" animated />
      <div v-else-if="!store.items.length" class="empty-state"><FlaskConical :size="30" /><h2>暂无发酵罐</h2><p>创建首个罐体档案后即可维护配方与时序。</p></div>
      <el-table v-else :data="store.items" row-key="id">
        <el-table-column label="罐体" min-width="210">
          <template #default="{ row }"><div class="primary-cell"><strong>{{ row.vessel_code }} · {{ row.name }}</strong><span>{{ row.location }}</span></div></template>
        </el-table-column>
        <el-table-column label="工作容积" width="120"><template #default="{ row }"><span class="numeric">{{ row.working_volume_l.toLocaleString() }} L</span></template></el-table-column>
        <el-table-column label="传感通道" min-width="250"><template #default="{ row }"><div class="token-list"><span v-for="channel in row.sensor_channels" :key="channel">{{ channel }}</span></div></template></el-table-column>
        <el-table-column label="数据质量" width="140"><template #default="{ row }"><span class="numeric">{{ ((1 - row.analysis_summary.latest_missing_rate) * 100).toFixed(0) }}%</span><small class="cell-note">最近完整度</small></template></el-table-column>
        <el-table-column label="最近偏差" width="110"><template #default="{ row }"><DeviationBadge v-if="row.analysis_summary.latest_deviation_level" :level="row.analysis_summary.latest_deviation_level" /><span v-else class="muted">—</span></template></el-table-column>
        <el-table-column label="状态" width="95"><template #default="{ row }"><StateBadge :state="row.vessel_state" /></template></el-table-column>
        <el-table-column v-if="canWriteVessels" label="" width="62" align="right">
          <template #default="{ row }"><el-tooltip v-if="row.vessel_state === 'active'" content="停用罐体"><el-button text circle aria-label="停用罐体" @click="deactivate(row.id)"><Power :size="16" /></el-button></el-tooltip></template>
        </el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dialog" title="新建发酵罐档案" width="min(620px, 94vw)">
      <el-form label-position="top">
        <div class="form-grid two">
          <el-form-item label="罐体编号"><el-input v-model="form.vessel_code" placeholder="FV-401" /></el-form-item>
          <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
          <el-form-item label="工作容积 (L)"><el-input-number v-model="form.working_volume_l" :min="1" /></el-form-item>
          <el-form-item label="投用日期"><el-date-picker v-model="form.commissioned_at" type="date" value-format="YYYY-MM-DD" /></el-form-item>
          <el-form-item label="位置"><el-input v-model="form.location" /></el-form-item>
          <el-form-item label="责任团队"><el-input v-model="form.owner_team" /></el-form-item>
        </div>
        <el-form-item label="传感通道（逗号分隔）"><el-input v-model="form.sensor_channels" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="submit">创建档案</el-button></template>
    </el-dialog>
  </AppShell>
</template>
