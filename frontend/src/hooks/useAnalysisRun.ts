import { computed, ref } from 'vue'
import { useAnalysisStore } from '../stores/deviation-analysis'
import { useSeriesStore } from '../stores/sensor-series'

export function useAnalysisRun() {
  const analyses = useAnalysisStore()
  const series = useSeriesStore()
  const seriesId = ref<number>()
  const readySeries = computed(() => series.items.filter((item) => item.series_state === 'ready'))
  async function run() {
    if (!seriesId.value) return
    const key = crypto.randomUUID ? crypto.randomUUID() : String(Date.now())
    await analyses.run(seriesId.value, key)
  }
  return { seriesId, readySeries, running: computed(() => analyses.running), run }
}
