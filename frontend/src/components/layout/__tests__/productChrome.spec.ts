import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const read = (path: string) => readFileSync(resolve(dir, path), 'utf8')

const headerSource = read('../AppHeader.vue')
const layoutSource = read('../AppLayout.vue')
const sidebarSource = read('../AppSidebar.vue')
const authLayoutSource = read('../AuthLayout.vue')
const homeSource = read('../../../views/HomeView.vue')
const keyUsageSource = read('../../../views/KeyUsageView.vue')
const packageSource = read('../../../../package.json')
const indexSource = read('../../../../index.html')
const enMiscSource = read('../../../i18n/locales/en/misc.ts')
const zhMiscSource = read('../../../i18n/locales/zh/misc.ts')

describe('Nexus product chrome', () => {
  it('uses the Nexus header geometry and design tokens', () => {
    expect(headerSource).toContain('rounded-md border border-[var(--nx-border)] bg-[var(--nx-bg)]')
    expect(headerSource).toContain('rounded-md bg-[var(--nx-text)]')
    expect(headerSource).toContain('text-[var(--nx-muted)]')
  })

  it('does not expose upstream repository links in public chrome', () => {
    for (const source of [headerSource, homeSource, keyUsageSource]) {
      expect(source).not.toContain('github.com/Wei-Shaw/sub2api')
      expect(source).not.toContain('githubUrl')
    }
  })

  it('keeps the Nexus PNG as the built-in product icon', () => {
    for (const source of [indexSource, sidebarSource, authLayoutSource, homeSource, keyUsageSource]) {
      expect(source).toContain('/logo.png')
      expect(source).not.toContain('/logo.svg')
    }
  })

  it('does not ship onboarding UI or its runtime dependency', () => {
    for (const source of [headerSource, layoutSource, enMiscSource, zhMiscSource]) {
      expect(source.toLowerCase()).not.toContain('onboarding')
    }
    expect(packageSource).not.toContain('driver.js')
  })
})
