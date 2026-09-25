<script setup lang="ts">
defineProps<{ level: string }>()
const labels: Record<string, string> = { normal: '正常', watch: '关注', major: '重大', critical: '严重' }
</script>
<template><span class="deviation-badge" :class="level"><i />{{ labels[level] ?? level }}</span></template>
