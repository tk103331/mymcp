import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    redirect: '/workspace'
  },
  {
    path: '/workspace',
    name: 'Workspace',
    component: () => import('../views/workspace/index.vue')
  },
  {
    path: '/instances',
    name: 'Instances',
    component: () => import('../views/instances/index.vue')
  },
  {
    path: '/search',
    name: 'Search',
    component: () => import('../views/search/index.vue')
  },
  {
    path: '/search/:name',
    name: 'ServiceDetail',
    component: () => import('../views/search/detail.vue')
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('../views/settings/index.vue')
  }
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes
})

export default router