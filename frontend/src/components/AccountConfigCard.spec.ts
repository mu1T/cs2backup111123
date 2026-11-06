import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountConfigCard from './AccountConfigCard.vue'
import type { UserCardInfo } from '../types'
import { emptyAccountSummary, type AccountSummary } from '../utils/accountSummary'

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

const mountCard = (props: Record<string, unknown> = {}, summary: Partial<AccountSummary> = {}) => {
  const base = emptyAccountSummary()
  return mount(AccountConfigCard, {
    props: {
      user: USER,
      summary: { video: { ...base.video, ...summary.video }, crosshair: { ...base.crosshair, ...summary.crosshair } },
      ...props,
    },
  })
}

describe('AccountConfigCard 账号信息', () => {
  it('展示昵称、AccountID 与 Steam64', () => {
    const wrapper = mountCard()
    expect(wrapper.find('.profile-username').text()).toBe('ZywOo_CS2')
    const ids = wrapper.find('.profile-ids-line').text()
    expect(ids).toContain('123456789')
    expect(ids).toContain('76561198083722517')
  })

  it('已备份与未备份的徽标互斥', () => {
    expect(mountCard({ isBackedUp: true }).find('.badge-status-pill').text()).toContain('已备份就绪')
    expect(mountCard({ isBackedUp: false }).find('.badge-status-pill').text()).toContain('未备份')
  })

  it('当前登录账号显示「当前登录」，否则显示切换按钮', () => {
    const recent = mountCard({ user: { ...USER, is_recent: true } })
    expect(recent.find('.badge-recent-pill').exists()).toBe(true)
    expect(recent.find('.btn-switch-pill').exists()).toBe(false)

    const other = mountCard()
    expect(other.find('.btn-switch-pill').text()).toContain('切换至此账号')
  })

  it('记住密码的账号切换按钮提示免密切换', () => {
    expect(mountCard().find('.btn-switch-pill').text()).toContain('免密切换至此账号')
    const plain = mountCard({ user: { ...USER, remember_password: false } })
    expect(plain.find('.btn-switch-pill').text()).toContain('切换至此账号')
    expect(plain.find('.badge-remember-pill').exists()).toBe(false)
  })
})

describe('AccountConfigCard 启动项与画质摘要', () => {
  it('未配置启动项时显示占位文案且复制按钮禁用', () => {
    const wrapper = mountCard({ launchOptions: '' })
    expect(wrapper.find('.strip-code').text()).toBe('未配置启动项')
    expect(wrapper.find('.launch-preview-strip .btn-chip-action').attributes('disabled')).toBeDefined()
  })

  it('只读锁状态与默认预设徽标跟随 props', () => {
    const locked = mountCard(
      {},
      { video: { ...emptyAccountSummary().video, readOnly: true, defaultPresetName: '竞技默认', defaultPresetApplied: true } },
    )
    const badges = locked.findAll('.video-lock-badge').map((b) => b.text())
    expect(badges[0]).toContain('已锁定只读')
    expect(badges[1]).toContain('已套用默认预设')

    const noPreset = mountCard({}, { video: { ...emptyAccountSummary().video, readOnly: false, defaultPresetName: null } })
    expect(noPreset.findAll('.video-lock-badge')).toHaveLength(1)
    expect(noPreset.find('.video-lock-badge').text()).toContain('未锁定')
  })

  it('没有默认预设时「套用默认预设」不可点', () => {
    const wrapper = mountCard({}, { video: { ...emptyAccountSummary().video, defaultPresetName: null } })
    const buttons = wrapper.findAll('.video-summary-actions .btn-chip-action')
    expect(buttons[1].attributes('disabled')).toBeDefined()
  })
})

describe('AccountConfigCard 事件', () => {
  it('复制类按钮抛出带载荷的事件', async () => {
    const wrapper = mountCard({ launchOptions: '-novid' })
    const idButtons = wrapper.findAll('.btn-icon-subtle')
    await idButtons[0].trigger('click')
    await idButtons[1].trigger('click')
    await wrapper.find('.launch-preview-strip .btn-chip-action').trigger('click')

    expect(wrapper.emitted('copy-account-id')?.[0]).toEqual(['123456789'])
    expect(wrapper.emitted('copy-steam64')?.[0]).toEqual(['76561198083722517'])
    expect(wrapper.emitted('copy-launch-options')).toHaveLength(1)
  })

  it('切换账号、锁定、套用预设各抛一次事件', async () => {
    const wrapper = mountCard({}, { video: { ...emptyAccountSummary().video, defaultPresetName: '竞技默认' } })
    await wrapper.find('.btn-switch-pill').trigger('click')
    const videoButtons = wrapper.findAll('.video-summary-actions .btn-chip-action')
    await videoButtons[0].trigger('click')
    await videoButtons[1].trigger('click')
    expect(wrapper.emitted('switch-account')).toHaveLength(1)
    expect(wrapper.emitted('toggle-readonly')).toHaveLength(1)
    expect(wrapper.emitted('apply-default-preset')).toHaveLength(1)
  })

  it('备份与还原按钮抛出事件，撤销还原按需出现', async () => {
    const wrapper = mountCard({ canRollback: true })
    const actions = wrapper.findAll('.account-actions-area > button')
    expect(actions).toHaveLength(3)
    await actions[0].trigger('click')
    await actions[1].trigger('click')
    await actions[2].trigger('click')
    expect(wrapper.emitted('backup')).toHaveLength(1)
    expect(wrapper.emitted('restore')).toHaveLength(1)
    expect(wrapper.emitted('rollback')).toHaveLength(1)

    expect(mountCard().findAll('.account-actions-area > button')).toHaveLength(2)
  })

  it('备份进行中三个动作按钮全部禁用', () => {
    const wrapper = mountCard({ actionLoading: true, canRollback: true })
    wrapper.findAll('.account-actions-area > button').forEach((b) =>
      expect(b.attributes('disabled')).toBeDefined()
    )
  })
})

describe('AccountConfigCard 头像兜底', () => {
  it('有头像时用真实头像', () => {
    const wrapper = mountCard({ user: { ...USER, avatar_url: 'data:image/png;base64,AAA' } })
    expect(wrapper.find('.profile-avatar-img').attributes('src')).toBe('data:image/png;base64,AAA')
  })

  it('没有头像时用内置默认头像，而不是首字母方块', () => {
    const wrapper = mountCard({ user: { ...USER, avatar_url: '' } })
    const img = wrapper.find('.profile-avatar-img')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toBeTruthy()
    // 首字母占位已经移除
    expect(wrapper.find('.profile-avatar-placeholder').exists()).toBe(false)
  })
})
