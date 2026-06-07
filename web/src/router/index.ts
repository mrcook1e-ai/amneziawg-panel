import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login',    name: 'login',    component: () => import('@/views/LoginView.vue'), meta: { public: true } },
    { path: '/cabinet/:token', name: 'cabinet', component: () => import('@/views/CabinetView.vue'), meta: { public: true } },
    { path: '/',         name: 'clients',     component: () => import('@/views/ClientsView.vue') },
    { path: '/subscribers/:id', name: 'subscriber', component: () => import('@/views/SubscriberDetailView.vue') },
    { path: '/clients/:id',     name: 'client',     component: () => import('@/views/ClientDetailView.vue') },
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.ready) await auth.refresh()
  if (to.meta.public) return true
  if (auth.requiresPassword && !auth.authenticated) return { name: 'login', query: { to: to.fullPath } }
  return true
})

export default router
