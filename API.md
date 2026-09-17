# 上海电信 IPTV 爬虫 - API 接口文档

> 基础地址：`http://<host>:8888`
>
> 📖 返回 [README](README.md)

---

## 目录

- [快速开始](#快速开始)
- [通用说明](#通用说明)
- [接口总览](#接口总览)
- [1. 健康检查](#1-健康检查)
- [2. 定时任务调度](#2-定时任务调度)
- [3. 最近请求记录](#3-最近请求记录)
- [4. 手动触发任务](#4-手动触发任务)
- [5. 网络连通性检查](#5-网络连通性检查)
- [6. 版本更新检查](#6-版本更新检查)
- [7. 远程一键升级](#7-远程一键升级)
- [8. 状态监控页面](#8-状态监控页面)
- [9. M3U 播放列表](#9-m3u-播放列表)
- [10. 单频道 M3U8](#10-单频道-m3u8)
- [11. EPG 配置管理](#11-epg-配置管理)
- [12. 频道管理](#12-频道管理)
- [13. XMLTV 节目单](#13-xmltv-节目单)
- [14. 日志管理](#14-日志管理)
- [15. SSE 实时日志流](#15-sse-实时日志流)
- [附录A：数据库表结构](#附录a数据库表结构)
- [附录B：M3U8 输出格式说明](#附录bm3u8-输出格式说明)
- [附录C：API 优化建议](#附录capi-优化建议)

---

## 快速开始

### 最常用接口

| 场景 | 接口 | 示例 |
|------|------|------|
| 获取播放列表 | `GET /api/m3u8` | 导入 TiviMate/Kodi |
| 获取播放列表 | `GET /api/diyp` | 导入 diyp |
| 获取播放列表 | `GET /api/tsM3u8` | 导入 标准m3u |
| 获取节目单 | `GET /api/epg` | 导入 EPG 数据源 |
| 查看系统状态 | `GET /api/status.html` | 浏览器打开管理面板 |
| 健康检查 | `GET /api/health` | 监控/告警系统轮询 |

---

## 通用说明

### 请求格式

- **GET 请求**：参数通过 URL Query String 传递
- **POST 请求**：请求体为 `application/json`，参数通过 JSON Body 传递

### 响应格式

所有接口返回 JSON（除非特别说明）：

**成功响应：**
```json
{
  "success": true,
  // ... 业务数据
}
```

**错误响应：**
```json
{
  "error": "错误描述信息"
}
```

### HTTP 状态码一览

| 状态码 | 含义 | 常见场景 |
|--------|------|----------|
| 200 | 成功 | 正常响应 |
| 400 | 请求参数错误 | 缺少必填参数、参数格式错误、值不合法 |
| 404 | 资源不存在 | 频道未找到、配置记录未初始化 |
| 409 | 资源冲突 | 频道名称已存在（添加自定义频道时） |
| 500 | 服务器内部错误 | 数据库写入失败等 |
| 503 | 服务不可用 | 数据库连接断开 |

### 认证说明

当前版本 **未启用 API 认证**。所有接口可直接访问，无需 Token/API Key。

> ⚠️ **安全建议**：如果服务暴露在公网，建议通过反向代理（Nginx/Caddy）添加 Basic Auth 或 IP 白名单，限制 `/api/admin/*` 和 `POST` 类接口的访问。

### 缓存机制

- **M3U8**：默认缓存 240 分钟（`config.yaml` 中 `cache.default_timeout`），可通过 `?ref=true` 强制刷新
- **EPG XMLTV**：默认缓存，可通过 `?ref=true` 强制刷新
- **版本检查**：结果缓存 5 分钟，可通过 `?force=true` 强制刷新
- **状态页面**：频道/EPG 数据缓存 10 秒

### 并发控制

对于高开销操作（M3U8 生成、手动触发任务），使用 `singleflight` 机制：**同一时刻多个相同请求会被合并为一个**，避免重复计算/数据库压力。

---

## 接口总览

| # | 方法 | 路径 | 说明 | 认证 |
|---|------|------|------|------|
| 1 | GET | `/api/health` | 健康检查（待实现） | 无 |
| 2 | GET | `/api/schedule` | 定时任务调度信息 | 无 |
| 3 | GET | `/api/requests` | 最近请求记录（待实现） | 无 |
| 4 | GET | `/api/run?task=` | 手动触发任务 | 无 |
| 5 | GET | `/api/network-check` | 网络连通性检查 （待实现）| 无 |
| 6 | GET | `/api/version-check` | 版本更新检查 （待实现）| 无 |
| 7 | GET | `/api/self-upgrade` | 远程一键升级 （待实现）| 无 |
| 8 | GET | `/api/status.html` | 状态监控页面（HTML）（待实现） | 无 |
| 9 | GET | `/api/m3u8` | M3U 播放列表 | 无 |
| 9 | GET | `/api/diyp` | M3U 播放列表 | 无 |
| 9 | GET | `/api/tsM3u8` | M3U 播放列表 | 无 |
| 10 | GET | `/api/channel/m3u8` | 单频道 M3U8 | 无 |
| 11 | GET | `/api/epg/config` | 获取 EPG 配置 | 无 |
| 12 | POST | `/api/epg/config` | 更新 EPG 配置 | 无 |
| 13 | GET | `/api/channel/list` | 频道列表 | 无 |
| 14 | POST | `/api/channel/toggle` | 频道显隐开关 | 无 |
| 15 | POST | `/api/channel/rename` | 频道重命名 | 无 |
| 16 | POST | `/api/channel/sort` | 频道排序 | 无 |
| 17 | POST | `/api/channel/custom/add` | 添加自定义频道 | 无 |
| 18 | POST | `/api/channel/custom/update` | 更新自定义频道 | 无 |
| 19 | POST | `/api/channel/custom/delete` | 删除自定义频道 | 无 |
| 20 | GET | `/api/epg` | XMLTV 节目单 | 无 |
| 20 | GET | `/api/epgjson` | JSON 格式节目单 | 无 |
| 21 | GET | `/api/admin/log-level` | 获取日志级别（待实现） | 无 |
| 22 | POST | `/api/admin/log-level` | 设置日志级别（待实现） | 无 |
| 23 | GET | `/api/log/stream` | SSE 实时日志流（待实现） | 无 |

---

## 1. 健康检查

```
GET /api/health
```

**用途**：监控系统整体健康状况，适合接入 Prometheus、Uptime Kuma 等监控工具。

### 请求参数

无

### 成功响应

**HTTP 200** — 系统正常

```json
{
  "status": "ok",
  "db": "connected",
  "session": "valid",
  "uptime": "running",
  "last_fetch": "2026-06-13 13:00:00",
  "last_epg_fetch": "2026-06-13 12:58:30",
  "channel_count": 230,
  "epg_count": 4521
}
```

**HTTP 200** — 系统降级（数据库正常，但认证过期或未初始化）

```json
{
  "status": "degraded",
  "db": "connected",
  "session": "expired",
  "uptime": "running",
  "last_fetch": "2026-06-13 13:00:00",
  "last_epg_fetch": "",
  "channel_count": 230,
  "epg_count": 0
}
```

### 错误响应

**HTTP 503** — 数据库断开

```json
{
  "status": "down",
  "db": "disconnected",
  "session": "not_authenticated",
  "uptime": "running",
  "last_fetch": "2026-06-13 13:00:00",
  "last_epg_fetch": "",
  "channel_count": 230,
  "epg_count": 0
}
```

### 响应字段说明

| 字段 | 类型 | 说明 | 可能值 |
|------|------|------|--------|
| `status` | string | 系统整体状态 | `ok` / `degraded` / `down` |
| `db` | string | 数据库连接状态 | `connected` / `disconnected` |
| `session` | string | 认证会话状态 | `valid` / `recovering` / `expired` / `not_authenticated` / `not_initialized` |
| `uptime` | string | 运行状态 | `running` |
| `last_fetch` | string | 频道列表最后拉取时间 | `2006-01-02 15:04:05` 格式，未拉取则为空字符串 |
| `last_epg_fetch` | string | 节目单最后拉取时间 | `2006-01-02 15:04:05` 格式，未拉取则为空字符串 |
| `channel_count` | int | 频道总数 | - |
| `epg_count` | int | 近7天节目单总数 | - |

### 监控脚本示例

```bash
# 配合 cron 做健康告警
STATUS=$(curl -s http://127.0.0.1:8888/api/health | jq -r '.status')
if [ "$STATUS" != "ok" ]; then
    echo "IPTV Spider 异常: $STATUS" | mail -s "告警" admin@example.com
fi
```

---

## 2. 定时任务调度

```
GET /api/schedule
```

**用途**：查看所有 cron 定时任务的执行时间。

### 请求参数

无

### 成功响应

**HTTP 200**

```json
[
  {
    "ID": 1,
    "PreTime": "2026-06-13T08:00:00Z",
    "NextTime": "2026-06-13T16:00:00Z"
  },
  {
    "ID": 2,
    "PreTime": "2026-06-13T09:00:00Z",
    "NextTime": "2026-06-13T17:00:00Z"
  }
]
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | int | 任务编号（cron.EntryID） |
| `PreTime` | string | 上次执行时间 (ISO 8601) |
| `NextTime` | string | 下次执行时间 (ISO 8601) |

### 内置定时任务

| ID | 任务 | 默认频率 |
|----|------|----------|
| — | 会话保活 | 每 30 分钟 |
| — | 获取频道列表 | 每天 1 次 (`@daily`) |
| — | 获取节目单 | 每天 9:00, 17:00, 22:00（可配置） |
| — | 清理过期数据 | 每 2 小时 |
| — | 上传 M3U/XMLTV 到 OSS | 可配置（默认关闭） |

---

## 3. 最近请求记录

```
GET /api/requests
```

**用途**：查看最近访问过 API 的客户端信息。

### 请求参数

无

### 成功响应

**HTTP 200**

```json
[
  {
    "ip": "192.168.0.100:54321",
    "path": "/api/m3u8",
    "time": "2026-06-13T13:30:00+08:00",
    "ua": "TiviMate/4.7.0"
  },
  {
    "ip": "192.168.0.101:54322",
    "path": "/api/health",
    "time": "2026-06-13T13:31:00+08:00",
    "ua": "curl/7.68.0"
  }
]
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `ip` | string | 请求来源 IP:Port |
| `path` | string | 请求路径 |
| `time` | string | 请求时间 (ISO 8601) |
| `ua` | string | User-Agent（截取前 100 字符） |

> 💡 内存环形缓冲区，保留最近 **100** 条记录，重启后清空。

---

## 4. 手动触发任务

```
GET /api/run?task=<任务名>
```

**用途**：手动触发数据更新、清理等任务。任务异步执行，立即返回。

### 请求参数

| 参数 | 必填 | 类型 | 说明 |
|------|------|------|------|
| `task` | 是 | string | 任务名称，见下方可用任务列表 |

### 可用任务

| task 值 | 说明 | 执行耗时（估算） |
|---------|------|-----------------|
| `update-chi` | 更新频道列表（拉取上海电信IPTV频道数据） | 10-30s |
| `update-epg` | 更新节目单（逐频道拉取EPG数据） | 2-10min（取决于频道数） |
| `clean-ch` | 清理频道播放数据 | <1s |
| `clean-chi` | 清理频道信息数据 | <1s |
| `clean-epg` | 清理过期节目单数据 | <1s |
| `clean` | 清理全部过期数据（频道 + 频道信息 + 节目单） | <1s |
| `upload-m3u` | 生成并上传 M3U 到 OSS | 取决于 OSS 速度 |
| `upload-xmltv` | 生成并上传 XMLTV 到 OSS | 取决于 OSS 速度 |
| `upload-xmltv7` | 生成并上传7天 XMLTV 到 OSS | 取决于 OSS 速度 |

### 成功响应

**HTTP 200**

```
OK
```

> 返回纯文本，非 JSON。

### 错误响应

**HTTP 400** — 缺少 task 参数

```
Bad Request
```

### 注意事项

- 任务**异步执行**，接口立即返回 `OK`，不代表任务已完成
- **并发请求合并**：同一任务同时在执行时，后续请求不会重复触发
- 使用 `update-chi` 后再用 `update-epg` 拉取节目单
- 可通过 `/api/health` 查询 `last_fetch` 和 `epg_count` 来判断任务是否完成

### 使用示例

```bash
# 触发频道列表更新
curl http://127.0.0.1:8888/api/run?task=update-chi

# 触发节目单更新
curl http://127.0.0.1:8888/api/run?task=update-epg

# 组合：更新频道后再更新节目单
curl http://127.0.0.1:8888/api/run?task=update-chi && sleep 30 && curl http://127.0.0.1:8888/api/run?task=update-epg
```

---

## 5. 网络连通性检查

```
GET /api/network-check
```

**用途**：检测外网和 IPTV 专网的连通性（TCP 连接测试）。

### 请求参数

无

### 成功响应

**HTTP 200**

```json
{
  "internet": "ok",
  "internet_ms": 35,
  "iptv": "ok",
  "iptv_ms": 12,
  "message": "外网正常(35ms)，IPTV专网正常(12ms)"
}
```

### 可能响应

**外网不通：**
```json
{
  "internet": "fail",
  "internet_ms": 0,
  "iptv": "ok",
  "iptv_ms": 15,
  "message": "外网不通，IPTV专网正常(15ms)"
}
```

| 字段 | 类型 | 说明 | 可能值 |
|------|------|------|--------|
| `internet` | string | 外网连通状态 | `ok` / `fail` |
| `internet_ms` | int64 | 外网 TCP 连接延迟（毫秒） | ≥0，失败时为 0 |
| `iptv` | string | IPTV 专网连通状态 | `ok` / `fail` |
| `iptv_ms` | int64 | IPTV 专网 TCP 连接延迟（毫秒） | ≥0，失败时为 0 |
| `message` | string | 汇总描述信息 | 中文文本 |

### 检测目标

| 网络 | 目标 | 超时 |
|------|------|------|
| 外网 | `www.baidu.com:80` | 5 秒 |
| IPTV 专网 | `config.yaml` 中 `stb.auth_host` 配置地址 | 5 秒 |

---

## 6. 版本更新检查

```
GET /api/version-check?[force=true]
```

**用途**：对比 GitHub Release 检查是否有新版本。

### 请求参数

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `force` | 否 | `false` | 设为 `true` 强制刷新（跳过5分钟缓存） |

### 成功响应

**HTTP 200** — 有新版本

```json
{
  "current": "V0.0.8",
  "latest": "V0.0.9",
  "has_update": true,
  "url": "https://github.com/jjcszxh/sh-tel-iptv-spider/releases"
}
```

**HTTP 200** — 已是最新

```json
{
  "current": "V0.0.9",
  "latest": "V0.0.9",
  "has_update": false,
  "url": "https://github.com/jjcszxh/sh-tel-iptv-spider/releases"
}
```

**HTTP 200** — 检查失败（网络不通等）

```json
{
  "current": "V0.0.9",
  "latest": "",
  "has_update": false,
  "url": "https://github.com/jjcszxh/sh-tel-iptv-spider/releases"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `current` | string | 当前程序版本 |
| `latest` | string | 远端最新版本（获取失败时为空字符串） |
| `has_update` | bool | 是否有新版本可用 |
| `url` | string | GitHub Releases 页面地址 |

### 版本获取策略（三层降级）

1. **GitHub Releases API**（最可靠）：`api.github.com/repos/.../releases/latest`
2. **GitHub Releases 页面解析**（备用）：解析 HTML 提取 tag
3. **国内加速镜像**（兜底）：通过 `ghproxy.com` 代理访问 `version.txt`

---

## 7. 远程一键升级

```
GET /api/self-upgrade
```

**用途**：从 GitHub Release 下载最新二进制文件，校验后自动替换并重启程序。

### 请求参数

无

### 成功响应

**HTTP 200** — 升级成功，程序将在 1 秒后退出

```json
{
  "success": true,
  "message": "升级成功 V0.0.8 -> V0.0.9，程序将在 1 秒后重启",
  "step": "done",
  "version": "V0.0.9"
}
```

### 错误响应

**HTTP 200** — 已是最新版本

```json
{
  "success": false,
  "message": "已经是最新版本 V0.0.9",
  "step": "check"
}
```

**HTTP 200** — 下载失败

```json
{
  "success": false,
  "message": "下载失败: HTTP 404",
  "step": "download",
  "version": "V0.0.9"
}
```

**HTTP 200** — 校验失败

```json
{
  "success": false,
  "message": "SHA256 校验失败，文件可能已损坏",
  "step": "verify",
  "version": "V0.0.9"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | bool | 是否升级成功 |
| `message` | string | 详细说明信息 |
| `step` | string | 失败时所在的步骤 |
| `version` | string | 目标版本号 |

### step 步骤说明

| step 值 | 说明 |
|---------|------|
| `check` | 版本检查阶段（已是最新或无法获取版本） |
| `download` | 下载阶段（HTTP 请求失败） |
| `verify` | 校验阶段（文件头或 SHA256 不匹配） |
| `save` | 保存阶段（写入临时文件失败） |
| `backup` | 备份阶段（备份旧文件失败） |
| `replace` | 替换阶段（替换失败，已自动回滚） |
| `done` | 升级完成 |

### 升级流程

1. 强制刷新版本检查（跳过缓存）
2. 从 GitHub Release 下载对应平台二进制（3 次重试，超时 5 分钟）
3. 文件头校验（Linux: ELF `0x7F E L F`，Windows: PE `MZ`）
4. SHA256 校验（从 `.sha256` 文件对比）
5. 写入临时文件 → 备份旧文件 → 替换 → 清理
6. 程序退出，由进程管理器自动重启

> ⚠️ **限制**：
> - 下载文件上限 50MB
> - 依赖外部进程管理器（procd/systemd/supervisor）自动重启
> - 替换失败会自动回滚到备份文件

---

## 8. 状态监控页面

```
GET /api/status.html
```

**用途**：Web 管理面板，一站式管理 IPTV Spider。

### 页面功能

| 模块 | 功能 |
|------|------|
| 系统状态卡片 | 数据库连接、认证会话、下次拉取时间 |
| 数据统计卡片 | 频道总数、活跃频道、节目单总数、超时未更新、无节目单 |
| 频道列表表格 | 搜索、排序、HD/4K 标签、EPG 状态标签 |
| 频道 M3U8 弹窗 | 点击频道名查看 M3U8 内容，支持复制 |
| 一键下载 | 下载完整 M3U8 播放列表 / EPG 节目单 |
| 频道管理 | 显隐切换、重命名、排序（拖拽）、自定义频道增删改 |
| EPG 配置面板 | 在线修改 RTSP/RTP/Playseek/Cron/Logo URL |
| 网络检查 | 外网 + IPTV 专网连通性测试（实时终端输出） |
| 触发更新 | 手动触发频道+节目单更新（实时终端输出） |
| SSE 实时日志 | 实时查看后台日志流 |
| 最近请求 | 最近 API 访问记录 |
| 版本更新 | 自动检查 GitHub 更新，一键在线升级 |
| 暗黑模式 | 主题切换，偏好记忆到 localStorage |

### 安全建议

此页面会暴露系统内部信息。建议：
- 通过 Nginx 反向代理添加 Basic Auth
- 或限制仅内网访问
- 不要将此端口直接暴露到公网

---

## 9. M3U 播放列表

```
GET /api/m3u8?[udpxy=<ip:port>]&[xteve=true]&[all=true]&[ref=true]
```

**用途**：生成 IPTV M3U 播放列表文件，可导入 TiviMate、Kodi、VLC 等播放器。

### 请求参数

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `udpxy` | 否 | - | udpxy 代理地址，如 `192.168.0.1:4022` |
| `xteve` | 否 | - | 设为 `true` 输出 xteve 兼容格式（`udp://@` 开头） |
| `all` | 否 | - | 设为 `true` 包含所有频道（含已隐藏的频道） |
| `ref` | 否 | - | 设为 `true` 跳过缓存强制重新生成 |
| `scheme` | 否 | - | 自定义 scheme（低优先级，一般不使用） |

> `udpxy` 和 `xteve` 二选一。都不传则使用默认 RTP 代理模式。

### 使用场景

| 场景 | URL | 说明 |
|------|-----|------|
| 默认 RTP 模式 | `/api/m3u8` | 用于 `msd_lite` / 自建 RTP 代理 |
| udpxy 代理 | `/api/m3u8?udpxy=192.168.0.1:4022` | 用于 OpenWrt udpxy |
| xteve 格式 | `/api/m3u8?xteve=true` | 用于 xteve/Threadfin |
| 强制刷新 | `/api/m3u8?ref=true` | 频道变更后立即生效 |
| 包含隐藏频道 | `/api/m3u8?all=true` | 调试用 |
| 组合使用 | `/api/m3u8?udpxy=192.168.0.1:4022&ref=true` | 多个参数可组合 |

### 成功响应

**HTTP 200** — `Content-Type: application/octet-stream`，返回 M3U 文件下载

```m3u
#EXTM3U url-tvg="http://127.0.0.1:8888/api/epg" x-tvg-url="http://127.0.0.1:8888/api/epg"
#EXTINF:-1 tvg-id="cctv1" tvg-name="CCTV-1" catchup="default" catchup-source="http://127.0.0.1:5140/rtsp/10.0.0.1:554/live/cctv1&playseek={utc:YmdHMS}-{utcend:YmdHMS}" tvg-logo="http://127.0.0.1:8888/logo/CCTV-1.png" group-title="央视",CCTV-1
http://127.0.0.1:5140/rtp/233.18.204.55:5140?fcc=124.75.25.211:7777
```

### EXTINF 属性说明

| 属性 | 说明 | 出现条件 |
|------|------|----------|
| `tvg-id` | EPG 节目单匹配 ID | 始终存在 |
| `tvg-name` | 频道显示名称 | 始终存在 |
| `tvg-logo` | 台标图片地址 | 始终存在 |
| `group-title` | 频道分组（央视/卫视/本地等） | 始终存在 |
| `catchup="default"` | 支持回看 | 仅当频道有 `TimeShiftURL` 时出现 |
| `catchup-source` | 回放流地址 | 与 `catchup` 同时出现 |

### 缓存行为

- M3U8 生成结果被缓存（默认 240 分钟）
- 频道管理操作（显隐/重命名/排序/自定义频道）会**自动清除缓存**
- 可通过 `?ref=true` 手动跳过缓存

---

## 10. 单频道 M3U8

```
GET /api/channel/m3u8?name=<频道名>
```

**用途**：获取单个频道的 M3U8 播放信息（从全量缓存中提取，无额外数据库查询）。

### 请求参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `name` | 是 | 频道名称，**精确匹配**（如 `CCTV-1`，不含 HD/4K 后缀） |

### 成功响应

**HTTP 200** — `Content-Type: text/plain; charset=utf-8`

```m3u
#EXTM3U url-tvg="http://127.0.0.1:8888/api/epg"
#EXTINF:-1 tvg-id="1" tvg-name="CCTV-1" group-title="央视",CCTV-1
http://127.0.0.1:5140/rtp/233.18.204.1:5140
```

### 错误响应

**HTTP 400** — 缺少 name 参数

```
缺少 name 参数
```

**HTTP 404** — 频道不存在

```
未找到频道: XXXXX
```

> 💡 匹配逻辑：先匹配 `tvg-name="name"`，再匹配行尾 `,name`，**精确匹配**避免子串问题（如 `CCTV-1` 不会误匹配 `CCTV-10`）。

---

## 11. EPG 配置管理

### 11.1 获取配置

```
GET /api/epg/config
```

**用途**：读取当前 EPG 相关配置。

#### 请求参数

无

#### 成功响应

**HTTP 200**

```json
{
  "generator": "Deny",
  "source": "Shanghai Telecom Iptv Spider",
  "xml_url": "http://127.0.0.1:8888/api/epg",
  "rtsp_url": "http://127.0.0.1:5140/rtsp/",
  "rtp_url": "http://127.0.0.1:5140/rtp/",
  "logo_url": "http://127.0.0.1:8888/logo/",
  "fetch_cron": "0 0 9,17,22 * * *",
  "playseek": "&playseek={utc:YmdHMS}-{utcend:YmdHMS}",
  "log_level": "info",
  "api_external_access": true
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `generator` | string | XMLTV `<generator>` 标签值 |
| `source` | string | XMLTV `<source>` 标签值 |
| `xml_url` | string | EPG XML 地址（M3U 头 `url-tvg` 值） |
| `rtsp_url` | string | **RTSP 代理地址（回放流前缀）** |
| `rtp_url` | string | **RTP 代理地址（直播流前缀）** |
| `logo_url` | string | 频道图标 URL 前缀 |
| `fetch_cron` | string | 节目单定时拉取 Cron 表达式 |
| `playseek` | string | 时移回看参数模板（支持 `{utc}` / `{utcend}` 占位符） |
| `log_level` | string | 日志级别（`debug`/`info`/`warn`/`error`） |
| `api_external_access` | bool | 是否允许外部直接调用 API（关闭后写操作仅允许管理页面操作） |

---

### 11.2 更新配置

```
POST /api/epg/config
Content-Type: application/json
```

**用途**：在线修改 EPG 配置，实时生效，持久化到数据库。

#### 请求参数

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `generator` | string | 否 | `"Deny"` | XMLTV 生成器名称 |
| `source` | string | 否 | `"Shanghai Telecom Iptv Spider"` | XMLTV 源名称 |
| `xml_url` | string | 否 | `"http://127.0.0.1:8888/api/epg"` | EPG XML 地址 |
| `rtsp_url` | string | 否 | `"http://127.0.0.1:5140/rtsp/"` | **RTSP 代理地址** |
| `rtp_url` | string | 否 | `"http://127.0.0.1:5140/rtp/"` | **RTP 代理地址** |
| `logo_url` | string | 否 | - | Logo URL 前缀 |
| `fetch_cron` | string | 否 | `"0 0 9,17,22 * * *"` | Cron 表达式（秒级解析器） |
| `playseek` | string | 否 | `"&playseek={utc:YmdHMS}-{utcend:YmdHMS}"` | 回看参数模板 |
| `log_level` | string | 否 | `"info"` | 日志级别 |
| `api_external_access` | bool | 否 | `true` | 是否允许外部直接调用 API（关闭后写操作仅允许管理页面操作） |

#### 成功响应

**HTTP 200**

```json
{
  "success": true,
  "fetch_cron_changed": false
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `success` | bool | 是否保存成功 |
| `fetch_cron_changed` | bool | Cron 表达式是否变更（变更时会热更新定时任务） |

#### 错误响应

**HTTP 400** — 参数解析失败

```json
{ "error": "参数解析失败" }
```

**HTTP 400** — Cron 表达式无效

```json
{ "error": "Cron 表达式无效: 解析错误详情" }
```

**HTTP 400** — 日志级别无效

```json
{ "error": "日志级别无效，可选值: debug/info/warn/error" }
```

#### 配置生效说明

| 字段 | 生效方式 | 说明 |
|------|----------|------|
| `fetch_cron` | **热更新** | 立即替换定时任务，无需重启 |
| `log_level` | **热更新** | 同时调整 Zap 和 GORM SQL 日志级别 |
| `rtsp_url` / `rtp_url` | **清除缓存** | 自动清除 M3U 缓存，下次请求时再生效 |
| 其他 | **立即生效** | 存入全局配置和数据库 |

> `rtsp_url` 控制回放流地址前缀，`rtp_url` 控制直播流地址前缀，两个独立配置互不影响。

---

## 12. 频道管理

### 12.1 频道列表

```
GET /api/channel/list
```

**用途**：获取所有频道（含 IPTV 频道和自定义频道），用于频道管理界面。

#### 请求参数

无

#### 成功响应

**HTTP 200**

```json
[
  {
    "comm_name": "CCTV-1",
    "name": "CCTV-1 HD",
    "mix_no": "1",
    "is_show": true,
    "is_hd": true,
    "is_4k": false,
    "custom_name": "中央一套",
    "sort_order": 1,
    "is_custom": false,
    "tvg_id": "cctv1",
    "igmp": "",
    "logo": "http://127.0.0.1:8888/logo/CCTV-1.png",
    "group": "央视",
    "effective_show": true,
    "shadowed": false,
    "shadowed_by": ""
  }
]
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `comm_name` | string | 通用频道名（去 HD/4K 后缀） |
| `name` | string | 原始频道名 |
| `mix_no` | string | 用户频道映射号 |
| `is_show` | bool | 该行自身的显示开关（数据库原值） |
| `is_hd` | bool | 是否高清频道 |
| `is_4k` | bool | 是否 4K 频道 |
| `custom_name` | string | 用户自定义名称（为空则使用 comm_name） |
| `sort_order` | int | 排序权重（越小越靠前，0=默认排序） |
| `is_custom` | bool | 是否自定义频道 |
| `tvg_id` | string | EPG 节目单匹配 ID |
| `igmp` | string | IGMP 组播地址（自定义频道） |
| `logo` | string | Logo 图片地址 |
| `group` | string | 频道分组 |
| `effective_show` | bool | **实际是否输出**：M3U 生成时会按 `comm_name` 去重（4K > HD > 其它），只有被保留的那一个变体决定输出；本字段即该分组的最终显示状态 |
| `shadowed` | bool | 本行是否被同组的 HD/4K 变体覆盖（`true` 时本行不会出现在 M3U 中，即使 `is_show=true`） |
| `shadowed_by` | string | 覆盖本行的变体名称（`shadowed=false` 时为空串） |

> **关于“被覆盖”**：同一 `comm_name` 下通常同时存在 SD 与 HD 两行（例如 `CCTV-1` 与 `CCTV-1 HD`）。
> M3U 只会输出其中一行（4K > HD > 其它），另一行即使 `is_show=true` 也不会出现在播放列表里。
> 因此判断“这个频道到底有没有输出”要看 `effective_show`，而不是裸 `is_show`；
> `shadowed=true` 的行表示它被同组变体顶掉了。管理面板对这类行显示为「被覆盖」。

---

### 12.2 频道显隐开关

```
POST /api/channel/toggle
Content-Type: application/json

{ "comm_name": "CCTV-1" }
```

**用途**：切换频道在 M3U 中的显示/隐藏状态。操作自动清除 M3U 缓存。
同一 `comm_name` 下的所有变体（SD/HD/4K）会**整组一起翻转**，
因为 M3U 只会输出其中一个变体，逐行翻转没有意义。

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `comm_name` | string | 是 | 频道通用名称 |

#### 成功响应

**HTTP 200**

```json
{
  "success": true,
  "comm_name": "CCTV-1",
  "is_show": false,
  "effective_show": false,
  "variants": 2
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `is_show` | bool | 该分组第一行自身的开关值（数据库原值） |
| `effective_show` | bool | **实际是否输出**（去重保留的那个变体的开关值），面板据此提示 |
| `variants` | int | 该 `comm_name` 下的变体数量（SD/HD/4K 行数） |

#### 错误响应

**HTTP 400** — 参数错误

```json
{ "error": "参数错误" }
```

**HTTP 404** — 频道不存在

```json
{ "error": "频道不存在" }
```

---

### 12.3 频道重命名

```
POST /api/channel/rename
Content-Type: application/json

{ "comm_name": "CCTV-1", "custom_name": "中央一套" }
```

**用途**：为频道设置自定义显示名称。操作自动清除 M3U 缓存。

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `comm_name` | string | 是 | 频道通用名称 |
| `custom_name` | string | 是 | 自定义显示名称（为空则恢复默认） |

#### 成功响应

**HTTP 200**

```json
{
  "success": true,
  "comm_name": "CCTV-1",
  "custom_name": "中央一套"
}
```

#### 错误响应

**HTTP 400** — 参数错误

```json
{ "error": "参数错误" }
```

---

### 12.4 频道排序

```
POST /api/channel/sort
Content-Type: application/json

{
  "orders": [
    { "comm_name": "CCTV-1", "sort_order": 1 },
    { "comm_name": "CCTV-2", "sort_order": 2 }
  ]
}
```

**用途**：批量设置频道排序。操作自动清除 M3U 缓存。

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `orders` | array | 是 | 排序列表 |
| `orders[].comm_name` | string | 是 | 频道通用名称 |
| `orders[].sort_order` | int | 是 | 排序权重（越小越靠前，0=默认） |

#### 成功响应

**HTTP 200**

```json
{
  "success": true,
  "count": 2
}
```

#### 错误响应

**HTTP 400** — 参数错误

```json
{ "error": "参数错误" }
```

---

### 12.5 自定义频道管理

> 自定义频道存储在 `m3u8_mappings` 表（`is_custom=true`），会出现在 M3U 输出中。

#### 12.5.1 添加自定义频道

```
POST /api/channel/custom/add
Content-Type: application/json

{
  "name": "BesTV4K电影",
  "igmp": "igmp://233.18.204.169:5140",
  "tvg_id": "721",
  "logo": "BesTV4K电影.png",
  "group": "BesTV4K电影"
}
```

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 频道名称（自动转大写） |
| `igmp` | string | 是 | IGMP 组播地址 |
| `tvg_id` | string | 否 | EPG 匹配 ID |
| `logo` | string | 否 | Logo 路径或完整 URL（非 HTTP 开头会自动拼接 `logo_url` 前缀） |
| `group` | string | 否 | 分组名称（为空则自动分配） |

**成功响应（HTTP 200）：**
```json
{
  "success": true,
  "data": { "comm_name": "BESTV4K电影", ... }
}
```

**错误响应：**

**HTTP 400** — 参数错误
```json
{ "error": "参数错误：name 和 igmp 为必填" }
```

**HTTP 409** — 频道名已存在
```json
{ "error": "频道名称已存在" }
```

**HTTP 500** — 数据库错误
```json
{ "error": "创建失败: 具体错误信息" }
```

---

#### 12.5.2 更新自定义频道

```
POST /api/channel/custom/update
Content-Type: application/json

{
  "comm_name": "BESTV4K电影",
  "igmp": "igmp://233.18.204.170:5140",
  "tvg_id": "722",
  "logo": "new_logo.png",
  "group": "新分组"
}
```

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `comm_name` | string | 是 | 频道通用名称 |
| `igmp` | string | 否 | IGMP 地址（不传则不更新） |
| `tvg_id` | string | 否 | EPG 匹配 ID |
| `logo` | string | 否 | Logo 路径 |
| `group` | string | 否 | 分组名称 |

**成功响应（HTTP 200）：**
```json
{ "success": true }
```

**错误响应：**

**HTTP 400** — 参数错误
```json
{ "error": "参数错误" }
```

**HTTP 404** — 频道不存在
```json
{ "error": "频道不存在" }
```

---

#### 12.5.3 删除自定义频道

```
POST /api/channel/custom/delete
Content-Type: application/json

{ "comm_name": "BESTV4K电影" }
```

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `comm_name` | string | 是 | 频道通用名称 |

**成功响应（HTTP 200）：**
```json
{ "success": true }
```

**错误响应：**

**HTTP 400** — 参数错误
```json
{ "error": "参数错误" }
```

**HTTP 404** — 自定义频道不存在
```json
{ "error": "自定义频道不存在" }
```

---

## 13. XMLTV 节目单

```
GET /api/epg?[daysAgo=<N>]&[ref=true]
```

**用途**：生成 XMLTV 格式的 EPG 节目单，可导入 TiviMate、Kodi、Emby/Jellyfin 等。

### 请求参数

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `daysAgo` | 否 | `1` | 拉取几天前至今的数据（`1` 表示昨天+今天，`7` 表示7天前至今） |
| `ref` | 否 | - | 设为 `true` 跳过缓存强制刷新 |

### 成功响应

**HTTP 200** — `Content-Type: application/xml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<tv generator-info-name="Deny" generator-info-url="https://github.com/jjcszxh/sh-tel-iptv-spider">
  <channel id="cctv1">
    <display-name>CCTV-1</display-name>
    <icon src="http://127.0.0.1:8888/logo/CCTV-1.png"/>
  </channel>
  <programme start="20260615060000 +0800" stop="20260615070000 +0800" channel="cctv1">
    <title>朝闻天下</title>
    <desc>早间新闻节目</desc>
  </programme>
</tv>
```

### 使用示例

```bash
# 获取 3 天内的节目单
curl http://127.0.0.1:8888/api/epg?daysAgo=3 -o epg.xml

# 获取 7 天内的节目单并强制刷新
curl "http://127.0.0.1:8888/api/epg?daysAgo=7&ref=true" -o epg7.xml

# TiviMate 中配置 EPG 源
http://192.168.0.100:8888/api/epg?daysAgo=3
```

---

## 14. 日志管理

### 14.1 获取日志级别

```
GET /api/admin/log-level
```

#### 成功响应

**HTTP 200**

```json
{ "level": "info" }
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `level` | string | 当前日志级别 |

---

### 14.2 设置日志级别

```
POST /api/admin/log-level
Content-Type: application/json

{ "level": "debug" }
```

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `level` | string | 是 | 目标日志级别：`debug` / `info` / `warn` / `error` |

#### 成功响应

**HTTP 200**

```json
{ "success": true, "level": "debug" }
```

#### 错误响应

**HTTP 400** — 参数错误

```json
{ "error": "参数解析失败" }
```

**HTTP 400** — 无效级别

```json
{ "error": "无效的日志级别，可选值: debug/info/warn/error" }
```

#### 日志级别说明

| 级别 | 用途 | GORM SQL 输出 |
|------|------|---------------|
| `debug` | 开发调试，输出所有日志 | 输出所有 SQL |
| `info` | 正常运行，输出关键信息 | 输出所有 SQL |
| `warn` | 仅输出警告和错误 | 输出慢 SQL (>200ms) |
| `error` | 仅输出错误 | 仅输出错误 SQL |

> 动态调整，立即生效，**同时影响 Zap 日志和 GORM SQL 日志**。设置会被持久化到数据库，重启后保持。

---

## 15. SSE 实时日志流

```
GET /api/log/stream
```

**用途**：通过 Server-Sent Events 实时推送应用日志，用于 Web 终端实时查看。

### 请求参数

无

### 连接行为

1. 客户端建立 SSE 连接
2. 服务端立即推送最近 **150 行**历史日志
3. 之后每 **300ms** 检查日志文件增量，推送新增行
4. 日志文件轮转时自动从新文件头部开始读取

### 事件格式

```
data: 2026-06-15 14:30:00 INFO  [iptv-spider-sh] 频道列表拉取完成

data: 2026-06-15 14:30:05 ERROR [iptv-spider-sh] EPG 拉取失败: timeout
```

### 前端使用

```javascript
const evt = new EventSource("/api/log/stream");
evt.onmessage = (e) => {
    console.log(e.data);         // 输出到控制台
    logContainer.innerHTML += `<div>${e.data}</div>`;  // 渲染到页面
};
evt.onerror = () => {
    evt.close();
    setTimeout(() => location.reload(), 5000);  // 5 秒后重连
};
```

### 限制

- 仅推送当前日期的日志文件（`log/2026-06-15.log`）
- 跨天时不会自动切换文件，需要重新连接
- 浏览器标签页关闭后连接自动断开

---

## 附录A：数据库表结构

项目使用 GORM AutoMigrate 自动管理表结构，共 **6 张表**。

### channels — 频道播放信息

| 字段 | 类型 | 说明 |
|------|------|------|
| `user_channel_id` | string (PK) | 用户频道 ID |
| `channel_id` | string | 频道 ID |
| `channel_url` | string | 频道播放地址（IGMP） |
| `time_shift` | string | 回放功能开关（默认 `0`） |
| `channel_sdp` | string | 回放服务器 SDP 信息 |
| `time_shift_url` | string(256) | 回放服务 URL 地址 |
| `channel_type` | string | 频道类型 |
| `channel_fcc_port` | string | FCC 端口 |
| `channel_fcc_ip` | string | FCC IP 地址 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |
| `deleted_at` | datetime | 软删除时间 |

### channel_infos — 频道信息 / EPG 频道

| 字段 | 类型 | 说明 |
|------|------|------|
| `mix_no` | string (PK) | 用户频道映射号 |
| `ts_time` | int | 时移时间长度 |
| `code` | string | 频道代码 |
| `auth_code` | string | 付费认证代码 |
| `name` | string | 频道名称 |
| `ch_id` | string | 频道 ID |
| `media_id` | string | 媒体 ID |
| `is_ts` | string | 是否支持回放 |
| `is_charge` | string | 是否需要付费 |
| `is_hd` | bool | 是否高清频道 |
| `is_4k` | bool | 是否 4K 频道 |
| `is_pull_epg` | bool | 是否拉取节目单 |
| `is_show` | bool | 是否在 M3U 中显示 |
| `comm_name` | string | 通用标题（去 HD/4K 后缀） |
| `last_fetch_time` | datetime | 节目单最后拉取时间 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |
| `deleted_at` | datetime | 软删除时间 |

> GORM 钩子 `BeforeCreate`/`BeforeUpdate` 自动分析频道名，设置 `is_hd`、`is_4k`、`comm_name`，并同步到 `m3u8_mappings` 表。

### m3u8_mappings — M3U8 映射表

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uint (PK) | 自增 ID |
| `comm_name` | string (UNI) | 节目通用名称 |
| `logo` | string | Logo URL |
| `auto_groups` | string | 自动分组（系统根据频道名自动分类） |
| `custom_groups` | string | 自定义分组 |
| `custom_name` | string | 自定义频道显示名称 |
| `sort_order` | int | 排序权重（越小越靠前，0=默认排序） |
| `is_custom` | bool | 是否为自定义频道 |
| `tvg_id` | string | tvg-id 标识 |
| `igmp` | string | IGMP 组播地址（自定义频道用） |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |
| `deleted_at` | datetime | 软删除时间 |

### auth_infos — 认证信息

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uint (PK) | 自增 ID |
| `uid` | string (UNI) | 用户 ID |
| `user_token` | string | 用户 Token |
| `jsessionid` | string | BIM EPG 会话 ID |
| `bim_auth_info` | longtext | BIM 认证信息（JSON） |
| `epg_host_url` | string | EPG 服务器地址 |
| `epg_login_host` | string | EPG 认证服务器主机 |
| `r4k_login_host` | string | R4K 服务器主机 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |
| `deleted_at` | datetime | 软删除时间 |

### epg_details — EPG 节目详情

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uint (PK) | 自增 ID |
| `comm_name` | string (IDX) | 节目通用名称 |
| `inter_record_status` | string | 未知字段 |
| `name` | string | 节目名称 |
| `start_time` | int64 | 开始时间（Unix 时间戳） |
| `end_time` | int64 (IDX) | 结束时间（Unix 时间戳） |
| `status` | string | 状态 |
| `program_id` | string | 节目 ID |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |
| `deleted_at` | datetime | 软删除时间 |

### epg_configs — EPG 配置（单行配置表）

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `id` | uint (PK) | `1`（固定） | 主键 |
| `generator` | string(255) | `Deny` | XMLTV 生成器名称 |
| `source` | string(255) | `Shanghai Telecom Iptv Spider` | 数据源名称 |
| `xml_url` | string(512) | - | EPG XML 地址 |
| `rtsp_url` | string(512) | - | RTSP 代理地址（回放流） |
| `rtp_url` | string(512) | - | RTP 代理地址（直播流） |
| `logo_url` | string(512) | - | Logo 前缀地址 |
| `fetch_cron` | string(64) | `0 0 9,17,22 * * *` | 节目单定时抓取 Cron |
| `playseek` | string(255) | `&playseek={utc:YmdHMS}-{utcend:YmdHMS}` | 回放 seek 参数模板 |
| `log_level` | string(16) | `info` | 日志级别 |
| `api_external_access` | bool | `true` | 是否允许外部直接调用 API（关闭后写操作仅允许管理页面操作） |

---

## 附录B：M3U8 输出格式说明

### URL 拼接模式

播放地址由参数决定，优先级从高到低：

| 优先级 | 模式 | 条件 | 输出示例 |
|--------|------|------|----------|
| 1 | xteve | `?xteve=true` | `udp://@233.18.204.55:5140` |
| 2 | udpxy | `?udpxy=IP:PORT` | `http://192.168.0.1:4022/udp/233.18.204.55:5140` |
| 3 | RTP + FCC | 默认（有 FCC 数据） | `http://127.0.0.1:5140/rtp/233.18.204.55:5140?fcc=124.75.25.211:7777` |
| 4 | RTP 无 FCC | 默认（无 FCC 数据） | `http://127.0.0.1:5140/rtp/233.18.204.55:5140` |

### 直播 vs 回放分流

| 类型 | 配置项 | 默认值 | 出现位置 |
|------|--------|--------|----------|
| 直播流 | `rtp_url` | `http://127.0.0.1:5140/rtp/` | `#EXTINF` 下方的播放地址行 |
| 回放流 | `rtsp_url` | `http://127.0.0.1:5140/rtsp/` | `catchup-source` 属性值 |

### 回放 URL 拼接规则

```
catchup-source = rtsp_url + TimeShiftURL(去掉 rtsp:// 前缀) + playseek 模板
```

**示例：**

```
rtsp_url     = http://127.0.0.1:5140/rtsp/
TimeShiftURL = rtsp://10.0.0.1:554/live/cctv1
playseek     = &playseek={utc:YmdHMS}-{utcend:YmdHMS}
-------------------------------------------------------------------
catchup-source = http://127.0.0.1:5140/rtsp/10.0.0.1:554/live/cctv1&playseek={utc:YmdHMS}-{utcend:YmdHMS}
```

播放器会将 `{utc:YmdHMS}` 替换为回看起始时间（UTC），`{utcend:YmdHMS}` 替换为结束时间。

---

## 附录C：API 优化建议

当前 API 设计存在一些可改进点，以下是具体建议：

### C.1 统一错误响应格式

**现状**：不同接口返回的错误格式不统一，有的返回 JSON，有的返回纯文本，有的直接返回 HTTP 状态码无 body。

**建议**：引入统一错误码体系，所有接口使用一致的结构：

```json
{
  "code": 40001,
  "message": "缺少必填参数: name",
  "detail": "请求 /api/channel/m3u8 缺少 name 参数"
}
```

**建议的错误码分段：**

| 范围 | 类别 | 示例 |
|------|------|------|
| 0 | 成功 | `{"code": 0, "message": "ok"}` |
| 40001-40099 | 参数错误 | `40001` 缺少必填参数, `40002` 参数格式错误, `40003` 参数值不合法 |
| 40100-40199 | 认证错误 | `40101` API Key 缺失, `40102` API Key 无效 |
| 40300-40399 | 权限错误 | `40301` 禁止外部调用, `40302` 管理员权限不足 |
| 40400-40499 | 资源不存在 | `40401` 频道不存在, `40402` 配置不存在 |
| 40900-40999 | 资源冲突 | `40901` 频道名已存在 |
| 50001-50099 | 服务器错误 | `50001` 数据库错误, `50002` 文件系统错误 |
| 50301-50399 | 服务不可用 | `50301` 数据库断开, `50302` 认证服务不可用 |

**实现方式**：创建一个 `apperr` 包，定义标准错误类型；添加一个 API 中间件统一捕获和格式化错误响应。

---

### C.2 API 访问控制 ✅ 已实现

**实现方案**：采用 Web 界面开关（方案 C）+ Referer/Origin 校验（方案 A），已在 `router/api/middleware.go` 中实现 `ApiGuardMiddleware`。

在 `epg_configs` 表中增加了 `api_external_access` 字段（bool，默认 `true`），Web 管理页面 EPG 配置弹窗中可开关：

| 开关状态 | 行为 |
|----------|------|
| 开启（默认） | 所有接口对外开放 |
| 关闭 | 读操作始终开放；写操作接口仅允许 `Referer`/`Origin` 来源为 `status.html` 的请求 |

**受保护的敏感接口**（开关关闭时返回 403）：
- `/api/run`、`/api/self-upgrade`
- `/api/channel/toggle`、`/api/channel/rename`、`/api/channel/sort`
- `/api/channel/custom/add`、`/api/channel/custom/update`、`/api/channel/custom/delete`
- `POST /api/epg/config`、`POST /api/admin/log-level`

**以下为未实现的备选方案，供参考：**

**备选方案 B — API Key 认证（更安全）：**

```go
// 在请求头中校验
func APIKeyMiddleware(ctx iris.Context) {
    key := ctx.GetHeader("X-API-Key")
    if key != global.CONFIG.System.APIKey {
        ctx.StatusCode(401)
        ctx.JSON(map[string]string{"error": "无效的 API Key"})
        return
    }
    ctx.Next()
}
```

只读接口（`/api/m3u8`、`/api/epg`、`/api/health`）可以不加 Key，写操作才需要。

---

### C.3 速率限制（Rate Limiting）

**现状**：无速率限制，可能被恶意刷接口导致资源耗尽。

**建议**：使用 token bucket 算法对高频接口进行限流。

| 接口 | 限制 | 说明 |
|------|------|------|
| `/api/m3u8` | 10 次/分钟 | 播放列表 |
| `/api/epg` | 5 次/分钟 | 节目单（生成开销大） |
| `/api/health` | 60 次/分钟 | 监控轮询 |
| `/api/run` | 2 次/分钟 | 手动触发任务 |
| `/api/self-upgrade` | 1 次/5分钟 | 远程升级 |

**实现方式**：使用 `golang.org/x/time/rate` 或 `github.com/ulule/limiter` 实现内存限流。

---

### C.4 批量频道操作

**现状**：频道管理接口一次只能操作一个频道（toggle/rename/sort 除外），如果用户想批量隐藏频道需要多次调用。

**建议**：增加批量操作接口：

```
POST /api/channel/batch/toggle
Content-Type: application/json

{
  "comm_names": ["CCTV-1", "CCTV-2", "CCTV-3"],
  "is_show": false
}
```

---

### C.5 API 响应分页

**现状**：`/api/channel/list` 一次性返回所有频道（可能 200+ 个），无分页。

**建议**：对于频道列表等大数据量接口支持分页：

```
GET /api/channel/list?page=1&page_size=50
```

响应中增加分页信息：
```json
{
  "channels": [...],
  "pagination": {
    "page": 1,
    "page_size": 50,
    "total": 230,
    "total_pages": 5
  }
}
```

---

### C.6 M3U8 格式增强

**现状**：M3U8 输出格式固定，不支持按分组筛选。

**建议**：增加可选参数：

```
GET /api/m3u8?group=央视          # 只输出指定分组
GET /api/m3u8?hd=true            # 只输出高清频道
GET /api/m3u8?sort=name           # 按名称排序（默认按 sort_order）
```

---

### C.7 API 文档自动生成

**建议**：代码中添加 Swagger/OpenAPI 注解，自动生成交互式 API 文档页面。

使用 `github.com/swaggo/swag` + `github.com/iris-contrib/swagger` 集成 Swagger UI：

```go
// @Summary 健康检查
// @Tags 系统
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /api/health [get]
func health(ctx iris.Context) { ... }
```

访问 `http://host:8888/swagger/index.html` 即可查看交互式 API 文档。

---

### C.8 安全增强清单

| 项目 | 优先级 | 说明 |
|------|--------|------|
| API 访问控制 | 🔴 高 | 防止敏感接口被外部随意调用 |
| 速率限制 | 🟡 中 | 防止恶意高频请求 |
| HTTPS 支持 | 🟡 中 | 如果暴露公网，建议使用反向代理加 TLS |
| 日志脱敏 | 🟢 低 | 日志中不输出用户 Token 和密码 |
| 请求大小限制 | 🟢 低 | POST body 限制（如 1MB）防止 OOM |
