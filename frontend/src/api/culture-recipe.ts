import { api, json, query } from './client'
import type { Page, ListQuery } from '../types/common'
import type { CreateRecipeInput, CultureRecipe, RecipeState } from '../types/culture-recipe'

export const listRecipes = (params: ListQuery & { vessel_id?: number; state?: string } = {}) =>
  api<Page<CultureRecipe>>(`/culture-recipes${query(params)}`)
export const createRecipe = (input: CreateRecipeInput) =>
  api<CultureRecipe>('/culture-recipes', json('POST', input))
export const transitionRecipe = (id: number, toState: RecipeState, version: number, comment = '') =>
  api<CultureRecipe>(`/culture-recipes/${id}/transition`, json('POST', { to_state: toState, version, comment }))
export const copyRecipe = (id: number) =>
  api<CultureRecipe>(`/culture-recipes/${id}/copy`, json('POST', {}))
