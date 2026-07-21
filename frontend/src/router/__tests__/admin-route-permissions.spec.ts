import { describe, expect, it } from 'vitest'
import { routes } from '@/router'
import { ADMIN_ROUTE_PERMISSIONS } from '@/utils/adminPermissions'

describe('admin route permission metadata', () => {
  it('maps the OpenAI quota summary route to accounts:view', () => {
    const route = routes.find((item) => item.name === 'AdminOpenAIQuotaSummary')

    expect(route).toBeDefined()
    expect(ADMIN_ROUTE_PERMISSIONS.AdminOpenAIQuotaSummary).toEqual({
      resource: 'accounts',
      action: 'view'
    })
    expect(route?.meta?.adminResource).toBe('accounts')
    expect(route?.meta?.adminAction).toBe('view')
  })

  it('assigns an explicit registered capability to every admin view route', () => {
    const adminRoutes = routes.filter((route) => route.meta?.requiresAdmin === true)

    expect(adminRoutes).not.toHaveLength(0)
    for (const route of adminRoutes) {
      expect(typeof route.name).toBe('string')
      const routeName = route.name as string
      const permission = ADMIN_ROUTE_PERMISSIONS[routeName]
      expect(permission, `${routeName} needs an explicit permission`).toBeDefined()
      expect(route.meta?.adminResource).toBe(permission?.resource)
      expect(route.meta?.adminAction).toBe(permission?.action)
    }
  })
})
