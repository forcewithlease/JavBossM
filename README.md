<h1 align="center">LocalJav</h1>

<p align="center"><strong>自建 JAV 媒体库 · 自动刮削 · 移动端播放 · Docker / NAS 部署</strong></p>

<p align="center">
  <a href="LICENSE"><img alt="License: GPL-3.0" src="https://img.shields.io/badge/License-GPL--3.0-blue.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white">
  <img alt="React" src="https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=white">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white">
  <img alt="Platform" src="https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-1E88E5">
  <img alt="PRs Welcome" src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg">
</p>

LocalJav 是一个**可自托管的 JAV / 视频媒体库**：自动扫描本地目录、识别番号、抓取 JAV 元数据、生成封面与截图，提供标签、收藏、筛选、瀑布流浏览，以及桌面和移动端浏览器播放。适合在 NAS、家庭服务器或本机上长期运行。

本项目基于开源项目 [JavBoss](https://github.com/Solr159/JavBoss) 二次开发，遵循 **GPL-3.0** 协议。相较于上游，本仓库重点增强了**移动端体验**与**本地下载器接入**。

> **关键词**：JAV 媒体库、JAV 刮削器、番号识别、JAV 管理、女优管理、自建影音库、NAS 媒体库、Docker 部署、移动端适配、aria2、qBittorrent、CloudDrive2

## 目录

- [核心特性](#核心特性)
- [移动端体验](#移动端体验)
- [下载器](#下载器)
- [快速开始](#快速开始)
- [Docker 部署](#docker-部署)
- [源码开发](#源码开发)
- [使用流程](#使用流程)
- [常见问题 FAQ](#常见问题-faq)
- [数据与备份](#数据与备份)
- [技术栈](#技术栈)
- [路线图](#路线图)
- [贡献](#贡献)
- [许可证与致谢](#许可证与致谢)

## 核心特性

- **自动扫描与增量同步**：监控本地目录，新增、删除、移动视频后自动同步媒体库，无需手动整理。
- **番号识别与元数据刮削**：从文件名识别 JAV 番号，抓取标题、女优、片商、系列、发行日期、时长、标签和封面。
- **JAV 资料库**：按女优、片商、系列、番号前缀、标签、喜爱度筛选，支持收藏夹与自定义标签。
- **视频媒体库**：按目录聚合视频，生成缩略图与截图，支持时长、分辨率、体积、观看进度展示。
- **多种浏览方式**：网格与瀑布流两种模式，无限滚动，分页与排序规则可配置。
- **网页播放**：内置浏览器播放器，支持 HLS / 直链、断点续播、播放列表、倍速、截图。
- **桌面播放器集成**：Client 模式下可调用本机 MPV，支持进度条预览等桌面能力。
- **移动端适配**：底部标签栏、搜索与筛选融合、默认瀑布流、B 站风格手势播放器。
- **本地下载器**：自动检测 aria2 / qBittorrent，未配置时回退到默认的 CloudDrive2 云盘离线下载。
- **数据自持**：所有元数据与封面保存在独立数据目录，不修改你的视频文件。
- **多平台**：Windows、Linux、macOS 原生运行，也支持 Docker / NAS 长期部署。

## 移动端体验

手机与平板通过浏览器访问同一实例即可，无需安装 App。

- **底部标签栏**：视频、JAV、女优、片商、系列一键切换，更多入口收纳在底部抽屉。
- **搜索与筛选融合**：顶部搜索框与筛选按钮合并为一个控件，收藏夹、喜爱度、女优筛选在顶部展开。
- **默认瀑布流**：移动端首次进入即使用瀑布流与无限滚动，不再显示拥挤的瀑布流开关。
- **B 站风格播放器**：
  - 单击显示 / 隐藏控制层，自动隐藏
  - 双击左 / 右 10 秒快退 / 快进
  - 左半屏上下滑调亮度，右半屏上下滑调音量
  - 长按 2 倍速，松手恢复
  - 底部进度条、倍速面板、选集抽屉、全屏
- **全屏管理页**：设置、下载等管理弹窗在手机上以全屏方式呈现。

## 下载器

下载设置路径：**下载 → 下载设置 → 下载器**。

| 下载器 | 类型 | 说明 |
| --- | --- | --- |
| CloudDrive2 | 云盘离线 | 默认下载器。云端离线完成后回传到本地下载目录，需要地址与 API Token。 |
| aria2 | 本地 | 自动检测 `127.0.0.1:6800`，通过 JSON-RPC 添加任务，直接写入下载目录。支持 RPC 密钥。 |
| qBittorrent | 本地 | 自动检测 `127.0.0.1:8080`，通过 Web API 添加任务，直接写入下载目录。支持用户名 / 密码。 |

工作方式：

1. 打开下载设置，程序会自动探测本机正在运行的下载器。
2. 选择要使用的下载器；未选择时使用默认下载器（CloudDrive2）。
3. 本地下载器需要能写入「本地下载目录」。如果下载器运行在容器或另一台机器上，请在「下载器写入目录」中填写它对同一块磁盘的挂载路径。
4. 下载完成后，LocalJav 会扫描任务目录并记录文件，无需再次搬运。

> **关于迅雷**：迅雷没有稳定的 Linux 远程控制接口，因此本项目暂不支持直接调用迅雷。当前可用的本地下载器为 aria2 与 qBittorrent。

## 快速开始

### 环境要求

- Go 1.25+
- Node.js 24+（仅前端构建）
- Docker（可选）
- FFmpeg（可选，HLS 转码与截图增强）

### 默认账号

- 用户名：`admin`
- 初始密码：`admin`（首次登录后请立即在 **设置 → 安全** 中修改）

### 默认端口

- 源码运行：`17654`
- Docker 示例：`8655`（可通过 `-port` 参数修改）

## Docker 部署

推荐使用 Docker Compose。以下示例将视频目录、数据目录和前端构建产物挂载到容器：

```yaml
services:
  localjav:
    image: localjav:latest
    container_name: localjav
    user: "1000:1000"
    network_mode: host
    command: ["./javboss", "-port", "8655"]
    environment:
      JAVBOSS_PROXY_HOST_GATEWAY: "0"
      JAVBOSS_HOST_PATH_PREFIX: "1"
      TZ: Asia/Shanghai
    volumes:
      - ./data:/app/data
      # 宿主机路径 -> 容器内 /host 下的同路径
      - /path/to/media:/host/path/to/media
      - ./web/dist:/app/web/dist:ro
    restart: unless-stopped
```

```bash
# 1. 构建镜像（在仓库根目录）
docker build -t localjav:latest .

# 2. 构建前端（如不使用镜像内置前端）
cd web && npm ci && npm run build && cd ..

# 3. 启动
docker compose up -d
```

打开 `http://<服务器 IP>:8655`，使用 `admin / admin` 登录。

**路径说明**：程序以 `/host` 为宿主机根目录。添加目录时直接填写宿主机路径，例如 `/path/to/media`，程序会自动映射到容器内的 `/host/path/to/media`。请确保 compose 中按相同路径挂载。

> 只想先体验？可以将 `image` 换成上游官方镜像 `ghcr.io/solr159/javboss:latest`，但那样不包含本仓库的移动端与本地下载器改动。

## 源码开发

```bash
# 后端（默认 17654，会自动托管 web/dist）
GOCACHE=$(pwd)/.gocache go run ./cmd/server

# 前端热更新
cd web
npm install
npm run dev
```

前端开发时默认把接口代理到 `http://localhost:17654`。如果后端跑在 Docker 的 `8655` 端口：

```bash
cd web
JAVBOSS_API_TARGET=http://127.0.0.1:8655 npm run dev -- --host 0.0.0.0
```

手机访问 `http://<电脑局域网 IP>:5173` 即可调试移动端。

### 测试与检查

```bash
# 后端
GOCACHE=$(pwd)/.gocache go test ./...

# 前端
cd web
npm run lint
npm run format:check
npm test
npm run build
```

## 使用流程

1. 打开 **设置 → 目录管理**，添加存放视频的本地文件夹。
2. 等待自动扫描完成；也可以在目录卡片上点击「手动扫描」立即触发。
3. 在 **视频** 或 **JAV** 页面浏览媒体库，点击卡片查看详情或播放。
4. 在 **下载** 页面粘贴磁力链接，选择下载器与下载目录。
5. 需要局域网访问时，在 **设置 → 网络与代理** 中确认已允许局域网访问（Docker 部署默认开启）。

## 常见问题 FAQ

### JAV 刮削失败 / 超时怎么办？

刮削依赖外部站点。如果所在网络无法直接访问，请在 **设置 → 网络与代理** 配置 HTTP 代理。Docker 部署时，若代理运行在宿主机，可填写 `127.0.0.1:7890`（`network_mode: host`）或使用 `host.docker.internal`。每次扫描只尝试一次，失败后重新扫描即可。

### 手机打不开页面？

- 确认手机和服务器在同一局域网。
- 使用服务器的局域网 IP（例如 `http://192.168.1.10:8655`），而不是 `localhost`。
- 检查防火墙是否放行端口。
- Docker 部署默认允许局域网访问；源码运行需在设置中手动开启。

### Docker 容器里看不到移动硬盘？

需要把宿主机目录挂载到容器的 `/host` 下，并保持路径一致：

```yaml
volumes:
  - /media/disk/AV:/host/media/disk/AV
```

移动硬盘重新插拔后，如果挂载点变化，请同步修改 compose 并重新扫描。

### 为什么媒体库突然空了？

如果视频目录被卸载或暂时不可访问，自动扫描会把对应文件标记为移除（JAV 元数据仍然保留）。重新挂载目录后执行一次手动扫描即可恢复。

### 支持迅雷吗？

暂不支持。迅雷缺少稳定的跨平台远程控制接口，目前可接入 aria2、qBittorrent 和 CloudDrive2。

### 下载速度慢 / 没有速度？

本地下载器的速度取决于种子健康度、网络和下载器自身配置。qBittorrent 首次使用请在下载器内确认监听端口和连接数。

### 端口被占用？

修改启动参数 `-port 端口号`，或调整 compose 中的端口与 `command`。

## 数据与备份

- 数据目录默认是程序目录下的 `data/`（Docker 中为 `/app/data`）。
- 包含 SQLite 数据库、封面、缩略图、截图与缓存。
- 备份时直接复制整个数据目录即可；视频文件不会被修改。
- 升级前建议先停止容器并备份数据目录。

## 技术栈

- **后端**：Go、Gin、GORM、SQLite、goose
- **前端**：React 18、Vite、Tailwind CSS、MUI、Zustand、video.js
- **媒体**：FFmpeg、HLS、MPV（桌面 Client 模式）
- **部署**：Docker、Docker Compose、多平台二进制

## 路线图

- [x] 移动端布局与底部导航
- [x] B 站风格移动端播放器（手势、倍速、选集）
- [x] 本地下载器自动检测（aria2 / qBittorrent）
- [ ] 更多下载器适配（Transmission 等）
- [ ] 播放器更多清晰度与字幕控制
- [ ] 移动端离线缓存（PWA）

## 贡献

欢迎提交 Issue 和 Pull Request。

- 提交前请运行 `gofmt`、`npm run lint`、`npm run format:check` 与相关测试。
- 数据库结构变更请新增 migration，不要修改历史 migration。
- 前端请按 `src/features/<domain>/` 的领域结构组织代码。

## 许可证与致谢

- 本项目基于 [Solr159/JavBoss](https://github.com/Solr159/JavBoss) 二次开发，原项目版权归原作者所有。
- 本项目与修改部分同样以 **GNU General Public License v3.0** 发布，详见 [LICENSE](LICENSE)。
- 分发修改版本时请保留版权声明、许可证文本，并提供对应源码。
- 使用本项目时请遵守所在国家或地区的法律法规，仅用于个人学习与合法的媒体管理场景。
