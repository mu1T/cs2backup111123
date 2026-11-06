import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { SnapshotInfo } from '../types'

const h = vi.hoisted(() => ({
  getSnapshots: vi.fn(),
  createSnapshot: vi.fn(),
  restoreVersion: vi.fn(),
  deleteSnapshot: vi.fn(),
  compareSnapshot: vi.fn(),
}))

vi.mock('../utils/wails', () => h)

import { useSnapshots } from './useSnapshots'

const SNAP = { id: 'snap_1', note: '开黑前', created_at: '', total_files: 3, total_bytes: 100 } as unknown as SnapshotInfo

function setup() {
  const showMessage = vi.fn()
  const logAction = vi.fn()
  const setBusy = vi.fn()
  const confirm = vi.fn()
  const reloadBackedUpAccounts = vi.fn(async () => undefined)
  const reloadAccountDetail = vi.fn(async () => undefined)
  const api = useSnapshots({
    showMessage,
    logAction,
    setBusy,
    confirm,
    reloadBackedUpAccounts,
    reloadAccountDetail,
  })
  return { api, showMessage, logAction, setBusy, confirm, reloadBackedUpAccounts, reloadAccountDetail }
}

describe('useSnapshots', () => {
  beforeEach(() => {
    Object.values(h).forEach((fn) => fn.mockReset())
  })

  describe('加载与打开', () => {
    it('读到列表就写入', async () => {
      h.getSnapshots.mockResolvedValue([SNAP])
      const { api } = setup()

      await api.load('123')

      expect(api.list.value).toHaveLength(1)
      expect(api.loading.value).toBe(false)
    })

    it('game_cfg 与空 ID 清空列表，不问后端', async () => {
      const { api } = setup()
      await api.load('game_cfg')
      await api.load('')
      expect(h.getSnapshots).not.toHaveBeenCalled()
      expect(api.list.value).toEqual([])
    })

    it('读取失败时保留原列表，不清空', async () => {
      h.getSnapshots.mockResolvedValueOnce([SNAP])
      const { api } = setup()
      await api.load('123')

      h.getSnapshots.mockRejectedValueOnce(new Error('boom'))
      await api.load('123')

      expect(api.list.value).toHaveLength(1)
      expect(api.loading.value).toBe(false)
    })

    it('打开面板前会先刷一次列表', async () => {
      h.getSnapshots.mockResolvedValue([SNAP])
      const { api } = setup()

      await api.open('123')

      expect(h.getSnapshots).toHaveBeenCalledWith('123')
      expect(api.modalVisible.value).toBe(true)
    })

    it('没选账号时不打开面板', async () => {
      const { api, showMessage } = setup()
      await api.open('game_cfg')
      expect(showMessage).toHaveBeenCalledWith('请先选择一个 Steam 账号', true)
      expect(api.modalVisible.value).toBe(false)
    })
  })

  describe('新建', () => {
    it('用 trim 过的备注创建，成功后清空备注并重读列表与备份状态', async () => {
      h.createSnapshot.mockResolvedValue(SNAP)
      h.getSnapshots.mockResolvedValue([SNAP])
      const { api, showMessage, reloadBackedUpAccounts, setBusy } = setup()
      api.note.value = '  开黑前  '

      await api.create('123')

      expect(h.createSnapshot).toHaveBeenCalledWith('123', '开黑前')
      expect(api.note.value).toBe('')
      expect(reloadBackedUpAccounts).toHaveBeenCalledTimes(1)
      expect(showMessage).toHaveBeenCalledWith('成功为账号 [123] 创建快照 [snap_1]！')
      expect(setBusy).toHaveBeenLastCalledWith(false)
    })

    it('没填备注时传空串', async () => {
      h.createSnapshot.mockResolvedValue(SNAP)
      h.getSnapshots.mockResolvedValue([])
      const { api } = setup()

      await api.create('123')

      expect(h.createSnapshot).toHaveBeenCalledWith('123', '')
    })

    it('没选账号时提示，不调后端', async () => {
      const { api, showMessage } = setup()
      await api.create('game_cfg')
      expect(showMessage).toHaveBeenCalledWith('请先选择要创建快照的账号', true)
      expect(h.createSnapshot).not.toHaveBeenCalled()
    })

    it('失败时报错，备注保留（用户不用重打）', async () => {
      h.createSnapshot.mockRejectedValue(new Error('disk full'))
      const { api, showMessage } = setup()
      api.note.value = '开黑前'

      await api.create('123')

      expect(showMessage).toHaveBeenCalledWith('创建快照失败: disk full', true)
      expect(api.note.value).toBe('开黑前')
    })
  })

  describe('还原', () => {
    it('先弹确认框，不直接调后端', () => {
      const { api, confirm } = setup()
      api.restore('123', SNAP)

      expect(confirm).toHaveBeenCalledTimes(1)
      expect(confirm.mock.calls[0][0]).toMatchObject({ icon: 'restore', confirmText: '确认还原' })
      expect(h.restoreVersion).not.toHaveBeenCalled()
    })

    it('确认后才还原，并重读账号摘要', async () => {
      h.restoreVersion.mockResolvedValue(undefined)
      const { api, confirm, reloadAccountDetail, showMessage } = setup()

      api.restore('123', SNAP)
      await confirm.mock.calls[0][0].onConfirm()

      expect(h.restoreVersion).toHaveBeenCalledWith('123', 'snap_1')
      expect(reloadAccountDetail).toHaveBeenCalledWith('123')
      expect(showMessage).toHaveBeenCalledWith('快照 [snap_1] 已安全恢复至本地账号 [123]！')
    })

    it('还原失败时报错', async () => {
      h.restoreVersion.mockRejectedValue(new Error('corrupt'))
      const { api, confirm, showMessage } = setup()

      api.restore('123', SNAP)
      await confirm.mock.calls[0][0].onConfirm()

      expect(showMessage).toHaveBeenCalledWith('恢复快照失败: corrupt', true)
    })
  })

  describe('删除', () => {
    it('先确认，确认后才删并重读列表', async () => {
      h.deleteSnapshot.mockResolvedValue(undefined)
      h.getSnapshots.mockResolvedValue([])
      const { api, confirm, showMessage } = setup()

      api.remove('123', SNAP)
      expect(h.deleteSnapshot).not.toHaveBeenCalled()

      await confirm.mock.calls[0][0].onConfirm()

      expect(h.deleteSnapshot).toHaveBeenCalledWith('123', 'snap_1')
      expect(h.getSnapshots).toHaveBeenCalledWith('123')
      expect(showMessage).toHaveBeenCalledWith('快照 [snap_1] 已成功删除')
    })

    it('删除确认框标为危险操作', () => {
      const { api, confirm } = setup()
      api.remove('123', SNAP)
      expect(confirm.mock.calls[0][0]).toMatchObject({ isDanger: true, icon: 'danger' })
    })
  })

  describe('差异比对', () => {
    it('比对成功后开弹窗，日志里报变动数（identical 不算）', async () => {
      h.compareSnapshot.mockResolvedValue([
        { status: 'identical', path: 'a' },
        { status: 'modified', path: 'b' },
        { status: 'added', path: 'c' },
      ])
      const { api, logAction } = setup()

      await api.openDiff('123', SNAP)

      expect(api.diffVisible.value).toBe(true)
      expect(api.diffSnapshotId.value).toBe('snap_1')
      expect(api.diffItems.value).toHaveLength(3)
      expect(logAction).toHaveBeenLastCalledWith('比对完成，发现 2 处配置文件变动')
      expect(api.diffLoading.value).toBe(false)
    })

    it('比对失败时不开弹窗', async () => {
      h.compareSnapshot.mockRejectedValue(new Error('snapshot gone'))
      const { api, showMessage } = setup()

      await api.openDiff('123', SNAP)

      expect(api.diffVisible.value).toBe(false)
      expect(showMessage).toHaveBeenCalledWith('比对失败: snapshot gone', true)
      expect(api.diffLoading.value).toBe(false)
    })
  })
})
