<script setup lang="ts">
import type { FermentationPhase } from '../../types/enums/fermentation-phase'
defineProps<{ phase: FermentationPhase | string }>()
const labels: Record<string, string> = { lag: '延滞期', growth: '生长期', production: '产物期', harvest: '收获期' }
</script>
<template><span class="phase-badge" :class="`phase-${phase}`"><i />{{ labels[phase] ?? phase }}</span></template>
