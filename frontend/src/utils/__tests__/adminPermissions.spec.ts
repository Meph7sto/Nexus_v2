import { describe, expect, it } from 'vitest'
import {
  canAdmin,
  getFirstAllowedAdminRoute,
} from '@/utils/adminPermissions'

describe('admin permissions', () => {
  it('reserves plugin management for super administrators', () => {
    expect(canAdmin({ role: 'super_admin' }, 'plugins', 'execute')).toBe(true)
    expect(canAdmin({ role: 'admin', admin_permissions: [{ resource: 'plugins', actions: ['view', 'execute'] }] }, 'plugins', 'execute')).toBe(false)
    expect(canAdmin({ role: 'user' }, 'plugins', 'view')).toBe(false)
  })
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

  it('treats monthly ledger as an independently grantable resource', () => {
    const principal = {
      role: 'admin' as const,
      admin_permissions: [{ resource: 'monthly_ledger' as const, actions: ['view', 'create'] as const }],
    }

    expect(canAdmin(principal, 'monthly_ledger', 'view')).toBe(true)
    expect(canAdmin(principal, 'monthly_ledger', 'create')).toBe(true)
    expect(canAdmin(principal, 'monthly_ledger', 'update')).toBe(false)
  })

  it('chooses the first explicitly mapped page a limited administrator can view', () => {
    expect(getFirstAllowedAdminRoute({
      role: 'admin',
      admin_permissions: [{ resource: 'users', actions: ['view'] }],
    })).toBe('/admin/users')

    expect(getFirstAllowedAdminRoute({ role: 'admin', admin_permissions: [] })).toBe('/dashboard')
  })
})
