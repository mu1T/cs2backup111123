/**
 * Go ↔ TS 的类型契约检查。
 *
 * 这个文件不导出任何运行时代码，它的全部价值是「编译不过就说明契约漂移了」。
 *
 * 背景：`types/index.ts` 里的这些接口是照着 Go 结构体手抄的。手抄的问题是
 * 改了 Go 侧一个 json tag，TS 侧不报错，运行时字段变 `undefined` → `NaN` →
 * 被后端的 Normalize 静默夹到边界值，用户看到的是「我的设置被改了」。
 *
 * 现在 `frontend/wailsjs/go/models.ts` 由 `wails generate module` 从 Go 结构体
 * 生成。这里断言 **Go 的每个数据字段在 TS 侧都存在**——Go 加字段忘了同步，
 * 就会在这里编译失败。
 *
 * 只断言「Go 的字段 ⊆ TS 的字段」，不要求双向相等：
 * - 生成出来的 class 带一个 `convertValues()` 辅助方法，那是 Wails 用来把嵌套
 *   对象转成类实例的，不是契约的一部分，双向比会把它算进去导致每条都假红；
 * - 前端有意保留几个收窄过的字段类型（例如 `format` 是字面量联合而不是
 *   `string`），那是为了分支收窄，不该被「对齐」掉。
 *
 * 重新生成绑定：`wails generate module`（需要 Wails v2 CLI）。
 */

import type { backup, crosshair, lan, steam, video } from '../../wailsjs/go/models'
import type {
  Crosshair,
  UserCardInfo,
  SnapshotInfo,
  LANServerStatus,
  VideoPreset,
  GameCFGFile,
  FileDiffItem,
} from './index'

/** 只取数据字段，把方法剔掉。 */
type DataKeys<T> = {
  [K in keyof T]: T[K] extends (...args: never[]) => unknown ? never : K
}[keyof T]

/** Go 的每个字段都在 TS 里存在。 */
type Covers<Ts, Go> = Exclude<DataKeys<Go>, DataKeys<Ts>> extends never ? true : false

/** 断言写成调用：类型不成立时这里就是编译错误。 */
function assertCovers<T extends true>(): T | void {
  return undefined
}

// 准星：19 个字段，最容易漏的是 2026-09 重构后新增的像素制字段。
assertCovers<Covers<Crosshair, crosshair.Crosshair>>()
assertCovers<Covers<SnapshotInfo, backup.SnapshotInfo>>()
assertCovers<Covers<UserCardInfo, steam.UserCardInfo>>()
assertCovers<Covers<LANServerStatus, lan.LANServerStatus>>()
assertCovers<Covers<VideoPreset, video.VideoPreset>>()
assertCovers<Covers<GameCFGFile, backup.GameCFGFile>>()
assertCovers<Covers<FileDiffItem, backup.FileDiffItem>>()

// 引用一下，避免被判为未使用。
void assertCovers

// ---- 方法名契约 ----

/**
 * 手写的 `AppBindings` 与生成的方法表必须**方法名完全一致**。
 *
 * 这里只比名字，不比签名：生成的方法返回的是 `models.ts` 里的 class（带
 * `convertValues()` 辅助方法），而 mock 数据是普通对象字面量——直接把签名
 * 对齐会逼着每个 mock 都去 new 一个 class。名字对得上就足以发现
 * 「Go 那边改了方法名，前端还在调老名字」这类漂移。
 */
import * as WailsApp from '../../wailsjs/go/main/App'
import type { AppBindings } from './bindings'

type GeneratedMethods = keyof typeof WailsApp
type HandWrittenMethods = keyof AppBindings

/**
 * Go 侧存在、但前端不该调用的方法。
 *
 * `SetAssets(assets fs.FS)` 的参数是 Go 的 `fs.FS`，前端拿不到这种值——它是
 * 启动时把内嵌资源注进去用的，Wails 照常生成了绑定，但前端没有调用面。
 * 这类方法必须在这里显式登记，否则契约检查会一直红。
 */
type GoOnlyMethods = 'SetAssets'

/** 手写的每个方法名都必须在生成物里存在（防改名）。 */
assertCovers<Exclude<HandWrittenMethods, GeneratedMethods> extends never ? true : false>()
/** 生成物里的每个方法都被覆盖，或者已登记为 Go 专用（防漏接新绑定）。 */
assertCovers<Exclude<GeneratedMethods, HandWrittenMethods | GoOnlyMethods> extends never ? true : false>()
