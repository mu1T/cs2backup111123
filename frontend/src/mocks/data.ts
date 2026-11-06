/**
 * 浏览器 / 开发态的假数据。
 *
 * 这里只放**领域数据**，不放行为：真正决定「有没有后端、要不要回退」的判断仍在 `utils/wails.ts`。
 * 每个导出的类型都由 `BindingResult<'Xxx'>` 推导，所以后端签名一变（返回值换形状），
 * 类型检查会直接报错——不会出现「UI 看起来坏了，其实是 mock 过期」。
 */
import type { BindingResult } from '../types/bindings'
import type { GameCFGFile } from '../types'

/** 本地 Steam 账号列表。 */
export const mockUsers: BindingResult<'GetUsers'> = [
  {
    account_id: '123456789',
    steam64: '76561198083722517',
    username: 'ZywOo_CS2',
    account_name: 'zywoo_official',
    remember_password: true,
    is_recent: true,
    avatar_url: '',
    notes: '主力账号 (完美 S / 5E 2800)',
  },
  {
    account_id: '12345678',
    steam64: '76561198848266283',
    username: 's1mple_fan',
    account_name: 's1mple_smurf',
    remember_password: true,
    is_recent: false,
    avatar_url: '',
    notes: '娱乐排位小号',
  },
  {
    account_id: '87654321',
    steam64: '76561198000000000',
    username: 'B1t_Headshot',
    account_name: 'b1t_alt',
    remember_password: true,
    is_recent: false,
    avatar_url: '',
    notes: '',
  },
]

/** game/csgo/cfg 的样例目录：五个来源都有代表文件，便于检查徽标与筛选。 */
const GAME_CFG_SAMPLES: Array<[string, number, GameCFGFile['source'], number]> = [
  ['autoexec.cfg', 9232, 'player', 2],
  ['surf.cfg', 128, 'player', 30],
  ['pt.cfg', 3419, 'player', 12],
  ['pwa.cfg', 53, 'pwa', 1],
  ['pwa_userconfig76561198083722517.cfg', 4577, 'pwa', 1],
  ['pwa_userconfig76561198515265839.cfg', 7079, 'pwa', 3],
  ['gamestate_integration_pwa.cfg', 420, 'pwa', 2],
  ['f5eautoexec.cfg', 657, 'fivee', 20],
  ['f5enostickjump.cfg', 17, 'fivee', 20],
  ['5e_bind.cfg', 412, 'fivee', 8],
  ['boot.vcfg', 11, 'valve', 60],
  ['gamemode_armsrace.cfg', 4337, 'valve', 45],
  ['gamemode_casual.cfg', 4545, 'valve', 20],
  ['gamemode_competitive.cfg', 4472, 'valve', 20],
  ['gamemode_deathmatch.cfg', 4544, 'valve', 45],
  ['machine_convars_default.vcfg', 313, 'valve', 60],
  ['perftest.cfg', 3367, 'valve', 60],
  ['server.cfg', 33, 'valve', 60],
  ['server_default.cfg', 364, 'valve', 60],
  ['user_keys_default.vcfg', 1232, 'valve', 60],
  ['user_keys_dev_default.vcfg', 1451, 'valve', 45],
]

export const createMockGameCFGFiles = (now: number = Date.now()): BindingResult<'ListGameCFGFiles'> =>
  GAME_CFG_SAMPLES.map(([name, size, source, daysAgo]) => ({
    name,
    size,
    modified: new Date(now - daysAgo * 86400000).toISOString(),
    ext: name.slice(name.lastIndexOf('.')).toLowerCase(),
    source,
    read_only: false,
  }))

/** userdata/<id>/730/local/cfg 的样例：编辑器与「按文件备份」都会用到。 */
const USER_CFG_SAMPLES: Array<[string, number]> = [
  ['cs2_machine_convars.vcfg', 13294],
  ['cs2_user_convars_0_slot0.vcfg', 3237],
  ['cs2_user_keys_0_slot0.vcfg', 1457],
  ['cs2_user_keys_0_slot1.vcfg', 16],
  ['cs2_user_keys_0_slot2.vcfg', 16],
  ['cs2_user_keys_0_slot3.vcfg', 16],
  ['cs2_video.txt', 1036],
  ['gamestate_integration_5e.cfg', 575],
  ['trustedlaunch.cfg', 111421],
]

export const createMockUserCFGFiles = (now: number = Date.now()): BindingResult<'ListUserCFGFiles'> =>
  USER_CFG_SAMPLES.map(([name, size]) => ({
    name,
    size,
    modified: new Date(now).toISOString(),
    ext: name.slice(name.lastIndexOf('.')).toLowerCase(),
    source: 'player' as const,
    read_only: false,
  }))

/** 快照列表：一条现代格式、一条旧版迁移格式，用于检查遗留标记的渲染。 */
export const createMockSnapshots = (accountID: string): BindingResult<'GetSnapshots'> => [
  {
    id: '20261104_183000',
    account_id: accountID,
    created_at: new Date().toISOString(),
    note: '示例快照',
    total_files: 3,
    total_bytes: 8420,
    file_hashes: {
      'local/cfg/cs2_video.txt': 'hash1',
      'local/cfg/cs2_user_keys_0_slot0.vcfg': 'hash2',
      'local/cfg/cs2_user_convars_0_slot0.vcfg': 'hash3',
    },
  },
  {
    id: 'v0-legacy',
    account_id: accountID,
    created_at: new Date(Date.now() - 86400000 * 7).toISOString(),
    note: '旧版备份迁移',
    total_files: 2,
    total_bytes: 5210,
    file_hashes: {
      'local/cfg/cs2_video.txt': 'hash0',
      'local/cfg/cs2_user_keys_0_slot0.vcfg': 'hash2',
    },
  },
]
