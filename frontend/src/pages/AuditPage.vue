<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { FileSearch, RefreshCw, Search } from 'lucide-vue-next'
import AnalysisExplanationDrawer from '../components/common/AnalysisExplanationDrawer.vue'
import AppShell from '../components/common/AppShell.vue'
import AuditEvidenceDrawer from '../components/common/AuditEvidenceDrawer.vue'
import PageHeader from '../components/common/PageHeader.vue'
import { useAnalysisStore } from '../stores/deviation-analysis'
import { useAuditStore } from '../stores/audit'
import type { AuditLog } from '../types/audit'

const audit = useAuditStore()
const analyses = useAnalysisStore()
const filters = reactive({ entity_type: '', request_id: '' })
const selectedAudit = ref<AuditLog | null>(null)
const evidenceDrawer = ref(false)
const explanationDrawer = ref(false)
const algorithmEvents = computed(() => audit.items.filter((item) => item.algorithm_version).length)
const selectedAnalysis = computed(() => selectedAudit.value?.entity_type === 'deviation_analysis'
  ? analyses.items.find((item) => item.id === selectedAudit.value?.entity_id) ?? null
  : null)
function inspect(row: AuditLog) {
  selectedAudit.value = row
  evidenceDrawer.value = true
}
function explainSelected() {
  if (!selectedAnalysis.value) { ElMessage.info('对应分析记录不可用'); return }
  analyses.selected = selectedAnalysis.value
  evidenceDrawer.value = false
  explanationDrawer.value = true
}
onMounted(() => Promise.all([audit.load(), analyses.load()]))
</script>

<template>
  <AppShell>
    <div class="page-wrap">
      <PageHeader eyebrow="TRACEABLE CHANGE PROJECTION" title="审计中心" description="按实体、request ID、操作者与时间追踪写操作和算法证据。">
        <el-tooltip content="刷新数据"><el-button circle aria-label="刷新" @click="audit.load(filters)"><RefreshCw :size="17" /></el-button></el-tooltip>
      </PageHeader>
      <section class="metric-band audit">
        <div><span>当前事件</span><strong>{{ audit.items.length }}</strong></div>
        <div><span>算法事件</span><strong>{{ algorithmEvents }}</strong></div>
        <div><span>独立 request ID</span><strong>{{ new Set(audit.items.map((item) => item.request_id)).size }}</strong></div>
        <div><span>实体类型</span><strong>{{ new Set(audit.items.map((item) => item.entity_type)).size }}</strong></div>
      </section>
      <div class="audit-filters">
        <el-select v-model="filters.entity_type" clearable placeholder="全部实体">
          <el-option label="发酵罐" value="fermentation_vessel" /><el-option label="培养配方" value="culture_recipe" />
          <el-option label="传感器时序" value="sensor_series" /><el-option label="偏差分析" value="deviation_analysis" />
        </el-select>
        <el-input v-model="filters.request_id" clearable placeholder="request ID"><template #prefix><Search :size="15" /></template></el-input>
        <el-button @click="audit.load(filters)">筛选</el-button>
      </div>
      <el-alert v-if="audit.error" :title="audit.error" type="error" :closable="false" show-icon />
      <el-skeleton v-if="audit.loading" :rows="7" animated />
      <div v-else-if="!audit.items.length" class="empty-state"><h2>暂无审计事件</h2><p>写操作发生后会记录请求与快照。</p></div>
      <el-table v-else :data="audit.items" row-key="id">
        <el-table-column label="时间" width="175"><template #default="{ row }">{{ new Date(row.created_at).toLocaleString() }}</template></el-table-column>
        <el-table-column label="实体" min-width="170"><template #default="{ row }"><div class="primary-cell"><strong>{{ row.entity_type }} #{{ row.entity_id }}</strong><span>{{ row.action }}</span></div></template></el-table-column>
        <el-table-column label="操作者" width="150"><template #default="{ row }"><div class="primary-cell"><strong>{{ row.actor_name }}</strong><span>{{ row.actor_role }}</span></div></template></el-table-column>
        <el-table-column label="Request ID" min-width="240"><template #default="{ row }"><code class="request-code">{{ row.request_id }}</code></template></el-table-column>
        <el-table-column label="算法" width="160"><template #default="{ row }"><span>{{ row.algorithm_version || '—' }}</span><small v-if="row.duration_ms" class="cell-note">{{ row.duration_ms }} ms</small></template></el-table-column>
        <el-table-column label="" width="58"><template #default="{ row }"><el-tooltip content="查看审计证据"><el-button text circle aria-label="查看审计证据" @click="inspect(row)"><FileSearch :size="16" /></el-button></el-tooltip></template></el-table-column>
      </el-table>
    </div>
    <AuditEvidenceDrawer v-model="evidenceDrawer" :event="selectedAudit" :analysis-available="Boolean(selectedAnalysis)" @open-analysis="explainSelected" />
    <AnalysisExplanationDrawer v-model="explanationDrawer" :analysis="analyses.selected" />
  </AppShell>
</template>
