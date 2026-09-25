import { api, json, query } from './client'
import type { Page, ListQuery } from '../types/common'
import type { CreateVesselInput, FermentationVessel } from '../types/fermentation-vessel'

export const listVessels = (params: ListQuery & { state?: string } = {}) =>
  api<Page<FermentationVessel>>(`/fermentation-vessels${query(params)}`)
export const getVessel = (id: number) => api<FermentationVessel>(`/fermentation-vessels/${id}`)
export const createVessel = (input: CreateVesselInput) =>
  api<FermentationVessel>('/fermentation-vessels', json('POST', input))
export const deactivateVessel = (id: number) =>
  api<FermentationVessel>(`/fermentation-vessels/${id}/deactivate`, json('POST'))
