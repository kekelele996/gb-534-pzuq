import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'

export function useAuth() {
  const auth = useAuthStore()
  const canWriteVessels = computed(() => auth.hasRole('admin', 'process_scientist'))
  const canWriteRecipes = computed(() => auth.hasRole('admin', 'process_scientist'))
  const canImportSeries = computed(() => auth.hasRole('admin', 'data_analyst'))
  const canProcessSeries = computed(() => auth.hasRole('admin', 'process_scientist', 'data_analyst'))
  const canRunAnalysis = computed(() => auth.hasRole('admin', 'data_analyst'))
  const canReview = computed(() => auth.hasRole('admin', 'process_scientist', 'reviewer'))
  const canConfirm = computed(() => auth.hasRole('admin', 'reviewer'))
  const canAudit = computed(() => auth.hasRole('admin', 'reviewer', 'auditor'))
  return { auth, canWriteVessels, canWriteRecipes, canImportSeries, canProcessSeries, canRunAnalysis, canReview, canConfirm, canAudit }
}
