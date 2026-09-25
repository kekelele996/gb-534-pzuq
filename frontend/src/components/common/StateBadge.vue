<script setup lang="ts">
defineProps<{ state: string }>()
const labels: Record<string, string> = {
  active: '运行中', inactive: '已停用', draft: '草稿', validated: '已校验', published: '已发布',
  obsolete: '已废止', imported: '已导入', normalized: '已标准化', ready: '就绪', rejected: '已拒绝',
  superseded: '已替代', queued: '排队中', analyzing: '分析中', completed: '已完成', failed: '失败',
  reviewed: '已复核', confirmed: '已确认', investigating: '调查中', voided: '已作废',
}
</script>
<template><span class="state-badge" :class="state">{{ labels[state] ?? state }}</span></template>
