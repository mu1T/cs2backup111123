import { describe, it, expect, vi, beforeEach } from 'vitest'

const h = vi.hoisted(() => ({
  getBackedUpAccounts: vi.fn(),
  previewBackupPrune: vi.fn(),
  applyBackupPrune: vi.fn(),
  getRemoteBackups: vi.fn(),
  backupUser: vi.fn(),
  restoreUser: vi.fn(),
  restoreSnapshot: vi.fn(),
  rollbackUserRestore: vi.fn(),
  hasUserRollback: vi.fn(),
  hasGameCFGRollback: vi.fn(),
}))

vi.mock('../utils/wails', () => h)

import { useBackup } from './useBackup'

function setup() {
  const showMessage = vi.fn()
  const logAction = vi.fn()
  const setBusy = vi.fn()
  const confirm = vi.fn()
  const reloadSnapshots = vi.fn(async () => undefined)
  const reloadAccountDetail = vi.fn(async () => undefined)
  const api = useBackup({ showMessage, logAction, setBusy, confirm, reloadSnapshots, reloadAccountDetail })
  return { api, showMessage, logAction, setBusy, confirm, reloadSnapshots, reloadAccountDetail }
}

describe('useBackup', () => {
  beforeEach(() => {
    Object.values(h).forEach((fn) => fn.mockReset())
  })

  describe('加载备份清单', () => {
    it('同时读本地备份账号与远程备份', async () => {
      h.getBackedUpAccounts.mockResolvedValue(['1', '2'])
      h.getRemoteBackups.mockResolvedValue([{ account_id: 'r1' }])
      const { api } = setup()

      await api.loadBackedUpAccounts()

      expect(api.backedUpAccounts.value).toEqual(['1', '2'])
      expect(api.remoteBackups.value).toHaveLength(1)
    })

    it('本地清单读取失败时保持原值，不清成 undefined', async () => {
      h.getBackedUpAccounts.mockResolvedValueOnce(['1'])
      h.getRemoteBackups.mockResolvedValue([])
      const { api } = setup()
      await api.loadBackedUpAccounts()

      h.getBackedUpAccounts.mockRejectedValueOnce(new Error('boom'))
      await api.loadBackedUpAccounts()

      expect(api.backedUpAccounts.value).toEqual(['1'])
    })

    it('远程备份读取失败时退回空数组', async () => {
      h.getBackedUpAccounts.mockResolvedValue([])
      h.getRemoteBackups.mockRejectedValue(new Error('boom'))
      const { api } = setup()

      await api.loadBackedUpAccounts()

      expect(api.remoteBackups.value).toEqual([])
    })
  })

  describe('撤销状态', () => {
    it('选了真实账号时查两条；选了 game_cfg 只查通用 CFG', async () => {
      h.hasUserRollback.mockResolvedValue(true)
      h.hasGameCFGRollback.mockResolvedValue(false)
      const { api } = setup()

      await api.checkRollbackStatus('123')
      expect(api.canRollbackUser.value).toBe(true)
      expect(api.canRollbackGameCFG.value).toBe(false)

      h.hasUserRollback.mockClear()
      await api.checkRollbackStatus('game_cfg')
      expect(h.hasUserRollback).not.toHaveBeenCalled()
      expect(api.canRollbackUser.value).toBe(false)
    })

    it('查询失败时置为 false，不把按钮留在可点状态', async () => {
      h.hasUserRollback.mockResolvedValueOnce(true)
      h.hasGameCFGRollback.mockResolvedValueOnce(true)
      const { api } = setup()
      await api.checkRollbackStatus('123')

      h.hasUserRollback.mockRejectedValueOnce(new Error('boom'))
      h.hasGameCFGRollback.mockRejectedValueOnce(new Error('boom'))
      await api.checkRollbackStatus('123')

      expect(api.canRollbackUser.value).toBe(false)
      expect(api.canRollbackGameCFG.value).toBe(false)
    })
  })

  describe('备份', () => {
    it('成功后重读备份清单与快照，并提示', async () => {
      h.backupUser.mockResolvedValue(undefined)
      h.getBackedUpAccounts.mockResolvedValue(['123'])
      h.getRemoteBackups.mockResolvedValue([])
      const { api, showMessage, reloadSnapshots, setBusy } = setup()

      await api.backup('123')

      expect(h.backupUser).toHaveBeenCalledWith('123')
      expect(api.backedUpAccounts.value).toEqual(['123'])
      expect(reloadSnapshots).toHaveBeenCalledWith('123')
      expect(showMessage).toHaveBeenCalledWith('用户 [123] 备份成功！')
      expect(setBusy).toHaveBeenLastCalledWith(false)
    })

    it('没选账号时提示，不调后端', async () => {
      const { api, showMessage } = setup()
      await api.backup('game_cfg')
      expect(showMessage).toHaveBeenCalledWith('请先选择要备份的账号', true)
      expect(h.backupUser).not.toHaveBeenCalled()
    })

    it('失败时报错文案带原因', async () => {
      h.backupUser.mockRejectedValue(new Error('disk full'))
      const { api, showMessage } = setup()

      await api.backup('123')

      expect(showMessage).toHaveBeenCalledWith('备份用户失败: disk full', true)
    })
  })

  describe('还原', () => {
    it('没选账号时不开还原弹窗', () => {
      const { api, showMessage } = setup()
      api.openRestoreSource('game_cfg')
      expect(showMessage).toHaveBeenCalledWith('请先选择要恢复的目标账号', true)
      expect(api.restoreSourceModalVisible.value).toBe(false)
    })

    it('选了账号才开弹窗', () => {
      const { api } = setup()
      api.openRestoreSource('123')
      expect(api.restoreSourceModalVisible.value).toBe(true)
    })

    it('来源是快照时调 restoreSnapshot', async () => {
      h.restoreSnapshot.mockResolvedValue(undefined)
      h.hasUserRollback.mockResolvedValue(true)
      h.hasGameCFGRollback.mockResolvedValue(false)
      const { api, reloadAccountDetail } = setup()

      await api.confirmRestore('123', { type: 'snapshot', sourceId: 'snap1' })

      expect(h.restoreSnapshot).toHaveBeenCalledWith('123', 'snap1')
      expect(reloadAccountDetail).toHaveBeenCalledWith('123')
      expect(api.restoreSourceModalVisible.value).toBe(false)
    })

    it('来源是其它账号时 RestoreUser(目标, 来源) 顺序不能反', async () => {
      h.restoreUser.mockResolvedValue(undefined)
      h.hasUserRollback.mockResolvedValue(true)
      h.hasGameCFGRollback.mockResolvedValue(false)
      const { api } = setup()

      await api.confirmRestore('target', { type: 'other', sourceId: 'source' })

      expect(h.restoreUser).toHaveBeenCalledWith('target', 'source')
    })

    it('来源是自己时目标与来源都是自己', async () => {
      h.restoreUser.mockResolvedValue(undefined)
      h.hasUserRollback.mockResolvedValue(true)
      h.hasGameCFGRollback.mockResolvedValue(false)
      const { api } = setup()

      await api.confirmRestore('123', { type: 'self', sourceId: '123' })

      expect(h.restoreUser).toHaveBeenCalledWith('123', '123')
    })

    it('还原失败时报错，弹窗已经关了（避免用户以为没点上）', async () => {
      h.restoreSnapshot.mockRejectedValue(new Error('archive corrupt'))
      const { api, showMessage } = setup()

      await api.confirmRestore('123', { type: 'snapshot', sourceId: 'snap1' })

      expect(showMessage).toHaveBeenCalledWith('还原配置失败: archive corrupt', true)
      expect(api.restoreSourceModalVisible.value).toBe(false)
    })
  })

  describe('撤销还原', () => {
    it('先弹确认框，不直接调后端', () => {
      const { api, confirm } = setup()
      api.rollbackUser('123')

      expect(confirm).toHaveBeenCalledTimes(1)
      expect(confirm.mock.calls[0][0]).toMatchObject({ isDanger: true, icon: 'restore' })
      expect(h.rollbackUserRestore).not.toHaveBeenCalled()
    })

    it('确认后才调后端，并重查撤销状态与账号摘要', async () => {
      h.rollbackUserRestore.mockResolvedValue(undefined)
      h.hasUserRollback.mockResolvedValue(false)
      h.hasGameCFGRollback.mockResolvedValue(false)
      const { api, confirm, reloadAccountDetail, showMessage } = setup()

      api.rollbackUser('123')
      await confirm.mock.calls[0][0].onConfirm()

      expect(h.rollbackUserRestore).toHaveBeenCalledWith('123')
      expect(reloadAccountDetail).toHaveBeenCalledWith('123')
      expect(showMessage).toHaveBeenCalledWith('账号 [123] 已成功回退至还原前的原始配置！')
      expect(api.canRollbackUser.value).toBe(false)
    })

    it('撤销失败时报错', async () => {
      h.rollbackUserRestore.mockRejectedValue(new Error('no backup'))
      const { api, confirm, showMessage } = setup()

      api.rollbackUser('123')
      await confirm.mock.calls[0][0].onConfirm()

      expect(showMessage).toHaveBeenCalledWith('撤销还原失败: no backup', true)
    })

    it('没选账号时什么都不做', () => {
      const { api, confirm } = setup()
      api.rollbackUser('game_cfg')
      expect(confirm).not.toHaveBeenCalled()
    })
  })
})

describe('useBackup 存量收敛', () => {
  beforeEach(() => {
    Object.values(h).forEach((fn) => fn.mockReset())
  })

  it('没超限时不弹任何东西', async () => {
    h.previewBackupPrune.mockResolvedValue([])
    const { api, confirm } = setup()

    await api.offerPrune('123', 10)

    expect(confirm).not.toHaveBeenCalled()
    expect(h.applyBackupPrune).not.toHaveBeenCalled()
  })

  it('超限时弹确认，文案里有保留数、删除数、以及会删哪些', async () => {
    h.previewBackupPrune.mockResolvedValue(['v1', 'v2', 'v3'])
    const { api, confirm } = setup()

    await api.offerPrune('123', 10)

    expect(confirm).toHaveBeenCalledTimes(1)
    const opts = confirm.mock.calls[0][0]
    expect(opts.message).toContain('13')  // 3 + 10
    expect(opts.message).toContain('10')
    expect(opts.detail).toContain('v1')
    expect(opts.isDanger).toBe(true)
  })

  it('确认后才真删，并重读备份清单', async () => {
    h.previewBackupPrune.mockResolvedValue(['v1', 'v2'])
    h.applyBackupPrune.mockResolvedValue(['v1', 'v2'])
    h.getBackedUpAccounts.mockResolvedValue([])
    h.getRemoteBackups.mockResolvedValue([])
    const { api, confirm, showMessage } = setup()

    await api.offerPrune('123', 10)
    await confirm.mock.calls[0][0].onConfirm()

    expect(h.applyBackupPrune).toHaveBeenCalledWith('123')
    expect(showMessage).toHaveBeenCalledWith('已清理 2 个旧版本')
  })

  it('同一个账号在一个会话里只问一次（点了取消就不再烦）', async () => {
    h.previewBackupPrune.mockResolvedValue(['v1'])
    const { api, confirm } = setup()

    await api.offerPrune('123', 10)
    await api.offerPrune('123', 10)

    expect(confirm).toHaveBeenCalledTimes(1)
  })

  it('game_cfg 和上限 0 都不检查', async () => {
    const { api, confirm } = setup()
    await api.offerPrune('game_cfg', 10)
    await api.offerPrune('123', 0)
    expect(h.previewBackupPrune).not.toHaveBeenCalled()
    expect(confirm).not.toHaveBeenCalled()
  })

  it('预览接口报错时静默返回，不打断用户', async () => {
    h.previewBackupPrune.mockRejectedValue(new Error('boom'))
    const { api, confirm } = setup()
    await expect(api.offerPrune('123', 10)).resolves.toBeUndefined()
    expect(confirm).not.toHaveBeenCalled()
  })
})
