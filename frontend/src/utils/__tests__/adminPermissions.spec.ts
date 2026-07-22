import { describe, expect, it } from 'vitest'
import {
  canAdmin,
  getFirstAllowedAdminRoute,
} from '@/utils/adminPermissions'

describe('admin permissions', () => {
  it('grants every registered capability to a super administrator', () => {
    expect(canAdmin(
      { role: 'super_admin', admin_permissions: [] },
      'settings',
      'update',
    )).toBe(true)
  })

  it('requires view together with a non-view limited-admin action', () => {
    expect(canAdmin(
      { role: 'admin', admin_permissions: [{ resource: 'users', actions: ['update'] }] },
      'users',
      'update',
    )).toBe(false)

    expect(canAdmin(
      { role: 'admin', admin_permissions: [{ resource: 'users', actions: ['view', 'update'] }] },
      'users',
      'update',
    )).toBe(true)
  })

  it('fails closed for missing grants and ordinary users', () => {
    expect(canAdmin({ role: 'admin', admin_permissions: [] }, 'users', 'view')).toBe(false)
    expect(canAdmin({ role: 'user', admin_permissions: [{ resource: 'users', actions: ['view'] }] }, 'users', 'view')).toBe(false)
  })

  it('chooses the first explicitly mapped page a limited administrator can view', () => {
    expect(getFirstAllowedAdminRoute({
      role: 'admin',
      admin_permissions: [{ resource: 'users', actions: ['view'] }],
    })).toBe('/admin/users')

    expect(getFirstAllowedAdminRoute({ role: 'admin', admin_permissions: [] })).toBe('/dashboard')
  })
})
