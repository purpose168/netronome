# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 提供在处理此仓库代码时的指南。

## 核心命令

### 构建与开发

```bash
# 构建整个应用程序（前端 + 后端）
make build

# 开发模式，支持热重载（需要 tmux）
make dev

# 仅后端热重载（需要 air）
make watch

# 清理构建产物
make clean

# 运行构建好的应用程序
make run

# Docker 命令
make docker-build
make docker-run
```

### 配置管理

```bash
# 生成默认配置（创建 ~/.config/netronome/config.toml）
./bin/netronome generate-config

# 使用特定配置运行
./bin/netronome serve --config config.toml

# 创建用户（交互式）
./bin/netronome create-user username

# 更改密码（交互式）
./bin/netronome change-password username

# 以代理模式运行
./bin/netronome agent --config config.toml
```

### 前端开发

```bash
cd web
pnpm install     # 安装依赖
pnpm dev         # 启动开发服务器（端口 5173）
pnpm build       # 构建生产版本
pnpm lint        # 运行 ESLint 检查
pnpm tsc --noEmit # 类型检查（不生成文件）
```

### 代码格式化与质量

```bash
# 为所有代码文件添加许可证头部
./license.sh false # 不使用交互式提示添加头部

# 开发过程中后端热重载（需要 air）
make watch

# 测试命令
go test ./...           # 运行所有 Go 测试
go test ./internal/...  # 运行 internal 包的测试
go test -v ./internal/speedtest -run TestPing  # 运行特定测试，带详细输出
cd web && pnpm lint     # 前端代码检查
```

### 测试

```bash
# 运行所有 Go 测试
go test ./...

# 运行特定包的测试
go test ./internal/server/...

# 运行测试并生成覆盖率报告
go test -cover ./...

# 运行特定测试，带详细输出
go test -v -run TestIsWhitelisted ./internal/server

# 前端代码检查
cd web && pnpm lint

# 前端类型检查
cd web && pnpm tsc --noEmit
```

## 高级架构

**重要提示**：Netronome 是一个自托管的单用户应用程序。系统设计为仅允许一个用户注册，所有功能都围绕此单用户模型构建。

### 后端架构（Go）

后端采用带有依赖注入的清晰架构模式：

1. **入口点** (`cmd/netronome/main.go`)：使用 Cobra 实现的 CLI 命令

   - `serve`：运行 Web 服务器
   - `agent`：运行监控代理
   - `generate-config`：创建默认配置
   - 用户管理命令

2. **核心服务** (`internal/`)：

   - **server**：使用 Gin 框架的 HTTP 服务器，处理路由和中间件
   - **database**：数据持久层，通过接口支持 SQLite/PostgreSQL
   - **speedtest**：核心速度测试逻辑（iperf3、librespeed、speedtest.net）
   - **monitor**：系统监控和代理管理
   - **scheduler**：类似 Cron 的调度器，用于自动化测试
   - **auth**：身份验证（内置和 OIDC 支持）
   - **broadcaster**：用于实时更新的 WebSocket/SSE
   - **tailscale**：用于安全网络的 Tailscale 集成
   - **notifications**：基于 Shoutrrr 的通知系统，支持 15+ 种服务

3. **代理架构**：

   - 轻量级 HTTP 服务器，提供 SSE 端点
   - 通过 gopsutil 和 vnstat 收集系统指标
   - 可以独立运行或与 Tailscale 集成
   - 支持 Tailscale 网络的自动发现
   - 带有组件级详细信息的温度传感器监控

4. **数据库模式**：
   - 基于接口的设计，支持多种后端
   - 迁移文件位于 `internal/database/migrations/`
   - 使用 Squirrel 查询构建器处理复杂查询
   - 为 SQLite 和 PostgreSQL 提供单独的实现

### 前端架构（React + TypeScript）

前端使用现代 React 模式和 TypeScript：

1. **核心技术栈**：

   - React 19 with TypeScript
   - TanStack Query 用于数据获取和缓存
   - TanStack Router 用于路由
   - Tailwind CSS v4 用于样式
   - Motion (framer-motion) 用于动画
   - Vite 用于打包

2. **组件组织**：

   - `components/auth/`：身份验证组件
   - `components/common/`：共享 UI 组件
   - `components/speedtest/`：速度测试功能
   - `components/monitor/`：系统监控 UI
   - `components/settings/`：配置和设置 UI
   - `components/settings/notifications/`：通知管理组件
   - `components/ui/`：基础 UI 组件

3. **状态管理**：

   - 使用 useState 管理组件本地状态
   - 使用 TanStack Query 管理服务器状态
   - 使用 localStorage 存储用户偏好
   - 使用 Context API 处理身份验证

4. **API 集成** (`api/`)：
   - 类型安全的 API 客户端
   - 错误处理和重试逻辑
   - 用于实时数据的 WebSocket/SSE 连接

### 关键架构决策

1. **嵌入式前端**：前端构建后嵌入到 Go 二进制文件中，实现单文件部署

2. **实时更新**：使用 Server-Sent Events (SSE) 处理实时监控数据，使用 WebSocket 处理速度测试进度

3. **插件架构**：速度测试提供商实现通用接口，便于添加新的提供商

4. **基于代理的监控**：分布式架构，代理可以与主服务器分开部署

5. **Tailscale 集成**：可选但深度集成，支持 tsnet 和 host 模式，实现灵活部署

6. **通知系统**：使用 Shoutrrr 库实现多服务通知，带有速率限制和基于状态的告警

## 数据库迁移

项目使用自定义迁移系统，为 SQLite 和 PostgreSQL 提供单独的 SQL 文件：

```
internal/database/migrations/
├── migrations.go        # 迁移运行器
├── postgres/           # PostgreSQL 迁移文件
│   └── *.sql
└── sqlite/            # SQLite 迁移文件
    └── *.sql
```

关键点：

- 迁移文件按顺序编号（001、002 等）
- 每个迁移都有对应的 SQLite 和 PostgreSQL 文件
- 系统在 `schema_migrations` 表中跟踪已应用的迁移
- 新功能应按照现有模式添加迁移

## 通知系统

通知系统基于 [Shoutrrr](https://github.com/containrrr/shoutrrr) 构建，支持 15+ 种服务：

### 支持的服务

- Discord、Telegram、Slack、Teams
- Email (SMTP)、Pushover、Pushbullet
- Gotify、Matrix、Ntfy、OpsGenie
- Rocketchat、Zulip、Join、Mattermost

### 通知事件

1. **速度测试事件**

   - 测试完成
   - 测试失败
   - 速度低于阈值
   - 延迟高于阈值

2. **数据包丢失监控**

   - 数据包丢失降级（基于状态）
   - 数据包丢失恢复

3. **代理监控**
   - 代理在线/离线
   - CPU 使用率阈值
   - 内存使用率阈值
   - 磁盘使用率阈值
   - 带宽使用率阈值
   - 温度阈值（带传感器详细信息）

### 速率限制

- 代理指标通知：1 小时冷却时间
- 数据包丢失：基于状态（仅在状态变化时通知）
- 速度测试：每次测试完成时通知

## 开发指南

### 代码标准

- **规范提交**：在建议分支名称和提交标题时，始终使用 Conventional Commit 指南
- **许可证头部**：使用 `./license.sh false` 为新源文件添加 GPL-2.0-or-later 头部
- **提交归因**：切勿将自己添加为提交的合著者
- **前端开发**：在编写任何前端代码之前，务必阅读 `ai_docs/style-guide.md` - 该文件包含 React、TypeScript、Tailwind CSS v4、Motion 动画和组件架构的基本模式
- **导入路径**：在前端代码中始终使用 `@` 别名进行导入（例如，使用 `@/components/...` 而不是相对路径 `../components/...`）

### 提交指南

- **提交归因**：在为用户编写提交时，切勿在提交详情中添加 Co-Authored-By: Claude <noreply@anthropic.com> 和/或 🤖 Generated with [Claude Code](https://claude.ai/code)

### 测试方法

**重要提示**：由于 Netronome 是单用户应用程序，测试应反映此设计：
- 不要为并发用户创建或多用户场景编写测试
- 系统在数据库级别强制单用户注册
- 在创建第一个用户后，后续注册尝试将失败并显示 `ErrRegistrationDisabled`
- 专注于单用户工作流和功能的测试

1. **后端测试**

   - 业务逻辑的单元测试
   - 数据库操作的集成测试
   - 外部依赖的模拟接口
   - 使用 testify 进行断言
   - 避免测试多用户场景或并发用户操作

2. **前端测试**
   - 使用 React Testing Library 进行组件测试
   - 使用 TypeScript 确保类型安全
   - 使用 ESLint 进行代码检查

### 通用模式

1. **错误处理**

   - 从函数返回错误，不要使用 panic
   - 使用 zerolog 进行结构化日志记录
   - 使用 `fmt.Errorf` 为错误添加上下文

2. **API 响应**

   - 一致的 JSON 结构
   - 正确的 HTTP 状态码
   - 在 `error` 字段中包含错误信息

3. **数据库操作**
   - 对多步骤操作使用事务
   - 始终使用参数化查询
   - 适当处理空值

## 代理架构详情

代理包 (`internal/agent/`) 已重构为专注于单一职责的模块：

1. **核心代理** (`agent.go`)：约 95 行 - 仅包含主构造函数和 Start() 方法
2. **Tailscale 集成** (`tailscale.go`)：处理 tsnet 和 host 模式的启动逻辑
3. **广播** (`broadcast.go`)：向客户端的 SSE/实时数据流
4. **带宽监控** (`bandwidth.go`)：vnstat 集成，带有峰值带宽跟踪
5. **系统信息** (`system.go`)：OS 详情、网络接口和 vnstat 数据收集
6. **硬件统计** (`hardware.go`)：通过 gopsutil 获取 CPU、内存、磁盘使用率和温度
7. **磁盘工具** (`disk_utils.go`)：路径匹配和设备发现，支持 glob
8. **SMART 监控** (`smart.go`/`smart_stub.go`)：平台特定的磁盘健康监控（仅 Linux/macOS）

**部署模式**：
- 独立 HTTP 服务器（默认）
- Tailscale tsnet（创建新的 Tailscale 节点）
- Tailscale host 模式（使用现有的 tailscaled）

## 测试特定组件

```bash
# 测试速度测试实现
go test -v ./internal/speedtest/...

# 测试数据库迁移
go test -v ./internal/database/migrations/...

# 测试监控服务
go test -v ./internal/monitor/...

# 运行带有竞态检测器的测试
go test -race ./...

# 基准测试
go test -bench=. ./internal/...
```

## 重要的项目特定细节

- **前端样式指南**：`ai_docs/style-guide.md` 是所有前端开发的权威参考，包含：

  - 带有 TypeScript 接口的 React 组件模式
  - Tailwind CSS v4 实用程序类和深色模式模式
  - Motion (framer-motion) 动画配置和计时
  - 响应式设计断点和移动优先模式
  - 全面的颜色系统和语义使用指南
  - 排版标准和可访问性要求

- **代理发现**：Tailscale 发现服务自动查找并添加网络上的代理：

  - 仅发现启用了 Tailscale 的代理
  - 使用 DNSName（而非 HostName）进行正确识别
  - 通过 `discovery_interval` 配置支持自动发现

- **迁移模式**：当添加需要数据库更改的新功能时：
  1. 在 `sqlite/` 和 `postgres/` 目录中创建迁移文件
  2. 使用顺序编号（例如 025_feature_name.sql），PostgreSQL 迁移文件添加 postgres 后缀
  3. 在提交前测试两种数据库类型

## 附加说明

- **始终阅读 CLAUDE.local.md（如果存在）**，无论它是否在 .gitignore 中
- 项目使用语义化版本控制
- 所有新功能都应包含适当的测试
- 功能变更时应更新文档
- 处理前端代码时，始终先检查 `ai_docs/style-guide.md`
- **单用户设计**：请记住 Netronome 设计用于单用户自托管。不要实现多用户功能或测试并发用户场景

## 指南

- **表情符号**：请勿使用表情符号