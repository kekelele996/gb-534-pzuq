import { ref } from 'vue'
import { defineStore } from 'pinia'
import { createVessel, deactivateVessel, listVessels } from '../api/fermentation-vessel'
import { errorMessage } from '../api/client'
import type { CreateVesselInput, FermentationVessel } from '../types/fermentation-vessel'

export const useVesselStore = defineStore('fermentation-vessels', () => {
  const items = ref<FermentationVessel[]>([])
  const loading = ref(false)
  const error = ref('')
  async function load(search = '') {
    loading.value = true; error.value = ''
    try { items.value = (await listVessels({ search, page_size: 100 })).items }
    catch (cause) { error.value = errorMessage(cause) }
    finally { loading.value = false }
  }
  async function create(input: CreateVesselInput) { await createVessel(input); await load() }
  async function deactivate(id: number) { await deactivateVessel(id); await load() }
  return { items, loading, error, load, create, deactivate }
})
