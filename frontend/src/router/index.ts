import { createRouter, createWebHistory } from 'vue-router'
import { tokenKey } from '../api/client'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/vessels' },
    { path: '/login', name: 'login', component: () => import('../pages/LoginPage.vue'), meta: { public: true } },
    { path: '/vessels', name: 'vessels', component: () => import('../pages/VesselsPage.vue') },
    { path: '/recipes', name: 'recipes', component: () => import('../pages/RecipesPage.vue') },
    { path: '/series', name: 'series', component: () => import('../pages/SeriesPage.vue') },
    { path: '/analyses', name: 'analyses', component: () => import('../pages/AnalysesPage.vue') },
    { path: '/audit', name: 'audit', component: () => import('../pages/AuditPage.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/vessels' },
  ],
})

router.beforeEach((to) => {
  const authenticated = Boolean(localStorage.getItem(tokenKey))
  if (!to.meta.public && !authenticated) return { name: 'login', query: { redirect: to.fullPath } }
  if (to.name === 'login' && authenticated) return { name: 'vessels' }
})

window.addEventListener('fermentation-auth-expired', () => router.push('/login'))
export default router
