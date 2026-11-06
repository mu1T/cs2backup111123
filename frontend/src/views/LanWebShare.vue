<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { formatSize } from '../utils/format'
import type { SharedBackupItem } from '../types'
import SvgIcon from '../components/SvgIcon.vue'
import appIcon from '../assets/app_icon.ico?url'

interface ServerInfo {
  name: string
  pin_enabled: boolean
  app_version: string
  port: number
  shared_count: number
}

const loading = ref(true)
const serverInfo = ref<ServerInfo>({
  name: 'CS2-LAN-Server',
  pin_enabled: false,
  app_version: '2.0.0',
  port: 8765,
  shared_count: 0,
})

const isAuthenticated = ref(false)
const pinInput = ref('')
const pinError = ref('')
const authSubmitting = ref(false)
const authToken = ref('')

const backups = ref<SharedBackupItem[]>([])
const loadingBackups = ref(false)


const formatDate = (isoStr: string): string => {
  if (!isoStr) return '-'
  try {
    const d = new Date(isoStr)
    return d.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return isoStr
  }
}

const fetchServerInfo = async () => {
  try {
    loading.value = true
    const res = await fetch('/api/lan/info')
    if (res.ok) {
      serverInfo.value = await res.json()
      if (!serverInfo.value.pin_enabled) {
        isAuthenticated.value = true
        await fetchBackups()
      } else {
        // Try fetching backups with cookie to see if already authorized
        await checkExistingAuth()
      }
    }
  } catch (err) {
    console.warn('Failed to fetch /api/lan/info, using defaults', err)
    serverInfo.value = {
      name: '局域网 CS2 配置共享节点',
      pin_enabled: false,
      app_version: '2.0.0',
      port: 8765,
      shared_count: 0,
    }
    isAuthenticated.value = true
  } finally {
    loading.value = false
  }
}

const checkExistingAuth = async () => {
  try {
    const res = await fetch('/api/lan/backups')
    if (res.ok) {
      const data = await res.json()
      backups.value = data.backups || []
      isAuthenticated.value = true
    }
  } catch {
    // Requires PIN
  }
}

const handleVerifyPIN = async () => {
  if (!pinInput.value || pinInput.value.trim().length === 0) {
    pinError.value = '请输入 6 位 PIN 码'
    return
  }

  try {
    authSubmitting.value = true
    pinError.value = ''
    const res = await fetch('/api/lan/auth', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ pin: pinInput.value.trim() }),
    })

    const data = await res.json()
    if (res.ok && data.success) {
      authToken.value = data.token || ''
      isAuthenticated.value = true
      await fetchBackups()
    } else {
      pinError.value = data.message || 'PIN 码错误，请重新输入'
    }
  } catch (err: any) {
    pinError.value = '网络连接异常，无法完成认证'
  } finally {
    authSubmitting.value = false
  }
}

const fetchBackups = async () => {
  try {
    loadingBackups.value = true
    const headers: Record<string, string> = {}
    if (authToken.value) {
      headers['Authorization'] = `Bearer ${authToken.value}`
    }

    const res = await fetch('/api/lan/backups', { headers })
    if (res.ok) {
      const data = await res.json()
      backups.value = data.backups || []
    }
  } catch (err) {
    console.warn('Failed to fetch /api/lan/backups', err)
  } finally {
    loadingBackups.value = false
  }
}

const getDownloadUrl = (backupId: string): string => {
  let url = `/api/lan/download/${backupId}`
  if (authToken.value) {
    url += `?token=${encodeURIComponent(authToken.value)}`
  }
  return url
}

// ---- 下载状态：进度 / 失败重试 ----
const downloadingId = ref<string | null>(null)
const downloadProgress = ref(0)
const downloadErrorId = ref<string | null>(null)
const downloadErrorMsg = ref('')

const saveBlob = (blob: Blob, filename: string) => {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  setTimeout(() => URL.revokeObjectURL(url), 4000)
}

const handleDownload = async (item: SharedBackupItem) => {
  if (downloadingId.value) return
  downloadingId.value = item.id
  downloadProgress.value = 0
  downloadErrorId.value = null
  downloadErrorMsg.value = ''

  try {
    const res = await fetch(getDownloadUrl(item.id))
    if (!res.ok) {
      if (res.status === 401 || res.status === 403) {
        throw new Error('访问凭证已失效，请刷新页面后重新输入 PIN 码')
      }
      throw new Error(`服务器返回 ${res.status}，备份包可能已被移除`)
    }

    const total = Number(res.headers.get('content-length') || 0)
    const filename = item.filename || `cs2-backup-${item.id}.zip`

    if (res.body && typeof res.body.getReader === 'function') {
      const reader = res.body.getReader()
      const chunks: Uint8Array[] = []
      let received = 0
      for (;;) {
        const { done, value } = await reader.read()
        if (done) break
        if (value) {
          chunks.push(value)
          received += value.length
          if (total > 0) {
            downloadProgress.value = Math.min(99, Math.round((received / total) * 100))
          }
        }
      }
      saveBlob(new Blob(chunks as BlobPart[], { type: 'application/zip' }), filename)
    } else {
      saveBlob(await res.blob(), filename)
    }

    downloadProgress.value = 100
  } catch (err: any) {
    downloadErrorId.value = item.id
    downloadErrorMsg.value = err?.message || '下载失败，请检查网络后重试'
  } finally {
    downloadingId.value = null
  }
}

onMounted(() => {
  fetchServerInfo()
})
</script>

<template>
  <div class="lan-web-container">
    <!-- Navbar / Header -->
    <header class="web-header">
      <div class="brand">
        <div class="logo-badge">
          <img :src="appIcon" alt="CS2 个人配置备份工具" class="logo-img" />
        </div>
        <div class="title-group">
          <h1 class="brand-title">CS2 个人配置备份工具</h1>
          <span class="host-tag">
            <span class="status-dot"></span>
            <span class="host-version">v{{ serverInfo.app_version }}</span>
            <span class="host-sep">·</span>
            <span class="host-node">共享节点 {{ serverInfo.name }}</span>
          </span>
        </div>
      </div>
      <div class="header-actions" v-if="isAuthenticated">
        <button class="btn btn-refresh" @click="fetchBackups" :disabled="loadingBackups">
          <SvgIcon name="refresh" :size="14" :class="{ spinning: loadingBackups }" />
          <span>刷新列表</span>
        </button>
      </div>
    </header>

    <!-- Main Content Area -->
    <main class="web-main">
      <!-- Loading State -->
      <div v-if="loading" class="center-box">
        <div class="spinner"></div>
        <p>正在连接局域网共享服务...</p>
      </div>

      <!-- PIN Verification Screen -->
      <div v-else-if="!isAuthenticated" class="auth-wrapper">
        <div class="auth-card">
          <div class="auth-icon-circle">
            <SvgIcon name="lock" :size="28" />
          </div>
          <h2 class="auth-title">PIN 码验证</h2>
          <p class="auth-desc">
            当前主机已开启访问保护，请输入共享方展示的 <strong>6 位 PIN 码</strong>。
          </p>

          <form @submit.prevent="handleVerifyPIN" class="auth-form">
            <div class="input-group">
              <input
                v-model="pinInput"
                type="text"
                maxlength="6"
                pattern="[0-9]*"
                placeholder="请输入 6 位数字 PIN"
                class="pin-input"
                autofocus
                :disabled="authSubmitting"
              />
            </div>

            <div v-if="pinError" class="error-msg">
              <SvgIcon name="alert-triangle" :size="14" /> {{ pinError }}
            </div>

            <button type="submit" class="btn btn-submit" :disabled="authSubmitting || pinInput.length < 4">
              <span v-if="authSubmitting">验证中...</span>
              <span v-else>立即验证并访问</span>
            </button>
          </form>

          <div class="auth-hint">
            <span><SvgIcon name="shield" :size="13" /> 连续输错 5 次将临时锁定 30 秒。</span>
          </div>
        </div>
      </div>

      <!-- Authenticated Backups View -->
      <div v-else class="backups-view">
        <div class="view-banner">
          <div class="banner-left">
            <h2><SvgIcon name="archive" :size="20" /> 可下载的配置备份</h2>
            <p>点击下载 CS2 配置压缩包。解压后运行包内的 <code>restore.bat</code> 即可快速恢复。</p>
          </div>
          <div class="banner-right">
            <div class="count-pill">
              共 <strong>{{ backups.length }}</strong> 项备份
            </div>
          </div>
        </div>

        <!-- Empty State -->
        <div v-if="backups.length === 0" class="empty-box">
          <div class="empty-icon">
            <SvgIcon name="folder" :size="48" />
          </div>
          <h3>当前主机未共享任何备份</h3>
          <p>请联系主机管理员在客户端控制台中勾选要共享的账号或 ZIP 备份包。</p>
        </div>

        <!-- Backups Grid -->
        <div v-else class="backups-grid">
          <div v-for="item in backups" :key="item.id" class="backup-card">
            <div class="card-top">
              <div class="card-icon">
                <SvgIcon name="monitor" :size="24" />
              </div>
              <div class="card-heading">
                <h3 class="card-title">{{ item.backup_name || item.filename }}</h3>
                <span class="card-sub" v-if="item.account_id">Steam 账号 ID: {{ item.account_id }}</span>
                <span class="card-sub" v-else>{{ item.filename }}</span>
              </div>
            </div>

            <!-- Tags -->
            <div class="card-tags">
              <span v-if="item.contains_userdata" class="tag tag-userdata"><SvgIcon name="check" :size="12" /> 准星/键位 (730)</span>
              <span v-if="item.contains_cfg" class="tag tag-cfg"><SvgIcon name="check" :size="12" /> 游戏 CFG</span>
              <span class="tag tag-type">{{ item.is_local ? '本地实时同步' : '独立备份包' }}</span>
            </div>

            <!-- Meta details -->
            <div class="card-meta">
              <div class="meta-item">
                <span class="meta-label">打包体积:</span>
                <span class="meta-val font-mono">{{ formatSize(item.size) }}</span>
              </div>
              <div class="meta-item">
                <span class="meta-label">备份时间:</span>
                <span class="meta-val">{{ formatDate(item.created_at) }}</span>
              </div>
            </div>

            <!-- Download Action -->
            <div class="card-action">
              <button
                type="button"
                class="btn btn-download"
                :class="{ 'is-error': downloadErrorId === item.id }"
                :disabled="downloadingId !== null && downloadingId !== item.id"
                :aria-busy="downloadingId === item.id"
                @click="handleDownload(item)"
              >
                <span v-if="downloadingId === item.id" class="dl-inline">
                  <span class="spinner-tiny"></span>
                  <span>正在下载 {{ downloadProgress }}%</span>
                </span>
                <span v-else-if="downloadErrorId === item.id" class="dl-inline">
                  <SvgIcon name="refresh" :size="15" />
                  <span>下载失败，点击重试</span>
                </span>
                <span v-else class="dl-inline">
                  <SvgIcon name="download" :size="15" />
                  <span>一键下载备份包 (含 restore.bat)</span>
                </span>
              </button>

              <div v-if="downloadingId === item.id" class="dl-progress" role="progressbar" :aria-valuenow="downloadProgress" aria-valuemin="0" aria-valuemax="100">
                <span class="dl-progress-bar" :style="{ width: downloadProgress + '%' }"></span>
              </div>

              <p v-if="downloadErrorId === item.id" class="dl-error">
                <SvgIcon name="alert-triangle" :size="13" />
                <span>{{ downloadErrorMsg }}</span>
              </p>
            </div>
          </div>
        </div>

        <!-- Restore Instruction Box -->
        <div class="restore-guide-card">
          <div class="guide-header">
            <span class="guide-icon">
              <SvgIcon name="info" :size="16" />
            </span>
            <h4>客户端恢复提示与帮助</h4>
          </div>
          <ol class="guide-steps">
            <li>下载完成后解压 ZIP 压缩文件至任意目录（如桌面）。</li>
            <li>确保已安装 CS2，直接双击运行解压出的 <code>restore.bat</code>。</li>
            <li>脚本将自动检测您的 Steam 安装盘符并一键部署 CFG 与 Userdata 配置。</li>
          </ol>
        </div>
      </div>
    </main>

    <!-- Footer -->
    <footer class="web-footer">
      <span>CS2 个人配置备份工具共享互联服务</span>
    </footer>
  </div>
</template>

<style scoped>
.lan-web-container {
  height: 100vh;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  display: flex;
  flex-direction: column;
  background-color: var(--bg-app);
  color: #e2e8f0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
}

.web-header {
  height: 68px;
  background-color: var(--bg-surface);
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 32px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 14px;
}

.logo-badge {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  overflow: hidden;
  flex-shrink: 0;
}

.logo-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  display: block;
}

.host-version {
  font-variant-numeric: tabular-nums;
}

.host-sep {
  opacity: 0.5;
}

.title-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.brand-title {
  font-size: 17px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
}

.host-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: #22c55e;
  box-shadow: none;
}

.web-main {
  flex: 1;
  padding: 32px;
  max-width: 1180px;
  width: 100%;
  margin: 0 auto;
}

.center-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  min-height: 350px;
  color: var(--color-text-secondary);
}

.spinner {
  width: 40px;
  height: 40px;
  border: 3px solid var(--accent-subtle);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.spinning {
  animation: spin 0.8s linear infinite;
}

/* Auth Card */
.auth-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 450px;
}

.auth-card {
  width: 100%;
  max-width: 440px;
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  padding: 36px 32px;
  text-align: center;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.4);
}

.auth-icon-circle {
  width: 58px;
  height: 58px;
  border-radius: 50%;
  background: var(--accent-subtle);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 26px;
  margin: 0 auto 16px;
}

.auth-title {
  font-size: 20px;
  font-weight: 700;
  color: var(--color-text);
  margin-bottom: 8px;
}

.auth-desc {
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.5;
  margin-bottom: 24px;
}

.auth-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.pin-input {
  width: 100%;
  height: 52px;
  background-color: var(--bg-app);
  border: 1px solid var(--border-control);
  border-radius: 8px;
  font-size: 24px;
  font-weight: 700;
  letter-spacing: 8px;
  text-align: center;
  color: var(--accent);
  font-family: monospace;
  transition: border-color 0.2s;
  box-sizing: border-box;
}

.pin-input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: none;
}

.error-msg {
  font-size: 13px;
  color: #ef4444;
  background: var(--status-error-fill);
  border: 1px solid var(--status-error-stroke);
  padding: 8px 12px;
  border-radius: 6px;
}

.auth-hint {
  margin-top: 20px;
  font-size: 12px;
  color: var(--color-text-muted);
  line-height: 1.4;
}

/* Backups View */
.backups-view {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.view-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #252525;
  border: 1px solid var(--border-control);
  border-left: 4px solid var(--accent);
  border-radius: 10px;
  padding: 22px 28px;
}

.banner-left h2 {
  font-size: 19px;
  font-weight: 700;
  margin-bottom: 6px;
  color: var(--color-text);
}

.banner-left p {
  font-size: 13px;
  color: var(--color-text-secondary);
  margin: 0;
}

.count-pill {
  background: var(--bg-control);
  border: 1px solid var(--border-control);
  padding: 6px 14px;
  border-radius: var(--radius-pill);
  font-size: 13px;
  color: var(--color-text-secondary);
  white-space: nowrap;
  flex-shrink: 0;
}

.backups-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 20px;
}

.backup-card {
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 10px;
  padding: 22px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  transition: transform 0.2s, border-color 0.2s;
}

.backup-card:hover {
  transform: translateY(-2px);
  border-color: #3b4861;
}

.card-top {
  display: flex;
  align-items: center;
  gap: 12px;
}

.card-icon {
  font-size: 28px;
}

.card-heading {
  overflow: hidden;
}

.card-title {
  font-size: 16px;
  font-weight: 700;
  color: #f1f5f9;
  margin: 0 0 4px 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-sub {
  font-size: 12px;
  color: var(--color-text-muted);
  display: block;
}

.card-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tag {
  font-size: 12px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 4px;
}

.tag-userdata {
  background: var(--status-success-fill);
  color: #4ade80;
  border: 1px solid var(--status-success-stroke);
}

.tag-cfg {
  background: var(--accent-subtle);
  color: #60a5fa;
  border: 1px solid var(--accent-subtle-hover);
}

.tag-type {
  background: var(--accent-subtle);
  color: var(--level-warning);
  border: 1px solid var(--accent-subtle-hover);
}

.card-meta {
  background: var(--bg-app);
  border-radius: 6px;
  padding: 10px 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.meta-item {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
}

.meta-label {
  color: var(--color-text-muted);
}

.meta-val {
  color: var(--color-text-secondary);
  font-weight: 500;
}

.font-mono {
  font-family: monospace;
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 10px 18px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  text-decoration: none;
  transition: all 0.2s ease;
  border: none;
  box-sizing: border-box;
}

.btn-submit {
  width: 100%;
  background: var(--accent);
  color: #fff;
  padding: 12px;
  font-size: 14px;
}

.btn-submit:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn-download {
  width: 100%;
  background: var(--accent);
  color: #fff;
  border: 1px solid transparent;
  border-radius: var(--radius-control);
  cursor: pointer;
  font-family: var(--font-sans);
  font-size: 13px;
  font-weight: 500;
  transition: background-color var(--duration-fast) var(--ease-standard);
}

.btn-download:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn-download:disabled {
  opacity: 0.45;
  cursor: default;
}

.btn-download.is-error {
  background: var(--status-error-fill);
  color: var(--level-error);
  border-color: var(--status-error-stroke);
}

.btn-download.is-error:hover:not(:disabled) {
  background: rgba(255, 153, 164, 0.2);
}

.dl-inline {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.spinner-tiny {
  width: 13px;
  height: 13px;
  border: 2px solid rgba(255, 255, 255, 0.35);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  flex-shrink: 0;
}

.dl-progress {
  position: relative;
  height: 3px;
  margin-top: 8px;
  border-radius: var(--radius-pill);
  background: var(--bg-control);
  overflow: hidden;
}

.dl-progress-bar {
  display: block;
  height: 100%;
  background: var(--accent);
  border-radius: inherit;
  transition: width var(--duration-fast) linear;
}

.dl-error {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin-top: 8px;
  font-size: 12px;
  line-height: 18px;
  color: var(--level-error);
}

.btn-refresh {
  background: var(--border-color);
  color: var(--color-text-secondary);
  border: 1px solid #334155;
}

.btn-refresh:hover {
  background: var(--border-control);
  color: var(--color-text);
}

.restore-guide-card {
  background: var(--bg-surface);
  border: 1px dashed #2e3a50;
  border-radius: 8px;
  padding: 18px 22px;
  margin-top: 12px;
}

.guide-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.guide-header h4 {
  margin: 0;
  font-size: 14px;
  color: var(--accent);
}

.guide-steps {
  margin: 0;
  padding-left: 20px;
  font-size: 12px;
  color: var(--color-text-secondary);
  line-height: 1.8;
}

.empty-box {
  text-align: center;
  padding: 60px 20px;
  background: var(--bg-surface);
  border: 1px dashed var(--border-color);
  border-radius: 10px;
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 12px;
  opacity: 0.6;
}

.web-footer {
  height: 48px;
  border-top: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: var(--color-text-muted);
}

/* 手机端：收紧内边距，标题与主机信息单行不撑破 */
@media (max-width: 560px) {
  .web-header {
    height: auto;
    padding: 12px 16px;
    flex-wrap: wrap;
    gap: 8px;
  }

  .brand {
    gap: 10px;
    min-width: 0;
  }

  .brand-title {
    font-size: 15px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .web-main {
    padding: 16px;
  }

  .view-banner {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .banner-right {
    align-self: flex-start;
  }

  .auth-card {
    padding: 24px 20px;
  }
}
</style>
