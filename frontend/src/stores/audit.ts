import { ref } from 'vue'
import { defineStore } from 'pinia'
import { listAuditLogs } from '../api/audit'
import { errorMessage } from '../api/client'
import type { AuditLog } from '../types/audit'

export const useAuditStore = defineStore('audit', () => {
  const items = ref<AuditLog[]>([])
  const loading = ref(false)
  const error = ref('')
  async function load(filters: { entity_type?: string; request_id?: string } = {}) {
    loading.value = true; error.value = ''
    try { items.value = (await listAuditLogs({ ...filters, page_size: 100 })).items }
    catch (cause) { error.value = errorMessage(cause) }
    finally { loading.value = false }
  }
  return { items, loading, error, load }
})
