<script setup lang="ts">
/**
 * 当前账号配置卡片。
 *
 * 纯展示 + 事件上抛：选中账号、启动项文本、画质摘要、复制态、回滚态都由父级传入，
 * 备份 / 还原 / 切号等动作只抛事件，不碰绑定层。
 */
import { computed } from 'vue'
import SvgIcon from './SvgIcon.vue'
import defaultAvatar from '../assets/default_avatar.jpg'
import type { UserCardInfo } from '../types'
import { emptyAccountSummary, type AccountSummary } from '../utils/accountSummary'

const props = withDefaults(
  defineProps<{
    user: UserCardInfo
    isBackedUp?: boolean
    launchOptions?: string
    copySuccessId?: string | null
    copySuccessSteam64?: boolean
    copyOptionsSuccess?: boolean
    actionLoading?: boolean
    canRollback?: boolean
    /** 画质与准星两条摘要，字符串已在这一层拼好。 */
    summary?: AccountSummary
  }>(),
  {
    isBackedUp: false,
    launchOptions: '',
    copySuccessId: null,
    copySuccessSteam64: false,
    copyOptionsSuccess: false,
    summary: () => emptyAccountSummary(),
    actionLoading: false,
    canRollback: false,
  }
)

defineEmits<{
  (e: 'switch-account'): void
  (e: 'copy-account-id', accountId: string): void
  (e: 'copy-steam64', steam64: string): void
  (e: 'copy-launch-options'): void
  (e: 'toggle-readonly'): void
  (e: 'apply-default-preset'): void
  (e: 'backup'): void
  (e: 'restore'): void
  (e: 'rollback'): void
  (e: 'open-crosshair'): void
}>()

/**
 * 模板里只需要「有没有默认预设」和「它叫什么」。
 * 名字由 summary 带过来，这里只是把它包成对象，方便 v-if 与插值共用。
 */
const defaultPreset = computed(() => {
  const name = props.summary.video.defaultPresetName
  return name ? { name } : null
})
</script>

<template>
  <div class="winui-card card-account-config">
    <div class="card-header">
      <div class="card-header-left">
        <h3 class="card-title">
          <SvgIcon name="user" size="16" class="text-accent" />
          <span>当前账号配置</span>
        </h3>
        <p class="card-desc">管理选中账号的准星、按键、视频及启动项配置</p>
      </div>
      <div class="card-header-right">
        <span v-if="isBackedUp" class="badge-status-pill badge-backed" title="本地已有该账号备份">
          <SvgIcon name="check" size="11" />
          <span>已备份就绪</span>
        </span>
        <span v-else class="badge-status-pill badge-unbacked" title="本地尚未建立备份">
          <SvgIcon name="alert-triangle" size="11" />
          <span>未备份</span>
        </span>
      </div>
    </div>

    <!-- Focused user display info -->
    <div class="account-profile-row">
      <div class="profile-avatar-wrap">
        <!-- 没有头像的账号（多为没设置社区资料的小号）统一用内置默认头像，
             比只显示一个首字母更容易辨认。 -->
        <img
          :src="user.avatar_url || defaultAvatar"
          :alt="user.username"
          class="profile-avatar-img"
        />
        <span v-if="user.is_recent" class="profile-avatar-dot" title="当前登录账号"></span>
      </div>

      <div class="profile-meta-col">
        <div class="profile-name-line">
          <span class="profile-username">{{ user.username || '未知玩家' }}</span>
          <span v-if="user.is_recent" class="badge-recent-pill">当前登录</span>
          <button
            v-else
            class="btn-switch-pill"
            :class="{ 'btn-switch-quick': user.remember_password }"
            :title="user.remember_password ? '已在本地记住密码凭据，点击一键免密直接启动切换' : '切换 Steam 登录为此账号'"
            @click="$emit('switch-account')"
          >
            <SvgIcon :name="user.remember_password ? 'zap' : 'user'" size="12" />
            <span>{{ user.remember_password ? '免密切换至此账号' : '切换至此账号' }}</span>
          </button>
          <span
            v-if="user.remember_password"
            class="badge-remember-pill"
            title="loginusers.vdf 中已记住密码"
          >
            <SvgIcon name="key" size="11" />
            <span>存有登录凭据</span>
          </span>
        </div>
        <div class="profile-ids-line">
          <span class="id-item mono-font">
            <span class="id-label">SteamID:</span>
            <span>{{ user.account_id }}</span>
            <button
              class="btn-icon-subtle"
              :title="copySuccessId === user.account_id ? '已复制' : '复制 AccountID'"
              @click="$emit('copy-account-id', user.account_id)"
            >
              <SvgIcon :name="copySuccessId === user.account_id ? 'check' : 'copy'" size="11" />
            </button>
          </span>
          <span class="id-divider">|</span>
          <span class="id-item mono-font">
            <span class="id-label">Steam64:</span>
            <span>{{ user.steam64 }}</span>
            <button
              class="btn-icon-subtle"
              :title="copySuccessSteam64 ? '已复制' : '复制 SteamID64'"
              @click="$emit('copy-steam64', user.steam64)"
            >
              <SvgIcon :name="copySuccessSteam64 ? 'check' : 'copy'" size="11" />
            </button>
          </span>
        </div>
      </div>
    </div>

    <!-- Launch Options preview row -->
    <div class="launch-preview-strip">
      <div class="strip-label">
        <SvgIcon name="terminal" size="13" class="text-accent" />
        <span>启动项:</span>
      </div>
      <div class="strip-code mono-font" :title="launchOptions || '未配置启动项'">
        {{ launchOptions || '未配置启动项' }}
      </div>
      <div class="strip-actions">
        <button
          class="btn-chip-action"
          :disabled="!launchOptions"
          title="复制启动项"
          @click="$emit('copy-launch-options')"
        >
          <SvgIcon :name="copyOptionsSuccess ? 'check' : 'copy'" size="12" />
          <span>{{ copyOptionsSuccess ? '已复制' : '复制' }}</span>
        </button>
      </div>
    </div>

    <!-- 画质摘要条：分辨率 / 刷新率 / 只读锁定 / 默认预设 -->
    <div class="video-summary-strip">
      <div class="video-summary-meta">
        <SvgIcon name="monitor" size="13" class="text-accent" />
        <span class="video-metric mono-font" :title="summary.video.resolutionText">
          {{ summary.video.resolutionText || '未检测到配置' }}
        </span>
        <span class="metric-divider">·</span>
        <span class="video-metric mono-font">{{ summary.video.refreshRateText || '—' }}</span>
        <span
          class="video-lock-badge"
          :class="{ locked: summary.video.readOnly }"
          :title="summary.video.readOnly ? 'cs2_video.txt 已被锁定为只读，游戏不会重置画质' : 'cs2_video.txt 未加只读锁，游戏可能重置画质'"
        >
          <SvgIcon :name="summary.video.readOnly ? 'lock' : 'unlock'" size="11" />
          <span>{{ summary.video.readOnly ? '已锁定只读' : '未锁定' }}</span>
        </span>
        <span
          v-if="defaultPreset"
          class="video-lock-badge"
          :class="{ locked: summary.video.defaultPresetApplied }"
          :title="summary.video.defaultPresetApplied ? `当前画质与默认预设「${defaultPreset.name}」一致` : `当前画质与默认预设「${defaultPreset.name}」不一致`"
        >
          <SvgIcon :name="summary.video.defaultPresetApplied ? 'check' : 'alert-triangle'" size="11" />
          <span>{{ summary.video.defaultPresetApplied ? '已套用默认预设' : '未套用默认预设' }}</span>
        </span>
      </div>
      <div class="video-summary-actions">
        <button
          class="btn-chip-action"
          :disabled="actionLoading || summary.video.loading"
          :title="summary.video.readOnly ? '解除 cs2_video.txt 只读属性' : '将 cs2_video.txt 设为只读，防止游戏重置画质'"
          @click="$emit('toggle-readonly')"
        >
          <SvgIcon :name="summary.video.readOnly ? 'unlock' : 'lock'" size="12" />
          <span>{{ summary.video.readOnly ? '解锁' : '锁定' }}</span>
        </button>
        <button
          class="btn-chip-action"
          :disabled="actionLoading || !defaultPreset"
          :title="defaultPreset ? `套用默认预设：${defaultPreset.name}` : '尚未配置默认视频预设'"
          @click="$emit('apply-default-preset')"
        >
          <SvgIcon name="download" size="12" />
          <span>套用默认预设</span>
        </button>
      </div>
    </div>

    <!-- 准星摘要条：样式 / 尺寸 / 分享码 + 打开编辑器 -->
    <div class="crosshair-summary-strip">
      <div class="crosshair-summary-meta">
        <SvgIcon name="sliders" size="13" class="text-accent" />
        <span class="crosshair-metric" :title="summary.crosshair.text">
          {{ summary.crosshair.text || '未检测到准星设置' }}
        </span>
        <span
          v-if="summary.crosshair.shareCode"
          class="crosshair-code mono-font"
          :title="summary.crosshair.shareCode"
        >
          {{ summary.crosshair.shareCode.slice(0, 14) }}…
        </span>
        <span
          class="crosshair-toggle-badge"
          :class="{ off: summary.crosshair.toggleOff }"
          :title="'准星显示开关 crosshair 由 autoexec.cfg 决定，工具只读取不改写'"
        >
          <SvgIcon :name="summary.crosshair.toggleOff ? 'eye' : 'check'" size="11" />
          <span>{{ summary.crosshair.toggleLabel }}</span>
        </span>
      </div>
      <div class="crosshair-summary-actions">
        <button
          class="btn-chip-action"
          :disabled="actionLoading"
          title="打开准星编辑器：调参数、导入分享码、一键写入 autoexec.cfg"
          @click="$emit('open-crosshair')"
        >
          <SvgIcon name="sliders" size="12" />
          <span>调整准星</span>
        </button>
      </div>
    </div>

    <!-- Core actions area: prominent buttons -->
    <div class="account-actions-area">
      <button
        class="btn-winui-primary btn-action-prominent"
        :disabled="actionLoading"
        title="一键流式备份当前选中账号配置"
        @click="$emit('backup')"
      >
        <SvgIcon name="backup" size="18" />
        <span>{{ actionLoading ? '正在备份...' : '备份此账号' }}</span>
      </button>
      <button
        class="btn-winui-secondary btn-action-prominent"
        :disabled="actionLoading"
        title="选择还原来源（当前账号备份、跨账号克隆、历史快照恢复）"
        @click="$emit('restore')"
      >
        <SvgIcon name="restore" size="18" />
        <span>{{ actionLoading ? '正在还原...' : '还原此备份...' }}</span>
      </button>
      <button
        v-if="canRollback"
        class="btn-winui-danger-subtle btn-action-prominent"
        :disabled="actionLoading"
        title="撤销上一次还原，回退至还原前自动备份的原始状态"
        @click="$emit('rollback')"
      >
        <SvgIcon name="refresh" size="16" />
        <span>撤销还原</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
/* 卡片容器的背景/描边由父级 .winui-card.card-account-config 控制（scoped 作用于子组件根元素）。 */
.btn-switch-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 9px;
  border-radius: var(--radius-full);
  background: var(--tone-info-weak);
  border: 1px solid rgba(96, 205, 255, 0.35);
  color: var(--accent-light);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: var(--transition-fast);
}
.btn-switch-pill:hover {
  background: rgba(96, 205, 255, 0.22);
  border-color: var(--accent-color);
  transform: translateY(-1px);
}
.badge-status-pill {
  font-size: 12px;
  font-weight: 600;
  padding: 2px 7px;
  border-radius: 12px;
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.account-profile-row {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 6px;
}
.profile-avatar-wrap {
  position: relative;
  width: 48px;
  height: 48px;
  flex-shrink: 0;
}
.profile-avatar-img {
  width: 100%;
  height: 100%;
  border-radius: 6px;
  object-fit: cover;
  border: 1px solid var(--stroke-2);
}
.profile-avatar-dot {
  position: absolute;
  top: -2px;
  right: -2px;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background-color: #10b981;
  border: 2px solid #1a1a1a;
}
.profile-meta-col {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.profile-name-line {
  display: flex;
  align-items: center;
  gap: 8px;
}
.profile-username {
  font-size: 15px;
  font-weight: 600;
  color: #ffffff;
}
.badge-recent-pill {
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
  color: #34d399;
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.3);
  padding: 1px 5px;
  border-radius: 10px;
}
.profile-ids-line {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.6);
}
.id-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.id-label {
  color: rgba(255, 255, 255, 0.4);
}
.id-divider {
  color: rgba(255, 255, 255, 0.2);
}
.btn-icon-subtle {
  background: none;
  border: none;
  padding: 2px 4px;
  color: rgba(255, 255, 255, 0.5);
  cursor: pointer;
  border-radius: 4px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.btn-icon-subtle:hover {
  color: #ffffff;
  background: var(--stroke-2);
}
.launch-preview-strip {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.25);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 6px;
  font-size: 12px;
}
.strip-label {
  display: flex;
  align-items: center;
  gap: 5px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.85);
  flex-shrink: 0;
}
.strip-code {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #93c5fd;
}
.strip-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.btn-chip-action {
  height: 26px;
  padding: 0 7px;
  font-size: 12px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border-radius: 4px;
  background: var(--surface-4);
  border: 1px solid var(--stroke-2);
  color: #ffffff;
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-chip-action:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.14);
}
.btn-chip-action:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.video-summary-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 6px 12px;
  background: rgba(0, 0, 0, 0.18);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-control);
  font-size: 12px;
  flex-wrap: wrap;
}
/* 准星摘要条：与画质摘要条同构，只是配色换到强调色系，便于区分两类设置。 */
.crosshair-summary-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 6px 12px;
  background: rgba(0, 0, 0, 0.18);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-control);
  font-size: 12px;
  flex-wrap: wrap;
}
.crosshair-summary-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex-wrap: wrap;
}
.crosshair-metric {
  color: var(--color-text);
  font-size: 12px;
}
.crosshair-code {
  font-size: 11px;
  color: #93c5fd;
  background: rgba(147, 197, 253, 0.1);
  border-radius: var(--radius-sm);
  padding: 1px 6px;
}
.crosshair-toggle-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 20px;
  padding: 0 6px;
  border-radius: var(--radius-pill);
  font-size: 11px;
  color: var(--color-text-muted);
  background: rgba(255, 255, 255, 0.05);
}
.crosshair-toggle-badge.off {
  color: var(--level-warning);
  background: var(--status-warning-fill);
}
.crosshair-summary-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.video-summary-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex-wrap: wrap;
}
.video-metric {
  color: var(--color-text);
  font-size: 12px;
}
.metric-divider {
  color: var(--color-text-muted);
}
.video-lock-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 20px;
  padding: 0 6px;
  border-radius: var(--radius-pill);
  font-size: 12px;
  color: var(--color-text-muted);
  background: var(--bg-control);
  border: 1px solid var(--border-control);
}
.video-lock-badge.locked {
  color: #b6e8af;
  background: var(--status-success-fill);
  border-color: var(--status-success-stroke);
}
.video-summary-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.account-actions-area {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 4px;
}
.account-actions-area > :nth-child(3):last-child {
  grid-column: 1 / -1;
}
.btn-action-prominent {
  height: var(--control-height-lg, 36px);
  border-radius: var(--radius-md, 6px);
  font-size: 14px;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
  transition: all 0.15s ease;
}

/* 矮窗口收尾覆盖：必须放在基础规则之后才能生效 */
@media (max-height: 760px) {
  .video-summary-strip {
    padding: 4px 10px;
  }

  .btn-action-prominent {
    height: 36px;
  }
}
</style>
