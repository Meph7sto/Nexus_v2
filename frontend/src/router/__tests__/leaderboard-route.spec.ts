import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

import { routes } from '@/router'

const sidebarPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../components/layout/AppSidebar.vue')
const sidebarSource = readFileSync(sidebarPath, 'utf8')

describe('leaderboard route', () => {
  it('is a standard authenticated user route rather than an admin resource', () => {
    const route = routes.find((candidate) => candidate.name === 'Leaderboard')

    expect(route).toMatchObject({ path: '/leaderboard' })
    expect(route?.meta).toMatchObject({ requiresAuth: true, requiresAdmin: false })
    expect(route?.meta?.adminResource).toBeUndefined()
  })

  it('keeps the leaderboard in the shared user navigation', () => {
    expect(sidebarSource).toContain("{ path: '/leaderboard', label: t('nav.leaderboard'), icon: ChartIcon }")
  })
})
