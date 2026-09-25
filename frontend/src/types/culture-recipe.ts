import type { FermentationPhase } from './enums/fermentation-phase'
import type { FermentationVessel } from './fermentation-vessel'

export type RecipeState = 'draft' | 'validated' | 'published' | 'obsolete'
export interface PhaseBoundary { phase: FermentationPhase; start_hour: number; end_hour: number }
export interface CurvePoint { elapsed_h: number; value: number }
export interface ChannelTolerance { weight: number; max_distance: number }

export interface CultureRecipe {
  id: number
  vessel_id: number
  vessel?: FermentationVessel
  recipe_code: string
  version: number
  organism: string
  target_duration_h: number
  phase_boundaries_json: PhaseBoundary[]
  reference_curves_json: Record<string, CurvePoint[]>
  tolerance_profile_json: Record<string, ChannelTolerance>
  recipe_state: RecipeState
  created_by: number
  created_by_name: string
  created_at: string
  updated_at: string
}

export interface CreateRecipeInput {
  vessel_id: number
  recipe_code: string
  organism: string
  target_duration_h: number
  phase_boundaries_json: PhaseBoundary[]
  reference_curves_json: Record<string, CurvePoint[]>
  tolerance_profile_json: Record<string, ChannelTolerance>
}
