import { describe, it, expect, vi } from 'vitest'
import type { UserCardInfo } from '../types'
import { useModals } from './useModals'

const USER: UserCardInfo = {
  account_id: '123456789',
  steam64: '76561198083722517',
  username: 'ZywOo_CS2',
  account_name: 'zywoo_official',
  remember_password: true,
  is_recent: false,
  avatar_url: '',
  notes: '',
}

function setup(selected: UserCardInfo | null = USER, selectedID = '123456789') {
  const showMessage = vi.fn()
  const logAction = vi.fn()
  const reloadVideoSummary = vi.fn(async () => undefined)
  const api = useModals({
    showMessage,
    logAction,
    selectedUser: () => selected,
    selectedAccountID: () => selectedID,
    reloadVideoSummary,
  })
  return { api, showMessage, logAction, reloadVideoSummary }
}

describe('useModals', () => {
  describe('画质弹窗', () => {
    it('初始关着，目标账号为空', () => {
      const { api } = setup()
      expect(api.videoVisible.value).toBe(false)
      expect(api.videoTargetAccountId.value).toBe('')
    })

    it('没传人时用当前选中账号', () => {
      const { api } = setup()
      api.openVideo()
      expect(api.videoTargetAccountId.value).toBe('123456789')
      expect(api.videoTargetUsername.value).toBe('ZywOo_CS2')
      expect(api.videoVisible.value).toBe(true)
    })

    it('显式传人时用传进来的那个', () => {
      const other: UserCardInfo = { ...USER, account_id: '999', username: 'donk' }
      const { api } = setup()
      api.openVideo(other)
      expect(api.videoTargetAccountId.value).toBe('999')
    })

    it('没有目标时静默返回，不弹窗也不报错', () => {
      const { api, showMessage } = setup(null)
      api.openVideo()
      expect(api.videoVisible.value).toBe(false)
      expect(showMessage).not.toHaveBeenCalled()
    })

    it('带提示的入口在没有选中账号时会提示', () => {
      const { api, showMessage } = setup(null)
      api.openVideoGuarded()
      expect(showMessage).toHaveBeenCalledWith('请先选择一个 Steam 账号', true)
      expect(api.videoVisible.value).toBe(false)
    })

    it('带提示的入口在有选中账号时直接开', () => {
      const { api, showMessage } = setup()
      api.openVideoGuarded()
      expect(api.videoVisible.value).toBe(true)
      expect(showMessage).not.toHaveBeenCalled()
    })

    it('保存后提示并重读画质摘要', async () => {
      const { api, showMessage, reloadVideoSummary } = setup()
      await api.handleVideoSaved()
      expect(showMessage).toHaveBeenCalledWith('CS2 视频配置已成功保存')
      expect(reloadVideoSummary).toHaveBeenCalledWith('123456789')
    })

    it('选中 game_cfg 时不重读摘要（它没有画质）', async () => {
      const { api, reloadVideoSummary } = setup(USER, 'game_cfg')
      await api.handleVideoSaved()
      expect(reloadVideoSummary).not.toHaveBeenCalled()
    })

    it('没选中任何账号时也不重读', async () => {
      const { api, reloadVideoSummary } = setup(null, '')
      await api.handleVideoSaved()
      expect(reloadVideoSummary).not.toHaveBeenCalled()
    })
  })

  describe('文本编辑器', () => {
    it('初始关着，作用域默认 user', () => {
      const { api } = setup()
      expect(api.textEditorVisible.value).toBe(false)
      expect(api.textEditorScope.value).toBe('user')
    })

    it('打开时带上文件名与作用域', () => {
      const { api } = setup()
      api.openTextEditor('autoexec.cfg', 'game')
      expect(api.textEditorVisible.value).toBe(true)
      expect(api.textEditorFile.value).toBe('autoexec.cfg')
      expect(api.textEditorScope.value).toBe('game')
    })

    it('模板里直接绑函数时会把事件对象传进来，要忽略掉', () => {
      const { api } = setup()
      const ev = { type: 'click' } as unknown as Event
      api.openTextEditor(ev)
      expect(api.textEditorFile.value).toBe('')
      expect(api.textEditorVisible.value).toBe(true)
    })

    it('从差异弹窗打开时，game_cfg 账号算游戏作用域', () => {
      const { api } = setup()
      api.openEditorFromDiff({ accountId: 'game_cfg', file: 'x.cfg' })
      expect(api.textEditorScope.value).toBe('game')
    })

    it('文件名以 game / csgo 开头的也算游戏作用域', () => {
      const { api } = setup()
      api.openEditorFromDiff({ accountId: '123', file: 'game/csgo/cfg/a.cfg' })
      expect(api.textEditorScope.value).toBe('game')

      api.openEditorFromDiff({ accountId: '123', file: 'csgo_x.cfg' })
      expect(api.textEditorScope.value).toBe('game')
    })

    it('其它文件算账号作用域', () => {
      const { api } = setup()
      api.openEditorFromDiff({ accountId: '123', file: 'cs2_user_keys.vcfg' })
      expect(api.textEditorScope.value).toBe('user')
    })

    it('保存后提示并写日志', () => {
      const { api, showMessage, logAction } = setup()
      api.handleTextEditorSaved('autoexec.cfg')
      expect(showMessage).toHaveBeenCalledWith('配置文件 [autoexec.cfg] 保存成功！')
      expect(logAction).toHaveBeenCalledWith('已在内置编辑器中修改并保存配置文件: autoexec.cfg')
    })

    it('关闭后可见标志复位', () => {
      const { api } = setup()
      api.openTextEditor('a.cfg')
      api.closeTextEditor()
      expect(api.textEditorVisible.value).toBe(false)
    })
  })
})
