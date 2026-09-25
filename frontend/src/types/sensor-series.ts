import type { CultureRecipe } from './culture-recipe'
import type { FermentationVessel } from './fermentation-vessel'

export type SeriesState = 'imported' | 'validated' | 'normalized' | 'ready' | 'rejected' | 'superseded'
export interface SensorPoint { timestamp: string; values: Record<string, number | null> }
export interface QualitySummary {
  original_point_count?: number
  unique_point_count?: number
  duplicate_count?: number
  long_gap_count?: number
  max_gap_seconds?: number
  missing_rate?: Record<string, number>
  channels?: string[]
  warnings?: string[]
  valid?: boolean
}

export interface SensorSeries {
  id: number
  vessel_id: number
  recipe_id: number
  run_code: string
  channel: string
  sample_interval_s: number
  points_json: SensorPoint[]
  started_at: string
  ended_at: string
  source_checksum: string
  series_state: SeriesState
  quality_summary: QualitySummary
  normalization_json: Record<string, unknown>
  imported_by: number
  imported_by_name: string
  vessel?: FermentationVessel
  recipe?: CultureRecipe
  created_at: string
  updated_at: string
}

export interface ImportSeriesInput {
  vessel_id: number
  recipe_id: number
  run_code: string
  channel: string
  sample_interval_s: number
  points_json: SensorPoint[]
}
