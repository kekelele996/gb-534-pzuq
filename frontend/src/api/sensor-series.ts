import { api, json, query } from './client'
import type { Page, ListQuery } from '../types/common'
import type { ImportSeriesInput, SensorSeries, SeriesState } from '../types/sensor-series'

export const listSeries = (params: ListQuery & { vessel_id?: number; recipe_id?: number; state?: string } = {}) =>
  api<Page<SensorSeries>>(`/sensor-series${query(params)}`)
export const importSeries = (input: ImportSeriesInput) =>
  api<SensorSeries>('/sensor-series', json('POST', input))
export const transitionSeries = (id: number, toState: SeriesState, comment = '') =>
  api<SensorSeries>(`/sensor-series/${id}/transition`, json('POST', { to_state: toState, comment }))
