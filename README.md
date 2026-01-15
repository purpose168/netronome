<h1 align="center">Netronome</h1>
<p align="center">
  <strong>监控。分析。告警。</strong><br>
  一个完整的网络性能监控解决方案，具有分布式代理、实时指标和美观的可视化界面。
</p>
<div align="center">
<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24-blue?logo=go" alt="Go 版本">
  <img src="https://img.shields.io/badge/build-passing-brightgreen" alt="构建状态">
  <img src="https://img.shields.io/github/v/release/autobrr/netronome" alt="最新版本">
  </a>
    <a href="https://github.com/autobrr/netronome">
    <img src="https://img.shields.io/badge/%F0%9F%92%A1%20netronome-docs-00ACD7.svg?style=flat-square">
  </a>
</p>

[![Discord](https://img.shields.io/discord/881212911849209957.svg?label=&logo=discord&logoColor=ffffff&color=7389D8&labelColor=6A7EC2)](https://discord.gg/WehFCZxq5B)

</div>

<p align="center">
  <img src=".github/assets/netronome_dashboard.png" alt="Netronome 仪表板">
</p>

Netronome 是一个完整的网络性能监控解决方案，帮助您了解和跟踪网络健康状况。无论您是在监控家庭互联网连接、管理多站点基础设施，还是跟踪服务器性能，Netronome 都能通过直观的 Web 界面提供所需的洞察。

Netronome 使用 Go 构建，设计简洁，将前端和后端打包到单个二进制文件中，便于部署。仅占用约 35MB RAM 的最小占用空间，非常适合资源受限的环境。无需复杂的设置 - 只需下载、配置和运行。

**核心功能：** 跨多个提供商的速度测试、持续的数据包丢失监控、通过轻量级代理的分布式服务器监控以及自动化告警 - 所有功能都配有美观的可视化和历史跟踪。

## 快速开始

在 5 分钟内启动 Netronome：

### 选项 1：从发布页面下载

从[发布页面](https://github.com/autobrr/netronome/releases/latest)下载预构建的二进制文件。

### 选项 2：一键安装

```bash
# 下载最新版本
wget $(curl -s https://api.github.com/repos/autobrr/netronome/releases/latest | grep download | grep linux_x86_64 | cut -d\" -f4)
tar -C /usr/local/bin -xzf netronome*.tar.gz

# 生成默认配置
netronome generate-config

# 启动服务器
netronome serve
```

在浏览器中打开 `http://localhost:7575` 并通过注册页面创建您的账户。对于 Docker 用户，请参阅 [Docker 安装](#docker-installation) 部分。

## 目录

- [功能特性](#功能特性)
- [外部依赖](#外部依赖)
- [安装](#安装)
  - [Linux 通用安装](#linux-通用安装)
  - [Docker 安装](#docker-安装)
- [基本配置](#基本配置)
  - [首次运行设置](#首次运行设置)
  - [身份验证](#身份验证)
  - [数据库](#数据库)
- [高级配置](#高级配置)
  - [系统监控](#系统监控)
  - [数据包丢失监控](#数据包丢失监控)
  - [Tailscale 集成](#tailscale-集成)
  - [Docker 代理集成](#docker-代理集成)
  - [GeoIP 配置](#geoip-配置)
  - [通知](#通知)
  - [调度](#调度)
- [参考](#参考)
  - [环境变量](#环境变量)
  - [CLI 命令](#cli-命令)
- [常见问题与故障排除](#常见问题与故障排除)
- [从源代码构建](#从源代码构建)
- [贡献](#贡献)
- [许可证](#许可证)

## 功能特性

### 核心功能

- **速度测试**：多个提供商（Speedtest.net、iperf3、LibreSpeed），具有实时进度和历史跟踪
- **网络诊断**：Traceroute 和持续的数据包丢失监控，集成 MTR
- **系统监控**：部署代理进行分布式服务器监控，具有实时指标
- **灵活调度**：自动化测试，具有可自定义的间隔和智能抖动预防

### 网络诊断

<p align="center">
  <img src=".github/assets/packetloss-monitors(mtr).png" alt="网络诊断界面">
</p>

高级网络路径分析，具有：

- 跨平台 traceroute 支持
- 持续 ICMP 监控
- 每跳数据包丢失统计
- 带有国家标志的 GeoIP 可视化
- 历史性能跟踪

### 系统监控

<p align="center">
  <img src=".github/assets/agents-dashboard.png" alt="系统监控仪表板">
</p>

<p align="center">
  <img src=".github/assets/agents-bandwidth.png" alt="带宽监控">
</p>

<p align="center">
  <img src=".github/assets/agents-systeminfo.png" alt="代理系统信息">
</p>

从一个仪表板监控多个服务器：

- CPU、内存、磁盘和温度指标
- 实时带宽监控（vnstat）
- Tailscale 网络的自动发现
- 可配置的告警阈值
- 通过 SSE 的实时数据流
- 代理也是单个二进制文件 - 同样简单的部署

### 附加功能

- **现代 UI**：响应式设计，支持深色模式
- **身份验证**：内置身份验证、OIDC 支持、IP 白名单
- **通知**：通过 Shoutrrr 支持 15+ 种服务（Discord、Telegram、Email 等）
- **数据库支持**：SQLite（默认）或 PostgreSQL
- **Tailscale 集成**：无需端口暴露的安全网状网络

### 技术概述

- **单个二进制文件**：前端和后端编译到一个可执行文件中（约 66MB）
- **语言**：使用 Go 编写，以获得高性能和易于部署
- **前端**：React with TypeScript，嵌入在二进制文件中
- **数据库**：默认使用 SQLite，可选 PostgreSQL
- **无运行时依赖**：只需要二进制文件和可选的外部工具

## 系统要求

### 系统要求

- **操作系统**：Linux、macOS 或 Windows
- **架构**：x86_64、ARM64
- **内存**：约 35MB（典型使用情况）
- **磁盘空间**：应用程序 65MB + 数据库增长空间

### 外部依赖

以下工具启用特定功能（Docker 中自动包含）：

- **iperf3** - 用于 iperf3 速度测试
- **librespeed-cli** - 用于 LibreSpeed 测试
- **traceroute** - 用于基本网络路径发现（通常预装）
- **mtr** - 用于每跳的高级数据包丢失分析（可选，回退到 traceroute）
  - Windows 用户应从 https://github.com/dqos/WinMTRCmd/releases 获取二进制文件
- **vnstat** - 用于代理上的带宽监控（可选但推荐）

在 Linux 上安装：

```bash
# Debian/Ubuntu
sudo apt-get install iperf3 traceroute mtr vnstat

# RHEL/Fedora
sudo dnf install iperf3 traceroute mtr vnstat
```

注意事项：

- Speedtest.net 是内置的
- 所有外部依赖都是可选的 - Netronome 优雅地处理缺失的工具

## 安装

### Linux 通用安装

1. **下载和安装**

   ```bash
   wget $(curl -s https://api.github.com/repos/autobrr/netronome/releases/latest | grep download | grep linux_x86_64 | cut -d\" -f4)
   tar -C /usr/local/bin -xzf netronome*.tar.gz
   ```

2. **创建 Systemd 服务**（推荐）

   ```bash
   sudo tee /etc/systemd/system/netronome@.service > /dev/null <<EOF
   [Unit]
   Description=netronome service for %i
   After=syslog.target network-online.target

   [Service]
   Type=simple
   User=%i
   Group=%i
   ExecStart=/usr/local/bin/netronome serve --config=/home/%i/.config/netronome/config.toml

   [Install]
   WantedBy=multi-user.target
   EOF
   ```

3. **启用和启动**
   ```bash
   systemctl enable --now netronome@$USER
   ```

### Windows 通用安装

1. **下载和安装**
```cmd
从 https://github.com/autobrr/netronome/releases 获取最新的二进制文件
将发布版本解压到一个文件夹
将任何第三方二进制文件放在同一文件夹中
将文件夹添加到 Windows 环境变量 - 重启 explorer.exe（以及任何打开的终端）
```

2. **创建 Windows 任务计划**
```cmd
https://www.windowscentral.com/how-create-automated-task-using-task-scheduler-windows-10
```

3. **创建配置**
```cmd
运行 `netronome generate-config`
编辑 config.toml 以适应 `C:\Users\{USERNAME}\.config\netronome`
```

### Docker 安装

快速 Docker 部署，自动安装依赖：

```bash
# 克隆仓库（用于 docker-compose 文件）
git clone https://github.com/autobrr/netronome.git
cd netronome

# 使用 SQLite 的基本设置
docker-compose up -d

# 或使用 PostgreSQL 以获得更好的性能
docker-compose -f docker-compose.postgres.yml up -d
```

Docker 镜像包含所有依赖项（iperf3、librespeed-cli、traceroute、mtr、vnstat）预安装，因此您无需单独安装它们。对于与 Docker 的 Tailscale 集成，请参阅 [Docker Tailscale Sidecar 指南](docs/docker-tailscale-sidecar.md)。

## 基本配置

### 首次运行设置

1. **生成配置**

   ```bash
   netronome generate-config
   ```

   这将创建 `~/.config/netronome/config.toml`，包含默认设置。

2. **启动服务器**

   ```bash
   netronome serve
   ```

3. **访问界面**
   导航到 `http://localhost:7575` 并通过 Web 界面注册您的账户。

   要从网络上的其他设备访问，请将 config.toml 中的主机从 `127.0.0.1` 更改为 `0.0.0.0`。

### 身份验证

Netronome 支持多种身份验证方法：

#### 内置身份验证

用户可以在首次访问时直接通过 Web 界面注册。对于自动化或管理目的，您也可以通过 CLI 管理用户：

```bash
netronome create-user <username>     # 通过 CLI 创建用户
netronome change-password <username>  # 通过 CLI 更改密码
```

#### OpenID Connect (OIDC)

通过环境变量配置：

```bash
export NETRONOME__OIDC_ISSUER=https://your-provider.com
export NETRONOME__OIDC_CLIENT_ID=your-client-id
export NETRONOME__OIDC_CLIENT_SECRET=your-client-secret
export NETRONOME__OIDC_REDIRECT_URL=https://netronome.example.com/api/auth/oidc/callback
```

#### IP 白名单

添加到 `config.toml`：

```toml
[auth]
whitelist = ["127.0.0.1/32", "192.168.1.0/24"]
```

### 数据库

#### SQLite（默认）

无需额外设置。数据库文件自动创建。

#### PostgreSQL

通过环境变量配置：

```bash
export NETRONOME__DB_TYPE=postgres
export NETRONOME__DB_HOST=localhost
export NETRONOME__DB_PORT=5432
export NETRONOME__DB_USER=postgres
export NETRONOME__DB_PASSWORD=your-password
export NETRONOME__DB_NAME=netronome
```

### 使用基础 URL 的反向代理

要在 nginx 后面的子路径（例如 `/netronome`）下提供 Netronome：

#### 1. 配置 Netronome

在 `config.toml` 中设置基础 URL：

```toml
[server]
host = "127.0.0.1"  # 仅在 localhost 上监听，因为 nginx 将代理
port = 7575
base_url = "/netronome"  # 您要使用的子路径
```

#### 2. 配置 nginx

将此 location 块添加到您的 nginx 配置：

```nginx
# 将 /netronome 重定向到 /netronome/
location = /netronome {
    return 301 /netronome/;
}

location /netronome/ {
    proxy_pass http://127.0.0.1:7575;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 86400;
}
```

就是这样！上面的最小配置处理了 WebSocket/SSE 以实现实时功能。

## 常见用例

### 家庭网络监控

监控您的互联网连接质量：

1. 安排每小时对您的 ISP 进行速度测试
2. 设置对 `8.8.8.8` 或 `1.1.1.1` 的数据包丢失监控
3. 配置速度低于预期阈值时的通知

### 多站点基础设施

监控办公室位置之间的连接：

1. 在每个站点部署代理
2. 配置位置之间的 iperf3 测试
3. 使用 Tailscale 进行安全的代理通信
4. 为站点间连接降级设置告警

### 服务器健康监控

跟踪服务器性能指标：

1. 在生产服务器上安装代理
2. 监控 CPU、内存、磁盘使用情况和温度
3. 配置资源耗尽的阈值告警
4. 跟踪带宽使用模式

## 高级配置

### 系统监控

在远程服务器上部署监控代理以获得完整的系统可见性。

#### 快速代理安装

```bash
curl -sL https://netrono.me/install-agent | bash
```

该脚本提供交互式设置：

- 网络接口选择
- API 密钥配置
- 监听地址和端口
- Systemd 服务创建
- 自动更新

#### 手动代理设置

```bash
# 基本代理
netronome agent

# 带身份验证
netronome agent --api-key your-secret-key

# 自定义配置
netronome agent --host 192.168.1.100 --port 8300 --interface eth0
```

#### 代理配置

添加到 `config.toml`：

```toml
[agent]
host = "0.0.0.0"
port = 8200
interface = ""  # 空表示所有接口
api_key = "your-secret-key"
disk_includes = ["/mnt/storage"]  # 要监控的其他挂载点
disk_excludes = ["/boot", "/tmp"] # 要排除的挂载点

[monitor]
enabled = true
```

### 数据包丢失监控

使用 MTR 集成和性能跟踪的持续网络监控。

#### 关键功能

- 灵活的调度（10 秒到 24 小时或精确的每日时间）
- 实时进度指示器
- 历史性能图表
- 具有权限回退的跨平台支持

#### 重要说明

- MTR 需要提升的权限才能完全运行
- 即使中间跳超时，总体数据包丢失也可能为 0%（正常行为）

### Tailscale 集成

原生 Tailscale 支持，无需端口暴露的安全网状网络。

#### 代理设置

```bash
# 基本 Tailscale 代理
netronome agent --tailscale --tailscale-auth-key tskey-auth-YOUR-KEY

# 使用现有的 tailscaled
netronome agent --tailscale --tailscale-method host

# 自定义主机名
netronome agent --tailscale --tailscale-hostname "webserver-prod"
```

#### 服务器配置

```toml
[tailscale]
enabled = true
method = "auto"  # auto、host 或 tsnet
auth_key = ""    # tsnet 模式需要
hostname = ""    # 可选的自定义主机名

# 发现设置
auto_discover = true
discovery_interval = "5m"
discovery_port = 8200
```

### Docker 代理集成

您可能希望在 Docker 容器内运行代理，例如，在像 Gluetun 这样的容器上监控 VPN 流量。
默认情况下，这将不起作用，因为容器无法访问主机的网络接口以获取统计信息。

要监控 VPN 网络容器上的带宽，您需要在与 VPN 容器相同的网络中运行代理和 **vnstat** 容器。

<details>
<summary>Docker 代理集成 Compose 示例</summary>

```yml
services:
  # Gluetun - VPN 客户端容器
  gluetun:
    image: qmcgaw/gluetun:latest
    container_name: gluetun
    restart: unless-stopped
    cap_add:
      - NET_ADMIN
    devices:
      - /dev/net/tun:/dev/net/tun
    volumes:
      - /path/to/gluetun:/gluetun
    environment:
      - VPN_SERVICE_PROVIDER=your_provider
      - VPN_TYPE=wireguard
      # ... 您的 VPN 配置
    networks:
      monitoring:
        aliases:
          - netronome-vpn-agent  # 允许仪表板按名称访问代理

  # vnstat - 在 VPN 隧道上收集带宽数据
  vnstat:
    image: vergoh/vnstat:latest
    container_name: vnstat
    restart: unless-stopped
    network_mode: "service:gluetun"
    depends_on:
      - gluetun
    environment:
      - TZ=
    volumes:
      - /path/to/vnstat:/var/lib/vnstat # 添加 vnstat 数据库的挂载点

  # Netronome VPN 代理 - 监控 VPN 隧道流量
  netronome-vpn-agent:
    image: ghcr.io/autobrr/netronome:latest # 您也可以将代理 bin 放在较小的镜像中
    container_name: netronome-vpn-agent
    restart: unless-stopped
    network_mode: "service:gluetun"
    depends_on:
      - gluetun
      - vnstat
    environment:
      - TZ=
      - NETRONOME__AGENT_HOST=0.0.0.0
      - NETRONOME__AGENT_PORT=8200
      - NETRONOME__AGENT_API_KEY=  # 可选：设置用于身份验证
    command:
      - agent
      - --interface
      - tun0  # VPN 隧道接口
    volumes:
      - /path/to/vnstat:/var/lib/vnstat:ro
    cap_add:
      - NET_RAW
      - NET_ADMIN

  # Netronome 仪表板 - 主要 Web 界面
  netronome:
    image: ghcr.io/autobrr/netronome:latest
    container_name: netronome
    restart: unless-stopped
    environment:
      - TZ=
      - NETRONOME__HOST=0.0.0.0
      - NETRONOME__PORT=7575
    ports:
      - "7575:7575"
    volumes:
      - /path/to/netronome:/data
    networks:
      - monitoring
    cap_add:
      - NET_RAW
      - NET_ADMIN

networks:
  monitoring:
    driver: bridge
```
</details>

通过运行以下命令确定要监控的正确 VPN 接口：

```
docker exec gluetun ip -br link
```

常见接口名称：
* `tun0` - OpenVPN 或 Gluetun 自定义提供商
* `wg0` - WireGuard

#### 限制监控的接口

默认情况下，`vnstat` 将监控所有检测到的接口（例如 eth0 和 tun0）。要仅监控 VPN 隧道：

```sh
# 从 vnstat 中删除不需要的接口
docker exec vnstat vnstat --remove -i eth0 --force

# 验证仅跟踪 tun0
docker exec vnstat vnstat
```

### GeoIP 配置

在 traceroute 结果中启用国家标志和 ASN 信息（可选）：

1. 在 [MaxMind](https://www.maxmind.com/en/geolite2/signup) 注册免费许可证
2. 下载 GeoLite2 数据库（Country 和 ASN）
3. 将路径添加到您的配置：
   ```toml
   [geoip]
   country_database_path = "/path/to/GeoLite2-Country.mmdb"
   asn_database_path = "/path/to/GeoLite2-ASN.mmdb"
   ```

Netronome 在没有 GeoIP 的情况下也能完美运行 - 这只是添加了视觉国家指示器。

### 通知

<p align="center">
  <img src=".github/assets/notifications.png" alt="通知配置">
</p>

通过 Web 界面在 **设置 > 通知** 中配置通知。

#### 支持的服务

- Discord、Telegram、Slack、Teams
- Email (SMTP)、Pushover、Pushbullet
- Gotify、Matrix、Ntfy、Webhook
- [以及通过 Shoutrrr 的 15+ 种更多服务](https://containrrr.dev/shoutrrr/)

#### 通知事件

- 速度测试完成、失败、阈值突破
- 数据包丢失状态变化（降级/恢复）
- 代理指标：CPU、内存、磁盘、带宽、温度阈值

### 调度

支持两种调度类型：

#### 基于持续时间的间隔

```
"30s", "5m", "1h", "24h"
```

添加 1-300 秒的随机抖动以防止同时执行。

#### 精确时间间隔

```
"exact:14:30"           # 每天下午 2:30
"exact:00:00,12:00"     # 每天午夜和中午
```

添加 1-60 秒的随机抖动。

## 参考

### 环境变量

所有配置选项都可以使用 `NETRONOME__` 前缀通过环境变量设置。以下是最常用的：

```bash
# 服务器设置
NETRONOME__HOST=0.0.0.0              # 监听地址
NETRONOME__PORT=7575                 # Web UI 端口
NETRONOME__BASE_URL=/                # 反向代理的基础 URL

# 数据库（默认 SQLite）
NETRONOME__DB_TYPE=sqlite            # sqlite 或 postgres
NETRONOME__DB_PATH=netronome.db      # SQLite 数据库路径

# PostgreSQL（当 DB_TYPE=postgres 时）
NETRONOME__DB_HOST=localhost
NETRONOME__DB_PORT=5432
NETRONOME__DB_USER=postgres
NETRONOME__DB_PASSWORD=secret
NETRONOME__DB_NAME=netronome
NETRONOME__DB_SSLMODE=disable

# 身份验证
NETRONOME__AUTH_WHITELIST=127.0.0.1/32,192.168.1.0/24  # IP 白名单（逗号分隔）
NETRONOME__SESSION_SECRET=           # 会话密钥（如果为空则自动生成）

# OIDC（可选）
NETRONOME__OIDC_ISSUER=https://accounts.google.com
NETRONOME__OIDC_CLIENT_ID=your-client-id
NETRONOME__OIDC_CLIENT_SECRET=your-secret
NETRONOME__OIDC_REDIRECT_URL=https://example.com/api/auth/oidc/callback
```

<details>
<summary><b>完整环境变量参考</b>（点击展开）</summary>

### 服务器配置

```bash
NETRONOME__HOST=127.0.0.1                    # 服务器监听地址
NETRONOME__PORT=7575                         # 服务器端口
NETRONOME__BASE_URL=/                        # 基础 URL 路径（用于反向代理）
NETRONOME__GIN_MODE=                         # Gin 框架模式（debug/release/test）
```

### 数据库配置

```bash
NETRONOME__DB_TYPE=sqlite                    # 数据库类型：sqlite 或 postgres
NETRONOME__DB_PATH=netronome.db              # SQLite 数据库文件路径
NETRONOME__DB_HOST=localhost                 # PostgreSQL 主机
NETRONOME__DB_PORT=5432                      # PostgreSQL 端口
NETRONOME__DB_USER=postgres                  # PostgreSQL 用户名
NETRONOME__DB_PASSWORD=                      # PostgreSQL 密码
NETRONOME__DB_NAME=netronome                 # PostgreSQL 数据库名称
NETRONOME__DB_SSLMODE=disable                # PostgreSQL SSL 模式
```

### 身份验证配置

```bash
NETRONOME__AUTH_WHITELIST=127.0.0.1/32       # IP 白名单（逗号分隔）
NETRONOME__SESSION_SECRET=                    # 会话密钥（如果为空则自动生成）
```

### OIDC 配置

```bash
NETRONOME__OIDC_ISSUER=https://accounts.google.com  # OIDC 颁发者 URL
NETRONOME__OIDC_CLIENT_ID=your-client-id             # OIDC 客户端 ID
NETRONOME__OIDC_CLIENT_SECRET=your-secret            # OIDC 客户端密钥
NETRONOME__OIDC_REDIRECT_URL=https://example.com/api/auth/oidc/callback  # OIDC 重定向 URL
```

### 代理配置

```bash
NETRONOME__AGENT_HOST=0.0.0.0               # 代理监听地址
NETRONOME__AGENT_PORT=8200                   # 代理端口
NETRONOME__AGENT_INTERFACE=                  # 代理网络接口（空表示所有）
NETRONOME__AGENT_API_KEY=                    # 代理 API 密钥
```

### 监控配置

```bash
NETRONOME__MONITOR_ENABLED=true              # 启用监控
```

### Tailscale 配置

```bash
NETRONOME__TAILSCALE_ENABLED=true            # 启用 Tailscale
NETRONOME__TAILSCALE_METHOD=auto             # Tailscale 方法：auto、host 或 tsnet
NETRONOME__TAILSCALE_AUTH_KEY=               # Tailscale 认证密钥（tsnet 模式需要）
NETRONOME__TAILSCALE_HOSTNAME=               # Tailscale 主机名（可选）
NETRONOME__TAILSCALE_AUTO_DISCOVER=true      # 自动发现 Tailscale 节点
NETRONOME__TAILSCALE_DISCOVERY_INTERVAL=5m   # 发现间隔
NETRONOME__TAILSCALE_DISCOVERY_PORT=8200     # 发现端口
```

### GeoIP 配置

```bash
NETRONOME__GEOIP_COUNTRY_DATABASE_PATH=/path/to/GeoLite2-Country.mmdb  # GeoIP 国家数据库路径
NETRONOME__GEOIP_ASN_DATABASE_PATH=/path/to/GeoLite2-ASN.mmdb           # GeoIP ASN 数据库路径
```

### 通知配置

```bash
NETRONOME__NOTIFICATION_ENABLED=true         # 启用通知
```

</details>

### CLI 命令

```bash
# 服务器命令
netronome serve              # 启动服务器
netronome generate-config    # 生成默认配置文件

# 用户管理
netronome create-user <username>     # 创建新用户
netronome change-password <username> # 更改用户密码

# 代理命令
netronome agent              # 启动代理
netronome agent --api-key <key>  # 使用 API 密钥启动代理
netronome agent --host <addr> --port <port>  # 使用自定义地址和端口启动代理
netronome agent --interface <iface>  # 使用特定接口启动代理
netronome agent --tailscale  # 使用 Tailscale 启动代理
netronome agent --tailscale-auth-key <key>  # 使用 Tailscale 认证密钥启动代理
netronome agent --tailscale-hostname <hostname>  # 使用自定义 Tailscale 主机名启动代理
```

## 常见问题与故障排除

### 常见问题

**Q: Netronome 需要端口转发吗？**
A: 不需要。Netronome 使用 Tailscale 进行安全的网状网络，无需端口暴露。

**Q: 我可以运行多个代理吗？**
A: 可以。您可以在多个服务器上部署代理，并从一个仪表板监控所有代理。

**Q: 如何监控 VPN 流量？**
A: 您需要在与 VPN 容器相同的网络中运行代理和 vnstat 容器。请参阅 [Docker 代理集成](#docker-代理集成) 部分。

**Q: MTR 需要什么权限？**
A: MTR 需要提升的权限（root 或 sudo）才能完全运行。如果没有足够的权限，它将回退到 traceroute。

**Q: 如何更改数据库类型？**
A: 使用环境变量 `NETRONOME__DB_TYPE` 设置数据库类型为 `sqlite` 或 `postgres`。

**Q: 如何配置反向代理？**
A: 请参阅 [使用基础 URL 的反向代理](#使用基础-url-的反向代理) 部分。

### 故障排除

**问题：无法启动服务器**
- 检查端口 7575 是否已被占用
- 检查配置文件语法是否正确
- 检查数据库文件权限

**问题：代理无法连接到服务器**
- 检查 API 密钥是否正确
- 检查网络连接和防火墙设置
- 检查服务器地址和端口是否正确

**问题：速度测试失败**
- 检查 iperf3 或 librespeed-cli 是否已安装
- 检查网络连接
- 检查防火墙设置

**问题：数据包丢失监控不工作**
- 检查 mtr 或 traceroute 是否已安装
- 检查是否有足够的权限运行 mtr
- 检查目标主机是否可达

## 从源代码构建

### 前置要求

- Go 1.24 或更高版本
- Node.js 18 或更高版本（用于构建前端）
- npm 或 yarn

### 构建步骤

1. **克隆仓库**

   ```bash
   git clone https://github.com/autobrr/netronome.git
   cd netronome
   ```

2. **构建前端**

   ```bash
   cd web
   npm install
   npm run build
   cd ..
   ```

3. **构建后端**

   ```bash
   go build -o netronome ./cmd/netronome
   ```

4. **运行**

   ```bash
   ./netronome generate-config
   ./netronome serve
   ```

### 开发模式

1. **启动后端服务器**

   ```bash
   go run ./cmd/netronome serve
   ```

2. **启动前端开发服务器**

   ```bash
   cd web
   npm run dev
   ```

## 贡献

我们欢迎贡献！请遵循以下步骤：

1. Fork 本仓库
2. 创建您的特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交您的更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启一个 Pull Request

### 代码规范

- 遵循 Go 代码规范
- 为新功能添加测试
- 更新相关文档
- 确保所有测试通过

## 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 致谢

- [Gin](https://github.com/gin-gonic/gin) - Go Web 框架
- [React](https://reactjs.org/) - 前端框架
- [Shoutrrr](https://containrrr.dev/shoutrrr/) - 通知服务
- [MaxMind](https://www.maxmind.com/) - GeoIP 数据库

## 联系方式

- [GitHub Issues](https://github.com/autobrr/netronome/issues) - 报告问题和功能请求
- [Discord](https://discord.gg/WehFCZxq5B) - 加入我们的社区

---

<p align="center">
  <strong>Netronome</strong> - 监控、分析、告警
</p>
