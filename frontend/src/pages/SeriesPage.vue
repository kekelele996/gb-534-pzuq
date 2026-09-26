<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { FileUp, RefreshCw } from 'lucide-vue-next'
import AppShell from '../components/common/AppShell.vue'
import KineticsChart from '../components/common/KineticsChart.vue'
import PageHeader from '../components/common/PageHeader.vue'
import PhaseBadge from '../components/common/PhaseBadge.vue'
import StateBadge from '../components/common/StateBadge.vue'
import { useAuth } from '../hooks/useAuth'
import { useRecipeStore } from '../stores/culture-recipe'
import { useSeriesStore } from '../stores/sensor-series'
import { useVesselStore } from '../stores/fermentation-vessel'
import type { SensorPoint, SensorSeries, SeriesState } from '../types/sensor-series'

const store = useSeriesStore()
const recipes = useRecipeStore()
const vessels = useVesselStore()
const { canImportSeries, canProcessSeries } = useAuth()
const selected = ref<SensorSeries | null>(null)
const dialog = ref(false)
const saving = ref(false)
const form = reactive({ recipe_id: undefined as number | undefined, run_code: '', channel: 'multichannel', sample_interval_s: 7200 })
const publishedRecipes = computed(() => recipes.items.filter((item) => item.recipe_state === 'published'))
const selectedRecipe = computed(() => recipes.items.find((item) => item.id === form.recipe_id))
const worstMissing = computed(() => {
  const rates = Object.values(selected.value?.quality_summary.missing_rate ?? {})
  return rates.length ? Math.max(...rates) : 0
})
const phaseLabels: Record<string, string> = { lag: '延滞期', growth: '生长期', production: '产物期', harvest: '收获期' }
const phaseGroups = computed(() => selected.value?.quality_summary.phase_missing ?? [])
const criticalPhases = new Set(['growth', 'production'])
function phaseRateClass(rate: number, phase: string) {
  const limit = criticalPhases.has(phase) ? 0.2 : 0.35
  return rate > limit ? 'rate-bad' : rate > 0.1 ? 'rate-warn' : 'rate-ok'
}
function formatPercent(rate: number) { return `${(rate * 100).toFixed(1)}%` }
watch(() => store.items, (items) => {
  if (!selected.value || !items.some((item) => item.id === selected.value?.id)) selected.value = items[0] ?? null
  else selected.value = items.find((item) => item.id === selected.value?.id) ?? null
}, { immediate: true })

function samplePoints(): SensorPoint[] {
  const start = Date.now() - 24 * 3_600_000
  return Array.from({ length: 13 }, (_, index) => {
    const hour = index * 2
    return {
      timestamp: new Date(start + hour * 3_600_000).toISOString(),
      values: {
        ph: 6.8 - hour * .025 + Math.sin(hour / 4) * .05,
        temperature: 29.5 + hour * .04,
        do: 68 - hour * 1.35 + Math.sin(hour / 3) * 3,
        agitation: 320 + hour * 9.2,
      },
    }
  })
}
async function submit() {
  if (!selectedRecipe.value) return
  saving.value = true
  try {
    await store.importData({
      vessel_id: selectedRecipe.value.vessel_id, recipe_id: selectedRecipe.value.id,
      run_code: form.run_code, channel: form.channel, sample_interval_s: form.sample_interval_s,
      points_json: samplePoints(),
    })
    dialog.value = false; selected.value = store.items[0] ?? null; ElMessage.success('时序已导入')
  } catch (error) { ElMessage.error(error instanceof Error ? error.message : '导入失败') }
  finally { saving.value = false }
}
function nextState(state: SeriesState): SeriesState | null {
  if (state === 'imported') return 'validated'
  if (state === 'validated') return 'normalized'
  if (state === 'normalized') return 'ready'
  if (state === 'ready') return 'superseded'
  return null
}
async function advance(item: SensorSeries) {
  const state = nextState(item.series_state)
  if (!state) return
  try {
    await store.transition(item, state)
    selected.value = store.items.find((row) => row.id === item.id) ?? null
    ElMessage.success('时序状态已更新')
  } catch (error) { ElMessage.error(error instanceof Error ? error.message : '处理失败') }
}
onMounted(async () => {
  await Promise.all([store.load(), recipes.load(), vessels.load()])
  selected.value = store.items[0] ?? null
})
</script>

<template>
  <AppShell>
    <div class="page-wrap">
      <PageHeader eyebrow="MULTICHANNEL EVIDENCE" title="时序工作台" description="检查排序、重复时间戳、缺失率与稳健缩放证据，长间隔始终保留。">
        <el-tooltip content="刷新数据"><el-button circle aria-label="刷新" @click="store.load()"><RefreshCw :size="17" /></el-button></el-tooltip>
        <el-button v-if="canImportSeries" type="primary" @click="dialog = true"><FileUp :size="16" />导入时序</el-button>
      </PageHeader>
      <el-alert v-if="store.error" :title="store.error" type="error" :closable="false" show-icon />
      <div class="series-workspace">
        <section class="series-list">
          <div class="section-heading"><div><h2>批次记录</h2><p>{{ store.items.length }} 条时序</p></div></div>
          <el-skeleton v-if="store.loading" :rows="6" animated />
          <button v-for="item in store.items" v-else :key="item.id" class="series-row" :class="{ selected: selected?.id === item.id }" @click="selected = item">
            <span class="run-index">#{{ item.id }}</span>
            <span><strong>{{ item.run_code }}</strong><small>{{ item.vessel?.vessel_code }} · {{ item.recipe?.recipe_code }} v{{ item.recipe?.version }}</small></span>
            <StateBadge :state="item.series_state" />
          </button>
          <div v-if="!store.loading && !store.items.length" class="empty-inline">暂无时序记录</div>
        </section>
        <section class="series-detail">
          <template v-if="selected">
            <div class="detail-heading">
              <div><p class="eyebrow">RUN EVIDENCE</p><h2>{{ selected.run_code }}</h2></div>
              <el-button v-if="canProcessSeries && nextState(selected.series_state)" size="small" @click="advance(selected)">
                {{ selected.series_state === 'imported' ? '执行校验' : selected.series_state === 'validated' ? '稳健缩放' : selected.series_state === 'normalized' ? '标记就绪' : '标记替代' }}
              </el-button>
            </div>
            <div class="quality-band">
              <div><span>唯一观测</span><strong>{{ selected.quality_summary.unique_point_count ?? selected.points_json.length }}</strong></div>
              <div><span>重复时间戳</span><strong>{{ selected.quality_summary.duplicate_count ?? 0 }}</strong></div>
              <div><span>最长间隔</span><strong>{{ Math.round((selected.quality_summary.max_gap_seconds ?? 0) / 60) }} min</strong></div>
              <div><span>最高缺失率</span><strong :class="{ 'severity-number': worstMissing > .1 }">{{ (worstMissing * 100).toFixed(1) }}%</strong></div>
            </div>
            <div v-if="phaseGroups.length" class="phase-quality">
              <div class="phase-quality-heading">
                <strong>分阶段缺失率</strong>
                <small>生长期 / 产物期任一通道超过 20% 直接拒绝；延滞期 / 收获期按整条 35% 门槛</small>
              </div>
              <div class="phase-quality-grid">
                <div v-for="group in phaseGroups" :key="group.phase" class="phase-quality-card" :class="{ 'phase-critical': criticalPhases.has(group.phase) }">
                  <div class="phase-card-head">
                    <PhaseBadge :phase="group.phase" />
                    <span class="phase-threshold">{{ criticalPhases.has(group.phase) ? '门槛 20%' : '整条 35%' }}</span>
                  </div>
                  <div class="phase-worst" :class="phaseRateClass(group.worst_missing_rate, group.phase)">
                    <span>最差通道</span>
                    <strong>{{ group.worst_channel || '—' }} · {{ formatPercent(group.worst_missing_rate ?? 0) }}</strong>
                    <small>{{ group.observed_point_count }} 个阶段内观测</small>
                  </div>
                  <ul class="phase-channel-rates">
                    <li v-for="(rate, channel) in group.missing_rate" :key="`${group.phase}-${channel}`">
                      <span>{{ channel }}</span>
                      <i :class="phaseRateClass(rate, group.phase)">{{ formatPercent(rate) }}</i>
                    </li>
                  </ul>
                </div>
              </div>
            </div>
            <el-alert
              v-if="selected.quality_summary.rejection_reason"
              type="error" :closable="false" show-icon
              title="校验已拒绝" :description="selected.quality_summary.rejection_reason"
            />
            <KineticsChart :points="selected.points_json" :height="360" />
            <div v-if="selected.quality_summary.warnings?.length" class="warning-list">
              <strong>质量提示</strong><span v-for="warning in selected.quality_summary.warnings" :key="warning">{{ warning }}</span>
            </div>
            <dl class="evidence-grid compact">
              <div><dt>来源校验和</dt><dd>{{ selected.source_checksum }}</dd></div>
              <div><dt>采样间隔</dt><dd>{{ selected.sample_interval_s }} s</dd></div>
              <div><dt>开始时间</dt><dd>{{ new Date(selected.started_at).toLocaleString() }}</dd></div>
              <div><dt>导入人</dt><dd>{{ selected.imported_by_name }}</dd></div>
            </dl>
          </template>
          <div v-else class="empty-state"><h2>选择一条时序</h2><p>曲线与质量证据将在此显示。</p></div>
        </section>
      </div>
    </div>
    <el-dialog v-model="dialog" title="导入传感器时序" width="min(620px, 94vw)">
      <el-form label-position="top">
        <el-form-item label="已发布配方"><el-select v-model="form.recipe_id" placeholder="选择配方版本"><el-option v-for="recipe in publishedRecipes" :key="recipe.id" :label="`${recipe.recipe_code} v${recipe.version} · ${recipe.vessel?.vessel_code ?? ''}`" :value="recipe.id" /></el-select></el-form-item>
        <div class="form-grid two">
          <el-form-item label="运行编号"><el-input v-model="form.run_code" placeholder="RUN-2026-0822-C" /></el-form-item>
          <el-form-item label="采样间隔 (s)"><el-input-number v-model="form.sample_interval_s" :min="1" /></el-form-item>
        </div>
        <el-form-item label="通道模式"><el-input v-model="form.channel" disabled /></el-form-item>
        <div class="configuration-note">本次导入生成 24 小时、四通道演示数据，并通过真实导入 API 完成时间戳与质量校验。</div>
      </el-form>
      <template #footer><el-button @click="dialog = false">取消</el-button><el-button type="primary" :loading="saving" :disabled="!form.recipe_id || !form.run_code" @click="submit">导入</el-button></template>
    </el-dialog>
  </AppShell>
</template>
