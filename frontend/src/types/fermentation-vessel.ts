export type VesselState = 'active' | 'inactive'

export interface VesselSummary {
  recipe_count: number
  series_count: number
  ready_series_count: number
  latest_missing_rate: number
  latest_deviation_level: string
}

export interface FermentationVessel {
  id: number
  vessel_code: string
  name: string
  working_volume_l: number
  sensor_channels: string[]
  location: string
  owner_team: string
  vessel_state: VesselState
  commissioned_at: string
  analysis_summary: VesselSummary
  created_at: string
  updated_at: string
}

export interface CreateVesselInput {
  vessel_code: string
  name: string
  working_volume_l: number
  sensor_channels: string[]
  location: string
  owner_team: string
  commissioned_at: string
}
