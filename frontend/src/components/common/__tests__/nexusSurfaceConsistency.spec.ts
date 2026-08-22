import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '../../..')
const read = (path: string) => readFileSync(resolve(root, path), 'utf8')

describe('Nexus surface consistency', () => {
  it('uses the shared dialog shell for newly merged confirmation dialogs', () => {
    const backupSource = read('views/admin/BackupView.vue')
    const passkeySource = read('components/user/profile/ProfilePasskeyCard.vue')

    expect(backupSource).toContain('<BaseDialog')
    expect(backupSource).not.toContain('shadow-2xl')
    expect(passkeySource).toContain('<BaseDialog')
    expect(passkeySource).not.toContain('shadow-xl')
  })

  it('keeps generated media result cards at the standard radius', () => {
    const accountTestSource = read('components/admin/account/AccountTestModal.vue')

    expect(accountTestSource).not.toContain('rounded-xl')
  })
})
