import { ref } from 'vue'
import { defineStore } from 'pinia'
import { copyRecipe, createRecipe, listRecipes, transitionRecipe } from '../api/culture-recipe'
import { errorMessage } from '../api/client'
import type { CreateRecipeInput, CultureRecipe, RecipeState } from '../types/culture-recipe'

export const useRecipeStore = defineStore('culture-recipes', () => {
  const items = ref<CultureRecipe[]>([])
  const loading = ref(false)
  const error = ref('')
  async function load(search = '') {
    loading.value = true; error.value = ''
    try { items.value = (await listRecipes({ search, page_size: 100 })).items }
    catch (cause) { error.value = errorMessage(cause) }
    finally { loading.value = false }
  }
  async function create(input: CreateRecipeInput) { await createRecipe(input); await load() }
  async function transition(item: CultureRecipe, state: RecipeState) {
    await transitionRecipe(item.id, state, item.version); await load()
  }
  async function copy(item: CultureRecipe) { await copyRecipe(item.id); await load() }
  return { items, loading, error, load, create, transition, copy }
})
