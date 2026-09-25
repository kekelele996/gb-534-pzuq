<script setup lang="ts">
import { computed } from 'vue'
import { Braces, FileSearch, X } from 'lucide-vue-next'
import type { AuditLog } from '../../types/audit'

const props = defineProps<{ modelValue: boolean; event: AuditLog | null; analysisAvailable?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; 'open-analysis': [] }>()

function readableJSON(raw?: string) {
  if (!raw) return '{}'
  try { return JSON.stringify(JSON.parse(raw), null, 2) }
  catch { return raw }
}
const beforeSnapshot = computed(() => readableJSON(props.event?.before_snapshot))
const afterSnapshot = computed(() => readableJSON(props.event?.after_snapshot))
</script>

<template>
  <el-drawer :model-value="modelValue" size="min(960px, 96vw)" :with-header="false" @close="emit('update:modelValue', false)">
    <div class="drawer-heading">
      <div><Braces :size="22" /><span><small>审计证据</small><strong>{{ event ? `${event.entity_type} #${event.entity_id}` : '—' }}</strong></span></div>
      <el-button text circle aria-label="关闭" @click="emit('update:modelValue', false)"><X :size="18" /></el-button>
    </div>
    <template v-if="event">
      <dl class="audit-meta-grid">
        <div><dt>动作</dt><dd>{{ event.action }}</dd></div>
        <div><dt>操作者</dt><dd>{{ event.actor_name }} · {{ event.actor_role }}</dd></div>
        <div><dt>Request ID</dt><dd>{{ event.request_id }}</dd></div>
        <div><dt>时间</dt><dd>{{ new Date(event.created_at).toLocaleString() }}</dd></div>
        <div><dt>算法版本</dt><dd>{{ event.algorithm_version || '不适用' }}</dd></div>
        <div><dt>耗时</dt><dd>{{ event.duration_ms !== undefined ? `${event.duration_ms} ms` : '不适用' }}</dd></div>
        <div class="wide"><dt>输入哈希</dt><dd>{{ event.input_hash || '不适用' }}</dd></div>
        <div v-if="event.result_summary" class="wide"><dt>结果摘要</dt><dd>{{ event.result_summary }}</dd></div>
      </dl>
      <section class="drawer-section">
        <div class="snapshot-heading">
          <div><h3>前后快照</h3><p>原始审计 JSON 仅格式化展示，不改变持久化证据。</p></div>
          <el-button v-if="event.entity_type === 'deviation_analysis' && analysisAvailable" @click="emit('open-analysis')">
            <FileSearch :size="16" />关联分析解释
          </el-button>
        </div>
        <div class="snapshot-grid">
          <article class="snapshot-panel"><h4>变更前</h4><pre>{{ beforeSnapshot }}</pre></article>
          <article class="snapshot-panel after"><h4>变更后</h4><pre>{{ afterSnapshot }}</pre></article>
        </div>
      </section>
    </template>
  </el-drawer>
</template>
