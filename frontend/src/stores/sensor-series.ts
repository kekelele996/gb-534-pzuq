import { ref } from 'vue'
import { defineStore } from 'pinia'
import { importSeries, listSeries, transitionSeries } from '../api/sensor-series'
import { errorMessage } from '../api/client'
import type { ImportSeriesInput, SensorSeries, SeriesState } from '../types/sensor-series'

export const useSeriesStore = defineStore('sensor-series', () => {
  const items = ref<SensorSeries[]>([])
  const loading = ref(false)
  const error = ref('')
  async function load(search = '') {
    loading.value = true; error.value = ''
    try { items.value = (await listSeries({ search, page_size: 100 })).items }
    catch (cause) { error.value = errorMessage(cause) }
    finally { loading.value = false }
  }
  async function importData(input: ImportSeriesInput) { await importSeries(input); await load() }
  async function transition(item: SensorSeries, state: SeriesState) {
    await transitionSeries(item.id, state); await load()
  }
  return { items, loading, error, load, importData, transition }
})
