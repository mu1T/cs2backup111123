<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { formatSize } from '../utils/format'
import {
  getBackedUpAccounts,
  getLANServerStatus,
  startLANServer,
  stopLANServer,
  toggleLANPin,
  regenerateLANPin,
  setLANSharedBackups,
  createLANPackage,
  getLANPackages,
  getUsers,
  startDiscovery,
  stopDiscovery,
  getDiscoveredDevices,
  addManualDevice,
  connectRemoteDevice,
  downloadRemoteBackup,
  setLANRateLimit,
  setLANBindAddress,
} from '../utils/wails'
import type {
  LANServerStatus,
  LANPackageInfo,
  UserCardInfo,
  DiscoveredDevice,
  RemoteBackupItem,
  DownloadedBackupResult,
} from '../types'
import SvgIcon from '../components/SvgIcon.vue'

const emit = defineEmits<{
  (e: 'log-action', message: string): void
}>()

// Active section tab: 'server' (local sharing) or 'discovery' (client remote sync)
const activeTab = ref<'server' | 'discovery'>('discovery')

// ===== Local Server State =====
const status = ref<LANServerStatus>({
  running: false,
  url: '',
  all_urls: [],
  port: 8765,
  share_name: '',
  pin: '123456',
  pin_enabled: false,
  shared_count: 0,
  connections: 0,
  shared_local_ids: [],
  shared_zip_paths: [],
  rate_limit: 0,
  bind_address: '',
  available_addresses: [],
})

const loading = ref(false)
const portInput = ref(8765)
const nameInput = ref('')

/** 限速档位（bytes/sec），0 表示不限。 */
const KB = 1024
const MB = 1024 * 1024
const RATE_OPTIONS = [
  { value: 0, label: '不限速' },
  { value: 300 * KB, label: '300 KB/s' },
  { value: 512 * KB, label: '512 KB/s' },
  { value: 1 * MB, label: '1 MB/s' },
  { value: 2 * MB, label: '2 MB/s' },
  { value: 5 * MB, label: '5 MB/s' },
  { value: 10 * MB, label: '10 MB/s' },
  { value: 20 * MB, label: '20 MB/s' },
]
const rateLimitInput = ref(0)
/** 只提供服务的网卡地址；空串表示全部网卡。 */
const bindAddressInput = ref('')

/**
 * 可选的本机地址。
 *
 * 直接用后端的 `available_addresses`，**不要从 all_urls 推**——那个只在服务
 * 运行时才填，而用户恰恰是在启动之前选网卡。后端已经把不能共享的地址过滤掉
 * （环回、链路本地、Clash TUN）并按 LANRank 排好序。
 */
const availableAddresses = computed(() => status.value.available_addresses || [])

/**
 * 改监听网卡。重启共享服务后才生效——监听地址在启动时绑定。
 */
const handleBindAddressChange = async () => {
  try {
    await setLANBindAddress(bindAddressInput.value)
    status.value.bind_address = bindAddressInput.value
    emit(
      'log-action',
      bindAddressInput.value
        ? `局域网共享将只监听 ${bindAddressInput.value}（重启服务后生效）`
        : '局域网共享将监听全部网卡（重启服务后生效）',
    )
  } catch (err: any) {
    emit('log-action', `设置监听地址失败: ${err?.message || err}`)
  }
}

/**
 * 改限速档位。服务端改完立即生效，不用重启共享服务。
 */
const handleRateLimitChange = async () => {
  try {
    await setLANRateLimit(rateLimitInput.value)
    status.value.rate_limit = rateLimitInput.value
    emit('log-action', `局域网共享限速已调整为: ${RATE_OPTIONS.find((o) => o.value === rateLimitInput.value)?.label || '不限速'}`)
  } catch (err: any) {
    emit('log-action', `设置限速失败: ${err?.message || err}`)
  }
}
const copySuccess = ref(false)
const copyPinSuccess = ref(false)

// Local accounts for sharing
const availableUsers = ref<UserCardInfo[]>([])
/** 全部 userdata 账号。创建备份包时从这里面选——没备份过的账号也能现场打包。 */
const allUsers = ref<UserCardInfo[]>([])
const sharedAccountIDs = ref<string[]>([])

// Packages
const packages = ref<LANPackageInfo[]>([])
const sharedZipPaths = ref<string[]>([])

// Package creation modal/form
const showCreateModal = ref(false)
const createTargetAccount = ref('')
const createPkgName = ref('')
const createIncludeCfg = ref(true)
const createIncludeUserdata = ref(true)
const creatingPackage = ref(false)

// ===== Discovery & Client State =====
const discoveredDevices = ref<DiscoveredDevice[]>([])
const refreshingDevices = ref(false)
const manualIPInput = ref('')
const manualPortInput = ref(8765)
const manualAddError = ref('')

// Remote connection
const selectedDevice = ref<DiscoveredDevice | null>(null)
const remotePinInput = ref('')
const connectingRemote = ref(false)
const connectError = ref('')
const remoteBackups = ref<RemoteBackupItem[]>([])

// Download state
const downloadingBackupID = ref<string | null>(null)
const downloadResult = ref<DownloadedBackupResult | null>(null)
const downloadError = ref('')

let devicePollTimer: number | null = null


const formatRelativeTime = (lastSeenStr: string): string => {
  if (!lastSeenStr) return '未知'
  try {
    const diff = (Date.now() - new Date(lastSeenStr).getTime()) / 1000
    if (diff < 5) return '刚刚在线'
    if (diff < 60) return `${Math.floor(diff)} 秒前`
    return `${Math.floor(diff / 60)} 分钟前`
  } catch {
    return '在线'
  }
}

// ===== Server Methods =====
const refreshStatus = async () => {
  try {
    const s = await getLANServerStatus()
    status.value = s
    portInput.value = s.port || 8765
    nameInput.value = s.share_name || ''
    rateLimitInput.value = s.rate_limit || 0
    bindAddressInput.value = s.bind_address || ''
    sharedAccountIDs.value = [...s.shared_local_ids]
    sharedZipPaths.value = [...s.shared_zip_paths]
  } catch (err) {
    console.warn('Failed to get LAN server status:', err)
  }
}

const handleToggleServer = async () => {
  try {
    loading.value = true
    if (status.value.running) {
      await stopLANServer()
      emit('log-action', '局域网共享服务已停止')
    } else {
      await startLANServer(Number(portInput.value), nameInput.value)
      emit('log-action', `局域网共享服务已启动: Port ${portInput.value}`)
    }
    await refreshStatus()
  } catch (err: any) {
    emit('log-action', `操作局域网共享服务失败: ${err?.message || err}`)
  } finally {
    loading.value = false
  }
}

const handleTogglePin = async () => {
  try {
    const nextState = !status.value.pin_enabled
    await toggleLANPin(nextState)
    status.value.pin_enabled = nextState
    emit('log-action', nextState ? `已启用局域网 PIN 码保护: ${status.value.pin}` : '已停用局域网 PIN 码保护')
  } catch (err: any) {
    emit('log-action', `切换 PIN 状态失败: ${err?.message || err}`)
  }
}

const handleRegeneratePin = async () => {
  try {
    const newPin = await regenerateLANPin()
    status.value.pin = newPin
    emit('log-action', `已生成全新 6 位 PIN 码: ${newPin}`)
  } catch (err: any) {
    emit('log-action', `生成 PIN 码失败: ${err?.message || err}`)
  }
}

const handleToggleAccountShare = async (accountId: string) => {
  const index = sharedAccountIDs.value.indexOf(accountId)
  if (index >= 0) {
    sharedAccountIDs.value.splice(index, 1)
  } else {
    sharedAccountIDs.value.push(accountId)
  }

  try {
    await setLANSharedBackups(sharedAccountIDs.value, sharedZipPaths.value)
    emit('log-action', `已更新本地备份共享列表 (${sharedAccountIDs.value.length} 个账号)`)
    await refreshStatus()
  } catch (err: any) {
    emit('log-action', `保存共享配置失败: ${err?.message || err}`)
  }
}

const handleToggleZipShare = async (zipPath: string) => {
  const index = sharedZipPaths.value.indexOf(zipPath)
  if (index >= 0) {
    sharedZipPaths.value.splice(index, 1)
  } else {
    sharedZipPaths.value.push(zipPath)
  }

  try {
    await setLANSharedBackups(sharedAccountIDs.value, sharedZipPaths.value)
    emit('log-action', `已更新 ZIP 备份包共享列表 (${sharedZipPaths.value.length} 个文件)`)
    await refreshStatus()
  } catch (err: any) {
    emit('log-action', `保存共享配置失败: ${err?.message || err}`)
  }
}

const copyUrl = () => {
  if (!status.value.url) return
  navigator.clipboard.writeText(status.value.url)
  copySuccess.value = true
  setTimeout(() => {
    copySuccess.value = false
  }, 2000)
}

const copyPin = () => {
  if (!status.value.pin) return
  navigator.clipboard.writeText(status.value.pin)
  copyPinSuccess.value = true
  setTimeout(() => {
    copyPinSuccess.value = false
  }, 2000)
}

const handleCreatePackage = async () => {
  if (!createTargetAccount.value) return
  try {
    creatingPackage.value = true
    const targetUser = allUsers.value.find((u) => u.account_id === createTargetAccount.value)
    const defaultName = targetUser ? `${targetUser.username}_Backup` : `User_${createTargetAccount.value}`
    const pkgName = createPkgName.value.trim() || defaultName

    const path = await createLANPackage(
      createTargetAccount.value,
      pkgName,
      createIncludeCfg.value,
      createIncludeUserdata.value
    )

    emit('log-action', `成功创建并共享 LAN 备份包: ${pkgName}`)
    showCreateModal.value = false
    createPkgName.value = ''

    packages.value = await getLANPackages()
    await refreshStatus()
  } catch (err: any) {
    emit('log-action', `创建备份包失败: ${err?.message || err}`)
  } finally {
    creatingPackage.value = false
  }
}

// ===== Discovery & Client Methods =====
const refreshDevices = async () => {
  refreshingDevices.value = true
  try {
    const list = await getDiscoveredDevices()
    discoveredDevices.value = list
    // If selected device is not in list, keep it if manual
    if (selectedDevice.value) {
      const match = list.find(
        (d) => d.ip === selectedDevice.value?.ip && d.port === selectedDevice.value?.port
      )
      if (match) {
        selectedDevice.value = match
      }
    } else if (list.length > 0) {
      selectedDevice.value = list[0]
    }
  } catch (err) {
    console.warn('Failed to refresh discovered devices:', err)
  } finally {
    refreshingDevices.value = false
  }
}

const handleAddManualDevice = async () => {
  manualAddError.value = ''
  const ip = manualIPInput.value.trim()
  const port = Number(manualPortInput.value) || 8765

  if (!ip) {
    manualAddError.value = '请输入有效的 IP 地址'
    return
  }

  try {
    await addManualDevice(ip, port)
    emit('log-action', `已手动添加局域网设备: ${ip}:${port}`)
    manualIPInput.value = ''
    await refreshDevices()
    // Select newly added device
    const target = discoveredDevices.value.find((d) => d.ip === ip && d.port === port)
    if (target) {
      selectedDevice.value = target
    }
  } catch (err: any) {
    manualAddError.value = `添加失败: ${err?.message || err}`
  }
}

const handleSelectDevice = (dev: DiscoveredDevice) => {
  selectedDevice.value = dev
  connectError.value = ''
  downloadError.value = ''
  downloadResult.value = null
}

const handleConnectRemote = async () => {
  if (!selectedDevice.value) return
  connectingRemote.value = true
  connectError.value = ''
  downloadError.value = ''
  downloadResult.value = null

  const dev = selectedDevice.value
  try {
    const backups = await connectRemoteDevice(dev.ip, dev.port, remotePinInput.value.trim())
    remoteBackups.value = backups
    emit('log-action', `成功连接到 [${dev.name || dev.ip}]，获取到 ${backups.length} 个可用备份`)
  } catch (err: any) {
    connectError.value = err?.message || '连接失败，请确认该设备在线且 PIN 码正确'
    emit('log-action', `连接远程设备失败 [${dev.ip}:${dev.port}]: ${connectError.value}`)
  } finally {
    connectingRemote.value = false
  }
}

const handleDownloadBackup = async (item: RemoteBackupItem) => {
  if (!selectedDevice.value) return
  downloadingBackupID.value = item.id
  downloadError.value = ''
  downloadResult.value = null

  const dev = selectedDevice.value
  try {
    emit('log-action', `正在从 [${dev.name || dev.ip}] 下载备份: ${item.backup_name}...`)
    const res = await downloadRemoteBackup(dev.ip, dev.port, remotePinInput.value.trim(), item.id)
    downloadResult.value = res
    emit('log-action', `远程备份导入成功: 存档号 [${res.account_id}], 包含 ${res.total_files} 个文件`)
  } catch (err: any) {
    downloadError.value = err?.message || '下载并解压备份失败'
    emit('log-action', `下载远程备份异常: ${downloadError.value}`)
  } finally {
    downloadingBackupID.value = null
  }
}

const loadData = async () => {
  loading.value = true
  try {
    await refreshStatus()

    // Start background discovery and load devices
    await startDiscovery()
    await refreshDevices()

    // Poll devices every 4 seconds
    devicePollTimer = window.setInterval(refreshDevices, 4000)

    // 只列「已备份」的账号——这一栏叫「本地已备份账号共享」，
    // 列出所有 userdata 账号会让用户勾了之后才发现下不到东西。
    const [users, backedUp] = await Promise.all([getUsers(), getBackedUpAccounts()])
    allUsers.value = users || []
    const backedSet = new Set(backedUp || [])
    // 共享列表只要「已备份」的；创建备份包用全部账号（没备份过也能现场打包）。
    availableUsers.value = allUsers.value.filter((u) => backedSet.has(u.account_id))
    if (allUsers.value.length > 0 && !createTargetAccount.value) {
      createTargetAccount.value = allUsers.value[0].account_id
    }

    // Load packages
    const pkgs = await getLANPackages()
    packages.value = pkgs
  } catch (err) {
    console.warn('Error loading LAN sharing data:', err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})

onUnmounted(() => {
  if (devicePollTimer) {
    clearInterval(devicePollTimer)
  }
})
</script>

<template>
  <div class="lan-sharing-view">
    <!-- Top Header Banner -->
    <section class="banner-card">
      <div class="banner-content">
        <div class="badge-tag">局域网共享</div>
        <h2 class="banner-title">局域网配置共享</h2>
        <p class="banner-desc">
          自动发现局域网内的其他设备并获取配置，或启动本机服务与队友共享配置。
        </p>
      </div>
      <div class="banner-actions">
        <button
          class="btn"
          :class="status.running ? 'btn-stop' : 'btn-start'"
          :disabled="loading"
          :title="status.running ? '停止本机共享服务，局域网内其他设备将无法再连接' : '启动本机共享服务，让局域网内的其他设备可以发现并下载你的配置'"
          @click="handleToggleServer"
        >
          <SvgIcon :name="status.running ? 'stop' : 'play'" :size="14" class="btn-icon" />
          <span>{{ status.running ? '停止本机服务' : '启动本机服务' }}</span>
        </button>
      </div>
    </section>

    <!-- Sub-tab Selector -->
    <div class="view-tabs">
      <button
        class="tab-btn"
        :class="{ active: activeTab === 'discovery' }"
        title="查看局域网内的其他设备，连接后下载它们的共享配置"
        @click="activeTab = 'discovery'"
      >
        <SvgIcon name="search" :size="16" class="tab-icon" />
        <span>局域网设备</span>
        <span v-if="discoveredDevices.length > 0" class="tab-badge count">
          {{ discoveredDevices.length }} 台在线
        </span>
      </button>
      <button
        class="tab-btn"
        :class="{ active: activeTab === 'server' }"
        title="配置本机的共享服务：访问地址、PIN 保护与要共享的账号 / 压缩包"
        @click="activeTab = 'server'"
      >
        <SvgIcon name="wifi" :size="16" class="tab-icon" />
        <span>本机共享服务</span>
        <span class="tab-badge" :class="status.running ? 'online' : 'offline'">
          {{ status.running ? '运行中' : '未启动' }}
        </span>
      </button>
    </div>

    <!-- ========================================== -->
    <!-- TAB 1: 局域网设备 (Ticket 08) -->
    <!-- ========================================== -->
    <div v-if="activeTab === 'discovery'" class="discovery-section">
      <!-- 1. 局域网设备列表与手动添加 -->
      <div class="discovery-grid">
        <!-- Left: Discovered Devices Card -->
        <div class="lan-card">
          <div class="card-header">
            <div class="header-title">
              <SvgIcon name="search" :size="18" class="card-icon" />
              <div>
                <h3>局域网在线设备</h3>
                <span class="sub-caption">自动发现在线设备</span>
              </div>
            </div>
            <div class="header-actions">
              <button
                class="btn btn-secondary btn-sm"
                :disabled="refreshingDevices"
                @click="refreshDevices"
                title="重新广播并刷新在线设备"
              >
                <SvgIcon name="refresh" :size="13" :class="{ spinning: refreshingDevices }" />
                <span>{{ refreshingDevices ? '扫描中...' : '刷新设备' }}</span>
              </button>
            </div>
          </div>

          <div class="card-body">
            <div v-if="discoveredDevices.length === 0" class="empty-list">
              <div class="empty-icon">
                <SvgIcon name="wifi" :size="36" />
              </div>
              <div class="empty-title">当前局域网暂未搜索到其他设备</div>
              <div class="empty-sub">
                请确认同一局域网（同一 Wi-Fi 或交换机）下的对方设备已开启共享服务；亦可在右侧手动输入对方 IP 直接连接。
              </div>
            </div>

            <div v-else class="device-list">
              <div
                v-for="dev in discoveredDevices"
                :key="dev.ip + ':' + dev.port"
                class="device-item"
                :class="{ selected: selectedDevice?.ip === dev.ip && selectedDevice?.port === dev.port }"
                @click="handleSelectDevice(dev)"
              >
                <div class="device-avatar">
                  <SvgIcon name="monitor" :size="20" />
                </div>
                <div class="device-info">
                  <div class="device-name-row">
                    <span class="device-name">{{ dev.name || 'CS2 客户端' }}</span>
                    <span v-if="dev.is_manual" class="pill-tag manual">手动添加</span>
                    <span v-else class="pill-tag auto">自动广播</span>
                  </div>
                  <div class="device-meta font-mono">
                    <span class="meta-ip">{{ dev.ip }}:{{ dev.port }}</span>
                    <span class="meta-dot">·</span>
                    <span class="meta-seen">{{ formatRelativeTime(dev.last_seen) }}</span>
                    <span v-if="dev.version && dev.version !== 'manual'" class="meta-ver">v{{ dev.version }}</span>
                  </div>
                </div>

                <div class="device-action">
                  <button
                    class="btn btn-sm"
                    :class="selectedDevice?.ip === dev.ip && selectedDevice?.port === dev.port ? 'btn-primary' : 'btn-secondary'"
                    title="选中这台设备作为下载目标，之后可在下方获取它的共享备份"
                    @click.stop="handleSelectDevice(dev)"
                  >
                    <template v-if="selectedDevice?.ip === dev.ip && selectedDevice?.port === dev.port"><SvgIcon name="check" :size="13" /> 已选中</template><template v-else>连接</template>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Right: Manual Add Device Card -->
        <div class="lan-card">
          <div class="card-header">
            <div class="header-title">
              <SvgIcon name="plus" :size="18" class="card-icon" />
              <div>
                <h3>手动添加局域网设备</h3>
                <span class="sub-caption">适用于局域网路由器阻断 UDP 广播场景</span>
              </div>
            </div>
          </div>

          <div class="card-body">
            <p class="pin-desc">
              若由于路由器 AP 隔离、网吧交换机策略导致无法自动广播发现，可直接在此输入目标主机的局域网 IP 与端口进行直连。
            </p>
            <p class="field-hint">
              用 ZeroTier / Tailscale 这类内网穿透工具时，<strong>自动发现一定不会工作</strong>——UDP 广播过不了路由器，虚拟网段收不到。
              这种情况请用上面的方式手动填对方在虚拟网段里的 IP。
            </p>

            <div class="form-grid">
              <div class="form-col" style="flex: 2;">
                <label class="field-label">目标主机 IP 地址:</label>
                <input
                  v-model="manualIPInput"
                  type="text"
                  placeholder="例如: 192.168.1.108"
                  class="input-control font-mono"
                  title="对方设备的局域网 IPv4 地址，可在对方「本机共享服务」面板的访问地址里看到"
                  @keyup.enter="handleAddManualDevice"
                />
              </div>
              <div class="form-col" style="flex: 1;">
                <label class="field-label">端口:</label>
                <input
                  v-model.number="manualPortInput"
                  type="number"
                  min="1"
                  max="65535"
                  class="input-control font-mono"
                  title="对方共享服务的监听端口，默认 8765；对方改过端口的话要跟着改"
                  @keyup.enter="handleAddManualDevice"
                />
              </div>
            </div>

            <div v-if="manualAddError" class="alert-error">
              {{ manualAddError }}
            </div>

            <div class="manual-actions">
              <button
                class="btn btn-primary btn-sm"
                :disabled="!manualIPInput.trim()"
                title="把该地址加入设备列表并立即尝试连接（适用于路由器阻断了自动广播的情况）"
                @click="handleAddManualDevice"
              >
                <SvgIcon name="plus" :size="13" />
                <span>添加并开始连接</span>
              </button>
            </div>

            <div class="security-tips">
              <SvgIcon name="info" :size="14" class="tip-icon" />
              <span>提示：在对方电脑的「本机共享服务」面板中，即可查看到对方的局域网 IP 与当前端口。</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 2. 远程备份获取与导入 -->
      <div class="lan-card fetch-card">
        <div class="card-header">
          <div class="header-title">
            <SvgIcon name="download" :size="18" class="card-icon" />
            <div>
              <h3>获取并导入远程备份</h3>
              <span class="sub-caption">连接所选设备并获取备份</span>
            </div>
          </div>
        </div>

        <div class="card-body">
          <!-- Device connection bar -->
          <div class="connection-bar">
            <div class="conn-target">
              <span class="conn-label">当前目标设备:</span>
              <div v-if="selectedDevice" class="conn-device-chip">
                <span class="chip-name">{{ selectedDevice.name }}</span>
                <span class="chip-addr font-mono">({{ selectedDevice.ip }}:{{ selectedDevice.port }})</span>
              </div>
              <span v-else class="conn-none">未选择设备，请在上方面板选择或添加设备</span>
            </div>

            <div class="conn-pin-box">
              <label class="conn-label">PIN 码:</label>
              <input
                v-model="remotePinInput"
                type="text"
                maxlength="6"
                placeholder="6 位数字 (可留空)"
                class="input-control pin-input font-mono"
                title="对方开启了 PIN 保护时填入它的 6 位 PIN 码；对方没开启就留空"
                :disabled="!selectedDevice || connectingRemote"
                @keyup.enter="handleConnectRemote"
              />
            </div>

            <button
              class="btn btn-primary"
              :disabled="!selectedDevice || connectingRemote"
              title="用当前目标设备与 PIN 码拉取它共享的备份列表"
              @click="handleConnectRemote"
            >
              <SvgIcon :name="connectingRemote ? 'refresh' : 'link'" :size="14" :class="{ spinning: connectingRemote }" class="btn-icon" />
              <span>{{ connectingRemote ? '正在连接...' : '连接并获取备份列表' }}</span>
            </button>
          </div>

          <!-- Connection error message -->
          <div v-if="connectError" class="alert-error">
            <SvgIcon name="alert-triangle" :size="14" /> {{ connectError }}
          </div>

          <!-- Download success alert -->
          <div v-if="downloadResult" class="alert-success">
            <div class="success-title">
              <SvgIcon name="check" :size="15" /> 远程配置已成功导入本地备份！
            </div>
            <div class="success-details">
              本地存档编号: <code>{{ downloadResult.account_id }}</code> · 共写入 {{ downloadResult.total_files }} 个文件 ({{ formatSize(downloadResult.total_bytes) }})
              <br />
              来源主机: {{ downloadResult.source_name }} · 下载时间: {{ downloadResult.download_time }}
            </div>
            <div class="success-hint">
              可在「<strong>本地备份</strong>」中查看该配置，并一键还原或对比差异。
            </div>
          </div>

          <!-- Download error message -->
          <div v-if="downloadError" class="alert-error">
            <SvgIcon name="alert-triangle" :size="14" /> {{ downloadError }}
          </div>

          <!-- Remote Backups Card List -->
          <div class="remote-backups-container">
            <div class="backups-header">
              <h4>设备共享的备份列表</h4>
              <span class="count-badge">{{ remoteBackups.length }} 项就绪</span>
            </div>

            <div v-if="remoteBackups.length === 0" class="empty-list remote-empty">
              <span v-if="connectingRemote">正在读取远程设备备份数据...</span>
              <span v-else-if="selectedDevice">点击上方「连接并获取备份列表」按钮加载目标设备的共享备份</span>
              <span v-else>请先选择一个目标设备</span>
            </div>

            <div v-else class="remote-backup-grid">
              <div
                v-for="item in remoteBackups"
                :key="item.id"
                class="remote-backup-card"
              >
                <div class="pkg-card-top">
                  <div class="pkg-icon-box">
                    <SvgIcon name="archive" :size="24" />
                  </div>
                  <div class="pkg-main-info">
                    <div class="pkg-title">{{ item.backup_name }}</div>
                    <div class="pkg-filename font-mono">{{ item.filename }}</div>
                  </div>
                  <div class="pkg-size font-mono text-accent">
                    {{ formatSize(item.size) }}
                  </div>
                </div>

                <div class="pkg-badges-row">
                  <span v-if="item.contains_cfg" class="badge-tag-feature cfg">
                    <SvgIcon name="monitor" :size="11" /> CS2 游戏 CFG
                  </span>
                  <span v-if="item.contains_userdata" class="badge-tag-feature ud">
                    <SvgIcon name="user" :size="11" /> 730 准星与键位
                  </span>
                  <span v-if="item.is_local" class="badge-tag-feature live">
                    <SvgIcon name="zap" :size="11" /> 实时打包
                  </span>
                  <span v-else class="badge-tag-feature zip">
                    <SvgIcon name="archive" :size="11" /> ZIP 备份
                  </span>
                  <span v-if="item.created_at" class="badge-tag-feature time font-mono">
                    <SvgIcon name="clock" :size="11" /> {{ item.created_at.replace('T', ' ').slice(0, 16) }}
                  </span>
                </div>

                <div class="pkg-card-footer">
                  <button
                    class="btn btn-primary btn-sm btn-download"
                    :disabled="downloadingBackupID !== null"
                    title="下载该备份并解压到本地备份仓库，之后可在「本地备份」里一键还原或对比差异"
                    @click="handleDownloadBackup(item)"
                  >
                    <span v-if="downloadingBackupID === item.id">
                      <SvgIcon name="refresh" :size="13" class="spinning" /> 正在下载并解压...
                    </span>
                    <span v-else>
                      <SvgIcon name="download" :size="13" /> 下载并导入到本地
                    </span>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ========================================== -->
    <!-- TAB 2: 本机共享服务 (Ticket 07)     -->
    <!-- ========================================== -->
    <div v-else class="server-section">
      <!-- Two-column Grid: Server Control & PIN Security -->
      <div class="lan-grid">
        <!-- 1. 共享控制卡片 -->
        <div class="lan-card">
          <div class="card-header">
            <div class="header-title">
              <SvgIcon name="wifi" :size="18" class="card-icon" />
              <h3>局域网服务状态</h3>
            </div>
            <span class="status-pill" :class="status.running ? 'online' : 'offline'">
              <span class="dot"></span>
              {{ status.running ? '正在运行' : '已停止' }}
            </span>
          </div>

          <div class="card-body">
            <div class="form-row">
              <label class="field-label">共享访问地址:</label>
              <div class="url-copy-box">
                <input
                  type="text"
                  class="url-input"
                  readonly
                  title="本机共享服务的访问地址，发给同一局域网内的队友即可打开下载页"
                  :value="status.running ? status.url : '服务未启动'"
                  :placeholder="status.running ? '' : '请先点击上方按钮启动服务'"
                />
                <button
                  class="btn btn-secondary btn-sm"
                  :disabled="!status.running"
                  @click="copyUrl"
                  title="复制访问链接"
                >
                  <span v-if="copySuccess"><SvgIcon name="check" :size="13" /> 已复制</span><span v-else>复制</span>
                </button>
              </div>
            </div>

            <!-- Multiple LAN IPs list -->
            <div v-if="status.running && status.all_urls.length > 1" class="multi-ip-box">
              <span class="multi-ip-title">备用网卡地址:</span>
              <div class="multi-ip-list">
                <span v-for="u in status.all_urls.slice(1)" :key="u" class="ip-chip">{{ u }}</span>
              </div>
            </div>

            <div class="form-grid">
              <div class="form-col">
                <label class="field-label">监听端口:</label>
                <input
                  v-model.number="portInput"
                  type="number"
                  min="1024"
                  max="65535"
                  class="input-control"
                  title="共享服务监听的端口，范围 1024-65535；被其他程序占用时换一个再启动"
                  :disabled="status.running"
                />
              </div>
              <div class="form-col">
                <label class="field-label">设备主机名:</label>
                <input
                  v-model="nameInput"
                  type="text"
                  class="input-control"
                  placeholder="我的 CS2 共享主机"
                  title="显示在其他设备「局域网设备」列表里的名字，方便队友认出是哪台电脑"
                  :disabled="status.running"
                />
              </div>
              <div class="form-col">
                <label class="field-label">上传限速:</label>
                <select
                  v-model.number="rateLimitInput"
                  class="input-control"
                  title="限制别人从你这台机器下载的速度；改档立即生效，不用重启服务"
                  @change="handleRateLimitChange"
                >
                  <option v-for="o in RATE_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</option>
                </select>
              </div>
              <div class="form-col">
                <label class="field-label">监听网卡:</label>
                <select
                  v-model="bindAddressInput"
                  class="input-control"
                  title="只在这张网卡上提供服务；装 ZeroTier / Tailscale 时选虚拟网卡的地址。重启共享服务后生效"
                  @change="handleBindAddressChange"
                >
                  <option value="">全部网卡（默认）</option>
                  <option v-for="ip in availableAddresses" :key="ip" :value="ip">{{ ip }}</option>
                </select>
              </div>
            </div>

            <div class="stats-summary">
              <div class="stat-item">
                <span class="stat-label">已共享项:</span>
                <span class="stat-value text-accent">{{ status.shared_count }} 个</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">当前活跃连接:</span>
                <span class="stat-value">{{ status.connections }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 2. PIN 保护控制卡片 -->
        <div class="lan-card">
          <div class="card-header">
            <div class="header-title">
              <SvgIcon name="lock" :size="18" class="card-icon" />
              <h3>PIN 码访问保护</h3>
            </div>
            <label class="switch-container">
              <input
                type="checkbox"
                :checked="status.pin_enabled"
                title="开启后，其他设备访问本机共享服务需要输入 6 位 PIN 码"
                @change="handleTogglePin"
              />
              <span class="switch-slider"></span>
            </label>
          </div>

          <div class="card-body">
            <p class="pin-desc">
              启用后，其他设备访问此客户端需要输入 6 位 PIN 码。
            </p>

            <div class="pin-display-card" :class="{ disabled: !status.pin_enabled }">
              <div class="pin-label">当前访问 PIN 码</div>
              <div class="pin-code-row">
                <span class="pin-number font-mono">{{ status.pin }}</span>
                <div class="pin-actions">
                  <button
                    class="btn btn-secondary btn-sm"
                    @click="copyPin"
                    title="复制 PIN 码"
                  >
                    <span v-if="copyPinSuccess"><SvgIcon name="check" :size="13" /> 已复制</span><span v-else>复制</span>
                  </button>
                  <button
                    class="btn btn-secondary btn-sm"
                    @click="handleRegeneratePin"
                    title="随机重置"
                  >
                    <SvgIcon name="refresh" :size="13" />
                    <span>刷新</span>
                  </button>
                </div>
              </div>
            </div>

            <div class="security-tips">
              <SvgIcon name="shield" :size="14" class="tip-icon" />
              <span>连续输错 5 次将临时锁定 30 秒。</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 3. 本地备份与 ZIP 压缩包共享管理 -->
      <div class="management-grid">
        <!-- 本地账号备份共享 -->
        <div class="mgmt-card">
          <div class="mgmt-header">
            <div class="header-title">
              <SvgIcon name="folder" :size="18" class="card-icon" />
              <h3>本地账号备份共享 (实时打包)</h3>
            </div>
            <span class="count-badge">{{ sharedAccountIDs.length }} 个勾选</span>
          </div>

          <div class="mgmt-body">
            <p class="section-hint">勾选本地已备份账号，外部访问端可直接下载对应的独立安装包：</p>

            <div v-if="availableUsers.length === 0" class="empty-list">
              未发现本地账号备份
            </div>

            <div v-else class="account-share-list">
              <div
                v-for="user in availableUsers"
                :key="user.account_id"
                class="account-share-item"
                :class="{ active: sharedAccountIDs.includes(user.account_id) }"
                @click="handleToggleAccountShare(user.account_id)"
              >
                <div class="checkbox-box">
                  <input
                    type="checkbox"
                    :checked="sharedAccountIDs.includes(user.account_id)"
                    :title="sharedAccountIDs.includes(user.account_id) ? '取消共享后，外部访问端将看不到这个账号' : '勾选后把该账号的配置实时打包共享出去'"
                    @click.stop
                    @change="handleToggleAccountShare(user.account_id)"
                  />
                </div>
                <div class="item-avatar">
                  <SvgIcon name="user" :size="18" />
                </div>
                <div class="item-info">
                  <div class="item-name">{{ user.username }}</div>
                  <div class="item-sub">ID: {{ user.account_id }}</div>
                </div>
                <div class="item-status">
                  <span class="badge" :class="sharedAccountIDs.includes(user.account_id) ? 'badge-shared' : 'badge-idle'">
                    {{ sharedAccountIDs.includes(user.account_id) ? '已共享' : '未共享' }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 标准 LAN Package ZIP 管理 -->
        <div class="mgmt-card">
          <div class="mgmt-header">
            <div class="header-title">
              <SvgIcon name="archive" :size="18" class="card-icon" />
              <h3>ZIP 备份包 (含 restore.bat)</h3>
            </div>
            <button
              class="btn btn-primary btn-sm"
              title="把选中账号的配置打包成带 restore.bat 的 ZIP，接收方无需安装本工具即可还原"
              @click="showCreateModal = true"
            >
              <SvgIcon name="plus" :size="12" />
              <span>创建新备份包</span>
            </button>
          </div>

          <div class="mgmt-body">
            <p class="section-hint">已生成的标准 ZIP 分发包列表，可直接提供局域网一键下载：</p>

            <div v-if="packages.length === 0" class="empty-list">
              暂无已生成的 LAN Package，点击右上角快速创建
            </div>

            <div v-else class="package-list">
              <div
                v-for="pkg in packages"
                :key="pkg.path"
                class="package-item"
                :class="{ active: sharedZipPaths.includes(pkg.path) }"
                @click="handleToggleZipShare(pkg.path)"
              >
                <div class="checkbox-box">
                  <input
                    type="checkbox"
                    :checked="sharedZipPaths.includes(pkg.path)"
                    :title="sharedZipPaths.includes(pkg.path) ? '取消共享后，该压缩包会从下载列表里消失' : '勾选后该 ZIP 包会出现在其他设备的下载列表里'"
                    @click.stop
                    @change="handleToggleZipShare(pkg.path)"
                  />
                </div>
                <div class="item-icon">
                  <SvgIcon name="archive" :size="18" />
                </div>
                <div class="item-info">
                  <div class="item-name">{{ pkg.filename }}</div>
                  <div class="item-sub font-mono">{{ formatSize(pkg.size) }}</div>
                </div>
                <div class="item-status">
                  <span class="badge" :class="sharedZipPaths.includes(pkg.path) ? 'badge-shared' : 'badge-idle'">
                    {{ sharedZipPaths.includes(pkg.path) ? '已共享' : '未共享' }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Create Package Modal -->
    <div v-if="showCreateModal" class="modal-overlay" @click.self="showCreateModal = false">
      <div class="modal-content">
        <div class="modal-header">
          <h3><SvgIcon name="archive" :size="18" /> 生成局域网备份包</h3>
          <button class="modal-close" title="关闭" aria-label="关闭创建备份包" @click="showCreateModal = false">
            <SvgIcon name="x" :size="16" />
          </button>
        </div>

        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">选择目标 Steam 账号:</label>
            <select
              v-model="createTargetAccount"
              class="select-control"
              title="选择要打包进 ZIP 的 Steam 账号"
            >
              <option v-for="u in allUsers" :key="u.account_id" :value="u.account_id">
                {{ u.username }} ({{ u.account_id }})
              </option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label">自定义包名称 (选填):</label>
            <input
              v-model="createPkgName"
              type="text"
              class="input-control"
              placeholder="例如: 大号的CS2设置"
              title="选填。留空则按账号名与生成时间自动命名"
            />
          </div>

          <div class="checkbox-group">
            <label class="check-item">
              <input
                type="checkbox"
                v-model="createIncludeUserdata"
                title="包含 userdata/730：准星、键位绑定、视频画质参数等个人设置"
              />
              <span>包含 Userdata (730) [准星、键位绑定、视频画质参数]</span>
            </label>
            <label class="check-item">
              <input
                type="checkbox"
                v-model="createIncludeCfg"
                title="包含 game/csgo/cfg 下的全局脚本，例如 autoexec.cfg 与控制台脚本"
              />
              <span>包含全局游戏 CFG (game/csgo/cfg) [autoexec.cfg、控制台脚本]</span>
            </label>
          </div>

          <div class="tip-box">
            <SvgIcon name="info" :size="14" /> 打包后将自动在压缩包内写入 <code>restore.bat</code> 与 <code>metadata.json</code>，接收方无需安装任何软件即可一键还原配置。
          </div>
        </div>

        <div class="modal-footer">
          <button
            class="btn btn-secondary"
            title="放弃本次打包，不会生成任何文件"
            @click="showCreateModal = false"
            :disabled="creatingPackage"
          >
            取消
          </button>
          <button
            class="btn btn-primary"
            title="生成 ZIP 压缩包并立即加入共享列表"
            @click="handleCreatePackage"
            :disabled="creatingPackage"
          >
            <span v-if="creatingPackage">正在打包...</span>
            <span v-else>生成并共享</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.lan-sharing-view {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  height: 100%;
  overflow-y: auto;
  padding-bottom: 12px;
}

.banner-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-card);
  flex-shrink: 0;
}

.badge-tag {
  display: none;
}

.banner-title {
  font-family: var(--font-display);
  font-size: 16px;
  font-weight: 600;
  line-height: 22px;
  color: var(--color-text);
  margin-bottom: 2px;
}

.banner-desc {
  font-size: 13px;
  color: var(--color-text-secondary);
  max-width: 680px;
  line-height: 1.5;
}

/* Tabs */
.view-tabs {
  display: flex;
  gap: 8px;
  border-bottom: 1px solid var(--border-color);
  padding-bottom: 4px;
}

.tab-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  padding: 0 14px;
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  color: var(--color-text-secondary);
  font-family: var(--font-sans);
  font-size: 13px;
  font-weight: 400;
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-standard),
    background-color var(--duration-fast) var(--ease-standard);
}

.tab-btn:hover {
  color: var(--color-text);
}

.tab-btn.active {
  color: var(--color-text);
  font-weight: 600;
  border-bottom-color: var(--accent);
}

.tab-icon {
  font-size: 16px;
}

.tab-badge {
  font-size: 12px;
  padding: 2px 7px;
  border-radius: 12px;
  font-weight: 600;
}

.tab-badge.count {
  background: var(--accent-subtle);
  color: var(--accent-light);
}

.tab-badge.online {
  background: var(--status-success-fill);
  color: var(--level-success);
}

.tab-badge.offline {
  background: rgba(148, 163, 184, 0.15);
  color: var(--color-text-secondary);
}

/* Common buttons */
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: var(--control-height);
  padding: 0 14px;
  border-radius: var(--radius-control);
  font-family: var(--font-sans);
  font-size: 13px;
  font-weight: 500;
  line-height: 1;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard);
  border: 1px solid transparent;
}

.btn-start {
  background: var(--accent);
  color: #fff;
  box-shadow: none;
}

.btn-start:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn-stop {
  background: var(--status-error-fill);
  color: var(--level-error);
  border-color: var(--status-error-stroke);
}

.btn-stop:hover:not(:disabled) {
  background: var(--bg-control-hover); color: var(--level-error);
}

.btn-primary {
  background: var(--accent);
  color: #fff;
}

.btn-primary:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn-secondary {
  background: var(--bg-surface);
  color: var(--color-text);
  border: 1px solid var(--border-color);
}

.btn-secondary:hover:not(:disabled) {
  background: var(--bg-surface-alt);
}

.btn-sm {
  height: var(--control-height-sm);
  padding: 0 10px;
  font-size: 12px;
}

/* Grids */
.discovery-section,
.server-section {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.discovery-grid,
.lan-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  gap: 16px;
}

.lan-card {
  padding: 22px;
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-color);
}

.header-title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.card-icon {
  font-size: 20px;
}

.card-header h3 {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text);
  margin: 0;
}

.sub-caption {
  font-size: 12px;
  color: var(--color-text-muted);
}

.card-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

/* Device List Styling */
.device-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 280px;
  overflow-y: auto;
}

.device-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.device-item:hover {
  background: var(--bg-control-hover);
  border-color: var(--border-control-hover);
}

.device-item.selected {
  border-color: var(--accent);
  background: var(--tone-accent-weak);
}

.device-avatar {
  font-size: 20px;
}

.device-info {
  flex: 1;
  overflow: hidden;
}

.device-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.device-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pill-tag {
  font-size: 12px;
  padding: 1px 6px;
  border-radius: 4px;
  font-weight: 600;
}

.pill-tag.auto {
  background: var(--accent-subtle);
  color: var(--accent-light);
}

.pill-tag.manual {
  background: var(--accent-subtle);
  color: var(--accent);
}

.device-meta {
  font-size: 12px;
  color: var(--color-text-muted);
  display: flex;
  gap: 6px;
  margin-top: 2px;
}

.meta-ip {
  color: var(--color-text-secondary);
}

.meta-seen {
  color: var(--color-text-muted);
}

.meta-ver {
  color: var(--color-text-muted);
}

/* Manual add actions */
.manual-actions {
  display: flex;
  justify-content: flex-end;
}

/* Fetch & Download Card */
.fetch-card {
}

.connection-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 18px;
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  flex-wrap: wrap;
}

.conn-target {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 240px;
}

.conn-label {
  font-size: 12px;
  color: var(--color-text-muted);
}

.conn-device-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: var(--bg-surface);
  padding: 4px 10px;
  border-radius: 4px;
  border: 1px solid var(--border-control);
}

.chip-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--accent-light);
}

.chip-addr {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.conn-none {
  font-size: 12px;
  color: var(--color-text-muted);
  font-style: italic;
}

.conn-pin-box {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pin-input {
  width: 170px;
  text-align: center;
  letter-spacing: 2px;
}

.alert-error {
  background: var(--status-error-fill);
  border: 1px solid var(--status-error-stroke);
  color: var(--level-error);
  padding: 10px 14px;
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.4;
}

.alert-success {
  background: var(--status-success-fill);
  border: 1px solid var(--status-success-stroke);
  color: var(--level-success);
  padding: 14px 18px;
  border-radius: 6px;
  font-size: 13px;
  line-height: 1.5;
}

.success-title {
  font-weight: 700;
  margin-bottom: 4px;
}

.success-details {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.success-hint {
  font-size: 12px;
  color: var(--color-text);
  margin-top: 6px;
}

/* Remote Backups List */
.remote-backups-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 6px;
}

.backups-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.backups-header h4 {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  margin: 0;
}

.remote-empty {
  padding: 36px 20px;
}

.remote-backup-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 12px;
}

.remote-backup-card {
  padding: 16px;
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  transition: all 0.15s ease;
}

.remote-backup-card:hover {
  border-color: var(--accent-light);
  background: var(--bg-surface);
}

.pkg-card-top {
  display: flex;
  align-items: center;
  gap: 12px;
}

.pkg-icon-box {
  font-size: 24px;
}

.pkg-main-info {
  flex: 1;
  overflow: hidden;
}

.pkg-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pkg-filename {
  font-size: 12px;
  color: var(--color-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pkg-size {
  font-size: 13px;
  font-weight: 600;
}

.pkg-badges-row {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.badge-tag-feature {
  font-size: 12px;
  padding: 2px 7px;
  border-radius: 4px;
  font-weight: 600;
}

.badge-tag-feature.cfg {
  background: var(--accent-subtle);
  color: var(--accent);
}

.badge-tag-feature.ud {
  background: var(--status-success-fill);
  color: var(--level-success);
}

.badge-tag-feature.live {
  background: var(--accent-subtle);
  color: var(--accent-light);
}

.badge-tag-feature.zip {
  background: var(--accent-subtle);
  color: var(--accent-light);
}

.badge-tag-feature.time {
  background: rgba(148, 163, 184, 0.1);
  color: var(--color-text-secondary);
}

.pkg-card-footer {
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  padding-top: 10px;
}

.btn-download {
  width: 100%;
  justify-content: center;
}

/* Other form controls */
.form-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-label {
  font-size: 12px;
  color: var(--color-text-muted);
}

.url-copy-box {
  display: flex;
  gap: 8px;
}

.url-input {
  flex: 1;
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 8px 12px;
  color: var(--accent-light);
  font-family: monospace;
  font-size: 13px;
}

.multi-ip-box {
  background: var(--bg-app-alt);
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 12px;
}

.multi-ip-title {
  color: var(--color-text-muted);
  margin-right: 8px;
}

.multi-ip-list {
  display: inline-flex;
  gap: 6px;
  flex-wrap: wrap;
}

.ip-chip {
  background: var(--bg-surface);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--color-text-secondary);
  font-family: monospace;
}

.field-hint {
  margin: 0 0 10px;
  font-size: 11.5px;
  line-height: 1.6;
  color: var(--color-text-muted);
}

.field-hint strong {
  color: var(--color-text-secondary);
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.form-col {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.input-control,
.select-control {
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 8px 12px;
  color: var(--color-text);
  font-size: 13px;
  /* 显式高度：<select> 的固有高度按字体算，padding 上下会把它撑得比内容矮，
     表现就是「字比框大、下半截被切掉」。统一给死高度，两种控件对齐。 */
  height: 36px;
  line-height: 20px;
}

/* 下拉单独处理：Chrome/WebView2 里 select 的上下 padding 会挤压内容区，
   改成只留左右内边距，垂直居中交给 height + line-height。 */
select.input-control,
select.select-control {
  padding: 0 10px;
  cursor: pointer;
}

.input-control:focus,
.select-control:focus {
  outline: none;
  border-color: var(--accent);
}

.stats-summary {
  display: flex;
  justify-content: space-between;
  padding: 10px 14px;
  background: var(--bg-surface);
  border-radius: 6px;
  font-size: 12px;
}

.stat-item {
  display: flex;
  gap: 6px;
}

.stat-label {
  color: var(--color-text-muted);
}

.text-accent {
  color: var(--accent);
  font-weight: 600;
}

/* Switch */
.switch-container {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
}

.switch-container input {
  opacity: 0;
  width: 0;
  height: 0;
}

.switch-slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--border-control);
  transition: 0.3s;
  border-radius: 24px;
}

.switch-slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  transition: 0.3s;
  border-radius: 50%;
}

input:checked + .switch-slider {
  background-color: var(--accent);
}

input:checked + .switch-slider:before {
  transform: translateX(20px);
}

.pin-desc {
  font-size: 12px;
  color: var(--color-text-muted);
  line-height: 1.5;
}

.pin-display-card {
  background: var(--bg-app-alt);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.pin-display-card.disabled {
  opacity: 0.5;
}

.pin-label {
  font-size: 12px;
  color: var(--color-text-muted);
  text-transform: uppercase;
  letter-spacing: 1px;
}

.pin-code-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.pin-number {
  font-size: 24px;
  font-weight: 700;
  letter-spacing: 4px;
  color: var(--accent);
}

.pin-actions {
  display: flex;
  gap: 6px;
}

.security-tips {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--color-text-muted);
  line-height: 1.4;
}

/* Management grid */
.management-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  gap: 16px;
}

.mgmt-card {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 20px;
  display: flex;
  flex-direction: column;
}

.mgmt-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--border-color);
}

.mgmt-header h3 {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
}

.count-badge {
  font-size: 12px;
  background: var(--bg-surface);
  color: var(--color-text-secondary);
  padding: 2px 8px;
  border-radius: 12px;
}

.section-hint {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-bottom: 12px;
}

.account-share-list,
.package-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 280px;
  overflow-y: auto;
}

.account-share-item,
.package-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.account-share-item:hover,
.package-item:hover {
  background: var(--bg-control-hover);
}

.account-share-item.active,
.package-item.active {
  border-color: var(--accent);
  background: var(--tone-accent-weak);
}

.item-avatar,
.item-icon {
  font-size: 18px;
}

.item-info {
  flex: 1;
  overflow: hidden;
}

.item-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.item-sub {
  font-size: 12px;
  color: var(--color-text-muted);
}

.badge {
  font-size: 12px;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 600;
}

.badge-shared {
  background: var(--status-success-fill);
  color: var(--level-success);
}

.badge-idle {
  background: rgba(148, 163, 184, 0.15);
  color: var(--color-text-secondary);
}

.empty-list {
  text-align: center;
  padding: 30px;
  color: var(--color-text-muted);
  font-size: 12px;
  background: var(--bg-surface);
  border-radius: 6px;
}

.empty-icon {
  font-size: 28px;
  margin-bottom: 8px;
}

.empty-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
  margin-bottom: 4px;
}

.empty-sub {
  font-size: 12px;
  color: var(--color-text-muted);
  max-width: 400px;
  margin: 0 auto;
  line-height: 1.5;
}

/* Modal */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  /* 不能用 --bg-card：它是 rgba(255,255,255,0.05)，铺在 70% 黑的遮罩上
     等于没有背景，弹窗看起来是透明的。对话框要用实色。 */
  background: var(--bg-card-solid);
  border: 1px solid var(--border-color);
  border-radius: 10px;
  width: 480px;
  max-width: 90vw;
  box-shadow: var(--shadow-dialog);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color);
}

.modal-header h3 {
  font-size: 15px;
  color: var(--color-text);
}

.modal-close {
  background: none;
  border: none;
  color: var(--color-text-muted);
  font-size: 20px;
  cursor: pointer;
}

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 12px;
  color: var(--color-text-muted);
}

.checkbox-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: var(--bg-surface);
  padding: 12px;
  border-radius: 6px;
}

.check-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--color-text);
  cursor: pointer;
}

.tip-box {
  font-size: 12px;
  color: var(--color-text-secondary);
  background: var(--accent-subtle);
  border: 1px solid var(--accent-subtle-hover);
  padding: 10px 12px;
  border-radius: 6px;
  line-height: 1.5;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 20px;
  border-top: 1px solid var(--border-color);
}
</style>
