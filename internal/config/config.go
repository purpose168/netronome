// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later

// Netronome 配置模块
// 包名: config
// 功能: 提供应用程序配置的加载、保存和管理功能
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later

// 该模块是 Netronome 应用程序的核心配置管理系统，提供了以下主要功能：
// 1. 配置文件的加载和解析（支持 TOML 格式）
// 2. 环境变量覆盖配置
// 3. 配置默认值设置
// 4. 配置文件的生成和保存
// 5. 配置路径的管理和查找
// 6. 配置验证和迁移
//
// 主要特性：
// - 分层配置结构，支持复杂的配置项
// - 环境变量支持，便于容器化部署
// - 配置文件自动生成和确保存在
// - 向后兼容的配置迁移
// - Tailscale 配置的特殊处理
//
// 依赖：
// - github.com/BurntSushi/toml: TOML 配置文件解析
// - github.com/rs/zerolog/log: 日志记录
// - github.com/autobrr/netronome/internal/utils: 工具函数

package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/netronome/internal/utils"
)

const (
	// EnvPrefix 是所有环境变量的前缀，用于避免与其他应用程序的环境变量冲突
	// 例如：NETRONOME__DB_TYPE=postgres
	EnvPrefix = "NETRONOME__"

	// AppName 用于配置目录命名，默认配置目录为 ~/.config/netronome/
	AppName = "netronome"
)

// DatabaseType 表示数据库类型的枚举
// 该类型用于指定应用程序使用的数据库引擎
//
// 支持的数据库类型：
// - SQLite: 轻量级的文件数据库，适合单机部署和小型应用
// - Postgres: 功能强大的关系型数据库，适合多用户和大规模部署
//
// 使用场景：
// - 开发环境和小型部署通常使用 SQLite
// - 生产环境和大规模部署通常使用 Postgres
// - 可以通过配置文件或环境变量切换数据库类型
type DatabaseType string

const (
	// SQLite 表示使用 SQLite 数据库
	// SQLite 是一个轻量级的文件数据库，不需要单独的数据库服务器
	// 适合单机部署、开发环境和小型应用
	SQLite DatabaseType = "sqlite"

	// Postgres 表示使用 PostgreSQL 数据库
	// PostgreSQL 是一个功能强大的开源关系型数据库，支持高级特性
	// 适合多用户环境、生产部署和大规模应用
	Postgres DatabaseType = "postgres"
)

// Config 表示应用程序的主配置结构体
// 该结构体包含了应用程序的所有配置项，采用分层结构组织
// 每个配置项对应一个专门的子配置结构体，便于管理和维护
//
// 配置结构设计遵循以下原则：
// - 分层组织：按功能领域划分子配置结构体
// - TOML 标签：支持从 TOML 配置文件加载
// - 环境变量支持：可以通过环境变量覆盖配置值
// - 默认值：所有配置项都有合理的默认值
//
// 主要配置领域：
// - Database: 数据库连接配置
// - Server: Web 服务器配置
// - Logging: 日志系统配置
// - Auth: 认证系统配置
// - OIDC: OpenID Connect 认证配置
// - SpeedTest: 速度测试配置
// - GeoIP: GeoIP 数据库配置
// - Pagination: 分页系统配置
// - Session: 用户会话配置
// - PacketLoss: 数据包丢失监控配置
// - Agent: 代理服务器配置
// - Monitor: 监控系统配置
// - Tailscale: Tailscale 集成配置
//
// 使用方式：
// 1. 通过 New() 创建默认配置
// 2. 通过 Load() 从配置文件和环境变量加载配置
// 3. 通过 WriteToml() 保存配置到文件
// 4. 可以直接访问和修改配置字段
//
// 配置加载优先级（从高到低）：
// 1. 环境变量
// 2. 配置文件
// 3. 默认值
type Config struct {
	Database   DatabaseConfig   `toml:"database"`   // 数据库配置
	Server     ServerConfig     `toml:"server"`     // Web 服务器配置
	Logging    LoggingConfig    `toml:"logging"`    // 日志系统配置
	Auth       AuthConfig       `toml:"auth"`       // 认证系统配置
	OIDC       OIDCConfig       `toml:"oidc"`       // OpenID Connect 配置
	SpeedTest  SpeedTestConfig  `toml:"speedtest"`  // 速度测试配置
	GeoIP      GeoIPConfig      `toml:"geoip"`      // GeoIP 数据库配置
	Pagination PaginationConfig `toml:"pagination"` // 分页系统配置
	Session    SessionConfig    `toml:"session"`    // 用户会话配置
	PacketLoss PacketLossConfig `toml:"packetloss"` // 数据包丢失监控配置
	Agent      AgentConfig      `toml:"agent"`      // 代理服务器配置
	Monitor    MonitorConfig    `toml:"monitor"`    // 监控系统配置
	Tailscale  TailscaleConfig  `toml:"tailscale"`  // Tailscale 集成配置
}

// DatabaseConfig 表示数据库连接配置
// 支持 SQLite 和 PostgreSQL 两种数据库类型
//
// 配置字段说明：
// - Type: 数据库类型（sqlite 或 postgres）
// - Host: PostgreSQL 主机地址（仅适用于 postgres 类型）
// - Port: PostgreSQL 端口（仅适用于 postgres 类型）
// - User: PostgreSQL 用户名（仅适用于 postgres 类型）
// - Password: PostgreSQL 密码（仅适用于 postgres 类型）
// - DBName: PostgreSQL 数据库名（仅适用于 postgres 类型）
// - SSLMode: PostgreSQL SSL 模式（仅适用于 postgres 类型）
// - Path: SQLite 数据库文件路径（仅适用于 sqlite 类型）
//
// 使用示例：
// 1. SQLite 配置：
// [database]
// type = "sqlite"
// path = "netronome.db"
//
// 2. PostgreSQL 配置：
// [database]
// type = "postgres"
// host = "localhost"
// port = 5432
// user = "postgres"
// password = "secret"
// dbname = "netronome"
// sslmode = "disable"
type DatabaseConfig struct {
	Type     DatabaseType `toml:"type" env:"DB_TYPE"`         // 数据库类型（sqlite 或 postgres）
	Host     string       `toml:"host" env:"DB_HOST"`         // PostgreSQL 主机地址
	Port     int          `toml:"port" env:"DB_PORT"`         // PostgreSQL 端口
	User     string       `toml:"user" env:"DB_USER"`         // PostgreSQL 用户名
	Password string       `toml:"password" env:"DB_PASSWORD"` // PostgreSQL 密码
	DBName   string       `toml:"dbname" env:"DB_NAME"`       // PostgreSQL 数据库名
	SSLMode  string       `toml:"sslmode" env:"DB_SSLMODE"`   // PostgreSQL SSL 模式
	Path     string       `toml:"path" env:"DB_PATH"`         // SQLite 数据库文件路径
}

// ServerConfig 表示 Web 服务器配置
// 用于配置应用程序的 HTTP 服务器参数
//
// 配置字段说明：
// - Host: 服务器监听的主机地址
// - Port: 服务器监听的端口号
// - BaseURL: 应用程序的基础 URL 路径（用于反向代理场景）
// - GinMode: Gin 框架的运行模式（debug, release, test）
//
// 默认配置：
// - Host: "127.0.0.1"（本地监听）
// - Port: 7575
// - BaseURL: "/"（根路径）
// - GinMode: 自动设置（开发环境为 debug，生产环境为 release）
//
// 容器化部署注意事项：
// 在容器环境中，Host 会自动设置为 "0.0.0.0" 以允许外部访问
type ServerConfig struct {
	Host    string `toml:"host" env:"HOST"`         // 服务器监听的主机地址
	Port    int    `toml:"port" env:"PORT"`         // 服务器监听的端口号
	BaseURL string `toml:"base_url" env:"BASE_URL"` // 应用程序的基础 URL 路径
	GinMode string `toml:"gin_mode" env:"GIN_MODE"` // Gin 框架的运行模式
}

// LoggingConfig 表示日志系统配置
// 用于配置应用程序的日志级别
//
// 配置字段说明：
// - Level: 日志级别（trace, debug, info, warn, error, fatal, panic）
//
// 默认值："info"
//
// 日志级别说明：
// - trace: 最详细的日志，包含所有调试信息
// - debug: 调试信息，用于开发和调试
// - info: 普通信息，记录应用程序的主要操作
// - warn: 警告信息，记录潜在问题
// - error: 错误信息，记录错误情况
// - fatal: 致命错误，记录导致应用程序终止的错误
// - panic: 严重错误，记录导致 panic 的错误
type LoggingConfig struct {
	Level string `toml:"level" env:"LOG_LEVEL"` // 日志级别
}

// AuthConfig 表示认证系统配置
// 用于配置应用程序的认证规则和白名单
//
// 配置字段说明：
// - Whitelist: 无需认证即可访问的 IP 地址或 CIDR 范围列表
//
// 使用示例：
// [auth]
// whitelist = ["127.0.0.1/32", "192.168.1.0/24"]
//
// 注意事项：
// - 白名单中的 IP 地址或网络范围可以绕过认证
// - 请谨慎使用，仅将可信任的网络范围添加到白名单
// - 白名单配置使用 CIDR 表示法
// - 可以通过环境变量配置，多个值用逗号分隔：AUTH_WHITELIST="127.0.0.1/32,192.168.1.0/24"
type AuthConfig struct {
	Whitelist []string `toml:"whitelist" env:"AUTH_WHITELIST"` // 无需认证的 IP 地址或 CIDR 范围列表
}

// OIDCConfig 表示 OpenID Connect 认证配置
// 用于配置应用程序与 OIDC 提供商的集成
//
// 配置字段说明：
// - Issuer: OIDC 提供商的发行者 URL
// - ClientID: 应用程序的客户端 ID（由 OIDC 提供商分配）
// - ClientSecret: 应用程序的客户端密钥（由 OIDC 提供商分配）
// - RedirectURL: 认证成功后的重定向 URL
//
// 使用示例：
// [oidc]
// issuer = "https://accounts.google.com"
// client_id = "your-client-id"
// client_secret = "your-client-secret"
// redirect_url = "http://localhost:7575/auth/callback"
//
// 注意事项：
// - OIDC 配置是可选的，不配置则使用内置认证
// - 请确保重定向 URL 与 OIDC 提供商配置的一致
// - 客户端密钥应该安全存储，避免泄露
type OIDCConfig struct {
	Issuer       string `toml:"issuer" env:"OIDC_ISSUER"`               // OIDC 提供商的发行者 URL
	ClientID     string `toml:"client_id" env:"OIDC_CLIENT_ID"`         // 应用程序的客户端 ID
	ClientSecret string `toml:"client_secret" env:"OIDC_CLIENT_SECRET"` // 应用程序的客户端密钥
	RedirectURL  string `toml:"redirect_url" env:"OIDC_REDIRECT_URL"`   // 认证成功后的重定向 URL
}

// SpeedTestConfig 表示速度测试配置
// 用于配置应用程序的网络速度测试参数
//
// 配置字段说明：
// - IPerf: IPerf 速度测试配置
// - Librespeed: Librespeed 速度测试配置
// - Timeout: 速度测试的全局超时时间（秒）
//
// 速度测试支持两种方式：
// 1. IPerf: 通过 IPerf 服务器进行速度测试
// 2. Librespeed: 通过 Librespeed 服务器进行速度测试
//
// 默认配置：
// - Timeout: 30 秒
type SpeedTestConfig struct {
	IPerf      IperfConfig      `toml:"iperf"`                           // IPerf 速度测试配置
	Librespeed LibrespeedConfig `toml:"librespeed"`                      // Librespeed 速度测试配置
	Timeout    int              `toml:"timeout" env:"SPEEDTEST_TIMEOUT"` // 速度测试的全局超时时间（秒）
}

// IperfConfig 表示 IPerf 速度测试配置
// 用于配置 IPerf 速度测试的参数
//
// 配置字段说明：
// - TestDuration: 测试持续时间（秒）
// - ParallelConns: 并行连接数
// - Timeout: IPerf 测试的超时时间（秒）
// - Ping: Ping 测试配置
//
// 默认配置：
// - TestDuration: 10 秒
// - ParallelConns: 4 个并行连接
// - Timeout: 60 秒
type IperfConfig struct {
	TestDuration  int        `toml:"test_duration" env:"IPERF_TEST_DURATION"`   // 测试持续时间（秒）
	ParallelConns int        `toml:"parallel_conns" env:"IPERF_PARALLEL_CONNS"` // 并行连接数
	Timeout       int        `toml:"timeout" env:"IPERF_TIMEOUT"`               // IPerf 测试的超时时间（秒）
	Ping          PingConfig `toml:"ping"`                                      // Ping 测试配置
}

// LibrespeedConfig 表示 Librespeed 速度测试配置
// 用于配置 Librespeed 速度测试的参数
//
// 配置字段说明：
// - ServersPath: Librespeed 服务器列表文件路径（内部使用，不通过配置文件设置）
// - Timeout: Librespeed 测试的超时时间（秒）
//
// 默认配置：
// - Timeout: 60 秒
// - ServersPath: 自动设置为配置文件目录下的 "librespeed-servers.json"
//
// 注意事项：
// - ServersPath 字段在 TOML 配置中被忽略（使用 - 标签）
// - 服务器列表文件会自动生成或加载
type LibrespeedConfig struct {
	ServersPath string `toml:"-"`                                // Librespeed 服务器列表文件路径（内部使用）
	Timeout     int    `toml:"timeout" env:"LIBRESPEED_TIMEOUT"` // Librespeed 测试的超时时间（秒）
}

// PingConfig 表示 Ping 测试配置
// 用于配置 Ping 测试的参数
//
// 配置字段说明：
// - Count: Ping 测试的数据包数量
// - Interval: 数据包之间的间隔时间（毫秒）
// - Timeout: Ping 测试的超时时间（秒）
//
// 默认配置：
// - Count: 5 个数据包
// - Interval: 1000 毫秒（1 秒）
// - Timeout: 10 秒
type PingConfig struct {
	Count    int `toml:"count" env:"IPERF_PING_COUNT"`       // Ping 测试的数据包数量
	Interval int `toml:"interval" env:"IPERF_PING_INTERVAL"` // 数据包之间的间隔时间（毫秒）
	Timeout  int `toml:"timeout" env:"IPERF_PING_TIMEOUT"`   // Ping 测试的超时时间（秒）
}

// PaginationConfig 表示分页系统配置
// 用于配置应用程序的分页参数
//
// 配置字段说明：
// - DefaultPage: 默认页码
// - DefaultPageSize: 默认每页显示的记录数
// - MaxPageSize: 允许的最大每页记录数
// - DefaultTimeRange: 默认时间范围（用于查询历史数据）
// - DefaultLimit: 默认查询限制（用于非分页查询）
//
// 默认配置：
// - DefaultPage: 1
// - DefaultPageSize: 20
// - MaxPageSize: 100
// - DefaultTimeRange: "1w"（1 周）
// - DefaultLimit: 20
//
// 时间范围格式：
// - 数字 + 单位，如：1h（1 小时）、1d（1 天）、1w（1 周）、1m（1 月）
type PaginationConfig struct {
	DefaultPage      int    `toml:"default_page" env:"DEFAULT_PAGE"`             // 默认页码
	DefaultPageSize  int    `toml:"default_page_size" env:"DEFAULT_PAGE_SIZE"`   // 默认每页显示的记录数
	MaxPageSize      int    `toml:"max_page_size" env:"MAX_PAGE_SIZE"`           // 允许的最大每页记录数
	DefaultTimeRange string `toml:"default_time_range" env:"DEFAULT_TIME_RANGE"` // 默认时间范围
	DefaultLimit     int    `toml:"default_limit" env:"DEFAULT_LIMIT"`           // 默认查询限制
}

// SessionConfig 表示用户会话配置
// 用于配置应用程序的用户会话参数
//
// 配置字段说明：
// - Secret: 会话密钥，用于加密会话数据
//
// 默认配置：
// - Secret: 自动生成的 32 字节安全令牌
//
// 注意事项：
// - 会话密钥应该保密，避免泄露
// - 更换会话密钥会导致所有现有会话失效
// - 建议定期更换会话密钥以提高安全性
type SessionConfig struct {
	Secret string `toml:"session_secret" env:"SESSION_SECRET"` // 会话密钥，用于加密会话数据
}

// GeoIPConfig 表示 GeoIP 数据库配置
// 用于配置 GeoIP 数据库的路径，用于国家标记和 ASN 信息查询
//
// 配置字段说明：
// - CountryDatabasePath: GeoIP 国家数据库文件路径
// - ASNDatabasePath: GeoIP ASN 数据库文件路径
//
// 默认配置：
// - CountryDatabasePath: ""（未配置，GeoIP 功能禁用）
// - ASNDatabasePath: ""（未配置，GeoIP 功能禁用）
//
// 使用示例：
// [geoip]
// country_database_path = "/path/to/GeoLite2-Country.mmdb"
// asn_database_path = "/path/to/GeoLite2-ASN.mmdb"
//
// 注意事项：
// - GeoIP 配置是可选的，不配置则禁用 GeoIP 功能
// - 需要从 MaxMind 网站下载 GeoIP 数据库文件
// - 数据库文件需要定期更新以保持准确性
type GeoIPConfig struct {
	CountryDatabasePath string `toml:"country_database_path" env:"GEOIP_COUNTRY_DATABASE_PATH"` // GeoIP 国家数据库文件路径
	ASNDatabasePath     string `toml:"asn_database_path" env:"GEOIP_ASN_DATABASE_PATH"`         // GeoIP ASN 数据库文件路径
}

// PacketLossConfig 表示数据包丢失监控配置
// 用于配置应用程序的数据包丢失监控功能
//
// 配置字段说明：
// - Enabled: 是否启用数据包丢失监控
// - DefaultInterval: 默认测试间隔（秒）
// - DefaultPacketCount: 默认每个测试的数据包数量
// - MaxConcurrentMonitors: 允许的最大并发监控数
// - PrivilegedMode: 是否使用特权模式（使用 ICMP 协议需要 root 权限）
// - RestoreMonitorsOnStartup: 是否在启动时恢复之前的监控配置
//
// 默认配置：
// - Enabled: true（启用）
// - DefaultInterval: 3600 秒（1 小时）
// - DefaultPacketCount: 10 个数据包
// - MaxConcurrentMonitors: 10 个并发监控
// - PrivilegedMode: true（使用特权模式）
// - RestoreMonitorsOnStartup: false（不恢复监控）
//
// 注意事项：
// - PrivilegedMode 需要 root 或 sudo 权限才能使用 ICMP 协议
// - 非特权模式使用 UDP 协议，但可能不够准确
// - 过多的并发监控会消耗系统资源，请合理设置 MaxConcurrentMonitors
type PacketLossConfig struct {
	Enabled                  bool `toml:"enabled" env:"PACKETLOSS_ENABLED"`                                         // 是否启用数据包丢失监控
	DefaultInterval          int  `toml:"default_interval" env:"PACKETLOSS_DEFAULT_INTERVAL"`                       // 默认测试间隔（秒）
	DefaultPacketCount       int  `toml:"default_packet_count" env:"PACKETLOSS_DEFAULT_PACKET_COUNT"`               // 默认每个测试的数据包数量
	MaxConcurrentMonitors    int  `toml:"max_concurrent_monitors" env:"PACKETLOSS_MAX_CONCURRENT_MONITORS"`         // 允许的最大并发监控数
	PrivilegedMode           bool `toml:"privileged_mode" env:"PACKETLOSS_PRIVILEGED_MODE"`                         // 是否使用特权模式
	RestoreMonitorsOnStartup bool `toml:"restore_monitors_on_startup" env:"PACKETLOSS_RESTORE_MONITORS_ON_STARTUP"` // 是否在启动时恢复监控
}

// AgentConfig 表示代理服务器配置
// 用于配置 Netronome 代理的参数
//
// 配置字段说明：
// - Host: 代理服务器监听的主机地址
// - Port: 代理服务器监听的端口号
// - Interface: 要监控的网络接口名称（空表示所有接口）
// - APIKey: 代理服务器的 API 密钥，用于认证请求
// - DiskIncludes: 要包含的磁盘路径列表
// - DiskExcludes: 要排除的磁盘路径列表
//
// 默认配置：
// - Host: "0.0.0.0"（监听所有接口）
// - Port: 8200
// - Interface: ""（监控所有接口）
// - APIKey: ""（默认不设置，建议生产环境设置）
// - DiskIncludes: []（包含所有磁盘）
// - DiskExcludes: []（不排除任何磁盘）
//
// 使用示例：
// [agent]
// host = "0.0.0.0"
// port = 8200
// interface = "eth0"
// api_key = "your-secure-api-key"
// disk_includes = ["/", "/home"]
// disk_excludes = ["/tmp", "/var/log"]
//
// 注意事项：
// - APIKey 应该设置为强密码，避免未授权访问
// - DiskIncludes 和 DiskExcludes 支持通配符
// - Interface 可以通过 ifconfig 或 ip link 命令查看
type AgentConfig struct {
	Host         string   `toml:"host" env:"AGENT_HOST"`                                    // 代理服务器监听的主机地址
	Port         int      `toml:"port" env:"AGENT_PORT"`                                    // 代理服务器监听的端口号
	Interface    string   `toml:"interface" env:"AGENT_INTERFACE"`                          // 要监控的网络接口名称
	APIKey       string   `toml:"api_key" env:"AGENT_API_KEY"`                              // 代理服务器的 API 密钥
	DiskIncludes []string `toml:"disk_includes" env:"AGENT_DISK_INCLUDES" envSeparator:","` // 要包含的磁盘路径列表
	DiskExcludes []string `toml:"disk_excludes" env:"AGENT_DISK_EXCLUDES" envSeparator:","` // 要排除的磁盘路径列表
}

// MonitorConfig 表示监控系统配置
// 用于配置应用程序的监控参数
//
// 配置字段说明：
// - Enabled: 是否启用监控功能
// - ReconnectInterval: 重连间隔时间（用于与代理服务器的连接）
//
// 默认配置：
// - Enabled: true（启用监控）
// - ReconnectInterval: "30s"（30 秒）
//
// 时间格式：
// - 数字 + 单位，如：30s（30 秒）、1m（1 分钟）、5m（5 分钟）
//
// 注意事项：
// - 监控功能用于定期检查代理服务器的状态
// - 重连间隔应该根据网络稳定性和需求合理设置
type MonitorConfig struct {
	Enabled           bool   `toml:"enabled" env:"MONITOR_ENABLED"`                       // 是否启用监控功能
	ReconnectInterval string `toml:"reconnect_interval" env:"MONITOR_RECONNECT_INTERVAL"` // 重连间隔时间
}

// TailscaleConfig 表示 Tailscale 集成配置
// 用于配置应用程序与 Tailscale 的集成，支持安全的 agent-to-server 通信
//
// Tailscale 是一种零配置 VPN，允许你在任何地方安全地访问设备和应用程序
// 该配置支持两种集成模式：
// 1. host 模式：使用已安装的 Tailscale 主机代理
// 2. tsnet 模式：使用嵌入式 Tailscale 网络（无需预先安装 Tailscale）
// 3. auto 模式：自动检测并选择合适的模式
//
// 配置字段说明：
// - Core settings:
//   - Enabled: 是否启用 Tailscale 集成
//   - Method: 集成模式（auto, host, tsnet）
//   - AuthKey: Tailscale 认证密钥（tsnet 模式必需）
//
// - TSNet-specific settings (仅 tsnet 模式使用):
//   - Hostname: 自定义主机名（可选）
//   - Ephemeral: 是否在关闭时移除节点
//   - StateDir: 状态目录路径
//   - ControlURL: Headscale 控制服务器 URL（用于自托管 Tailscale）
//
// - Agent settings:
//   - AgentPort: Agent 监听的端口
//
// - Server discovery settings:
//   - AutoDiscover: 是否自动发现 Tailscale 上的 agent
//   - DiscoveryInterval: 发现间隔时间
//   - DiscoveryPort: 发现探测端口
//   - DiscoveryPrefix: 发现前缀（用于限制发现范围）
//
// - Deprecated fields (用于向后兼容):
//   - PreferHost: 优先使用 host 模式
//   - Agent: 旧版 Agent 配置
//   - Monitor: 旧版 Monitor 配置
//
// 默认配置：
// - Enabled: false
// - Method: "auto"
// - AuthKey: ""
// - Hostname: ""
// - Ephemeral: false
// - StateDir: "~/.config/netronome/tsnet"
// - ControlURL: ""
// - AgentPort: 8200
// - AutoDiscover: true
// - DiscoveryInterval: "5m"
// - DiscoveryPort: 8200
// - DiscoveryPrefix: ""
//
// 使用示例：
// 1. Host 模式：
// [tailscale]
// enabled = true
// method = "host"
//
// 2. TSNet 模式：
// [tailscale]
// enabled = true
// method = "tsnet"
// auth_key = "tskey-auth-xxxxxxxxxxxx"
// hostname = "netronome-agent"
// ephemeral = true
// state_dir = "~/.config/netronome/tsnet"
//
// 3. 自动模式：
// [tailscale]
// enabled = true
// method = "auto"
// auth_key = "tskey-auth-xxxxxxxxxxxx"  # 提供密钥则使用 tsnet，否则使用 host

type TailscaleConfig struct {
	// Core settings
	Enabled bool   `toml:"enabled" env:"TAILSCALE_ENABLED"`   // 是否启用 Tailscale 集成
	Method  string `toml:"method" env:"TAILSCALE_METHOD"`     // 集成模式（"auto", "host", 或 "tsnet"）
	AuthKey string `toml:"auth_key" env:"TAILSCALE_AUTH_KEY"` // Tailscale 认证密钥（tsnet 模式必需）

	// TSNet-specific settings
	Hostname   string `toml:"hostname" env:"TAILSCALE_HOSTNAME"`       // 自定义主机名（可选）
	Ephemeral  bool   `toml:"ephemeral" env:"TAILSCALE_EPHEMERAL"`     // 是否在关闭时移除节点
	StateDir   string `toml:"state_dir" env:"TAILSCALE_STATE_DIR"`     // 状态目录
	ControlURL string `toml:"control_url" env:"TAILSCALE_CONTROL_URL"` // Headscale 控制服务器 URL

	// Agent settings
	AgentPort int `toml:"agent_port" env:"TAILSCALE_AGENT_PORT"` // Agent 监听的端口

	// Server discovery settings
	AutoDiscover      bool   `toml:"auto_discover" env:"TAILSCALE_AUTO_DISCOVER"`           // 是否自动发现 Tailscale 上的 agent
	DiscoveryInterval string `toml:"discovery_interval" env:"TAILSCALE_DISCOVERY_INTERVAL"` // 发现间隔时间
	DiscoveryPort     int    `toml:"discovery_port" env:"TAILSCALE_DISCOVERY_PORT"`         // 发现探测端口
	DiscoveryPrefix   string `toml:"discovery_prefix" env:"TAILSCALE_DISCOVERY_PREFIX"`     // 发现前缀

	// Deprecated fields (for backward compatibility)
	PreferHost bool                   `toml:"prefer_host" env:"TAILSCALE_PREFER_HOST"` // 优先使用 host 模式（已废弃）
	Agent      TailscaleAgentConfig   `toml:"agent"`                                   // 旧版 Agent 配置（已废弃）
	Monitor    TailscaleMonitorConfig `toml:"monitor"`                                 // 旧版 Monitor 配置（已废弃）
}

// TailscaleAgentConfig 表示旧版 Tailscale Agent 配置
// Deprecated - 保留用于向后兼容，新代码应使用 TailscaleConfig 中的对应字段
//
// 配置字段说明：
// - Enabled: 是否启用 Agent
// - AcceptRoutes: 是否接受 Tailscale 路由
// - Port: Agent 监听的端口
//
// 注意：这些字段已被整合到 TailscaleConfig 中，将在未来版本中移除
type TailscaleAgentConfig struct {
	Enabled      bool `toml:"enabled" env:"TAILSCALE_AGENT_ENABLED"`             // 是否启用 Agent
	AcceptRoutes bool `toml:"accept_routes" env:"TAILSCALE_AGENT_ACCEPT_ROUTES"` // 是否接受 Tailscale 路由
	Port         int  `toml:"port" env:"TAILSCALE_AGENT_PORT"`                   // Agent 监听的端口
}

// TailscaleMonitorConfig 表示旧版 Tailscale Monitor 配置
// Deprecated - 保留用于向后兼容，新代码应使用 TailscaleConfig 中的对应字段
//
// 配置字段说明：
// - AutoDiscover: 是否自动发现 Agent
// - DiscoveryInterval: 发现间隔时间
// - DiscoveryPrefix: 发现前缀
// - DiscoveryPort: 发现探测端口
//
// 注意：这些字段已被整合到 TailscaleConfig 中，将在未来版本中移除
type TailscaleMonitorConfig struct {
	AutoDiscover      bool   `toml:"auto_discover" env:"TAILSCALE_MONITOR_AUTO_DISCOVER"`           // 是否自动发现 Agent
	DiscoveryInterval string `toml:"discovery_interval" env:"TAILSCALE_MONITOR_DISCOVERY_INTERVAL"` // 发现间隔时间
	DiscoveryPrefix   string `toml:"discovery_prefix" env:"TAILSCALE_MONITOR_DISCOVERY_PREFIX"`     // 发现前缀
	DiscoveryPort     int    `toml:"discovery_port" env:"TAILSCALE_MONITOR_DISCOVERY_PORT"`         // 发现探测端口
}

// isRunningInContainer 检测应用程序是否在容器环境中运行
// 该函数通过多种方式检测容器环境，以确保检测的准确性
//
// 检测方法：
// 1. 检查 /.dockerenv 文件（Docker 容器的标志）
// 2. 检查 /dev/.lxc-boot-id 文件（LXC 容器的标志）
// 3. 检查进程 ID 是否为 1（容器中的主进程通常为 PID 1）
// 4. 检查用户名是否为 ContainerAdministrator 或 ContainerUser（Windows 容器）
// 5. 检查 /proc/1/cgroup 文件是否包含 /docker 或 /lxc（Linux 容器）
//
// 返回值：
// - bool: 如果在容器环境中运行，则返回 true；否则返回 false
//
// 使用场景：
// - 根据运行环境自动调整配置（如服务器监听地址）
// - 容器化部署时的特殊处理
// - 日志和监控的环境标记
//
// 注意事项：
// - 该函数使用了多种检测方法以提高准确性
// - 某些容器环境可能不满足所有检测条件，但至少满足其中一种
// - 在非容器环境中，所有检测条件都不满足，返回 false
func isRunningInContainer() bool {
	// 检查 Docker 容器的标志文件
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// 检查 LXC 容器的标志文件
	if _, err := os.Stat("/dev/.lxc-boot-id"); err == nil {
		return true
	}

	// 检查进程 ID 是否为 1（容器中的主进程通常为 PID 1）
	if os.Getpid() == 1 {
		return true
	}

	// 检查 Windows 容器的用户名
	if user := os.Getenv("USERNAME"); user == "ContainerAdministrator" || user == "ContainerUser" {
		return true
	}

	// 检查 Linux 容器的 cgroup 文件
	if pd, _ := os.Open("/proc/1/cgroup"); pd != nil {
		defer pd.Close()
		b := make([]byte, 4096)
		pd.Read(b)
		if strings.Contains(string(b), "/docker") || strings.Contains(string(b), "/lxc") {
			return true
		}
	}

	return false
}

// New 创建一个带有默认值的 Config 实例
// 该函数初始化应用程序的所有默认配置
// 所有配置项都有合理的默认值，可以通过配置文件或环境变量覆盖
//
// 返回值：
// - *Config: 带有默认值的配置实例指针
//
// 使用场景：
// - 应用程序启动时初始化默认配置
// - 生成新的配置文件时使用
// - 重置配置到默认状态时使用
//
// 默认配置概述：
// - 数据库：使用 SQLite，文件名为 netronome.db
// - 服务器：监听 127.0.0.1:7575
// - 日志：级别为 info
// - 速度测试：使用 IPerf 和 Librespeed，默认测试持续时间为 10 秒
// - 分页：默认每页 20 条记录，最大 100 条
// - 数据包丢失监控：启用，默认每小时测试一次
// - Agent：监听 0.0.0.0:8200
// - Tailscale：默认禁用
//
// 注意事项：
// - 所有配置项都可以通过配置文件或环境变量覆盖
// - 会话密钥默认为空，会在生成配置文件时自动生成
// - 容器环境会自动调整某些配置（如服务器监听地址）
func New() *Config {
	return &Config{
		// 数据库配置默认值
		Database: DatabaseConfig{
			Type:    SQLite,         // 默认使用 SQLite 数据库
			Host:    "localhost",    // PostgreSQL 默认主机
			Port:    5432,           // PostgreSQL 默认端口
			User:    "postgres",     // PostgreSQL 默认用户名
			DBName:  "netronome",    // PostgreSQL 默认数据库名
			SSLMode: "disable",      // PostgreSQL 默认 SSL 模式
			Path:    "netronome.db", // SQLite 默认数据库文件路径
		},
		// 服务器配置默认值
		Server: ServerConfig{
			Host:    "127.0.0.1", // 默认只监听本地地址
			Port:    7575,        // 默认端口 7575
			BaseURL: "/",         // 默认基础 URL
		},
		// 日志配置默认值
		Logging: LoggingConfig{
			Level: "info", // 默认日志级别为 info
		},
		// 速度测试配置默认值
		SpeedTest: SpeedTestConfig{
			IPerf: IperfConfig{
				TestDuration:  10, // 默认测试持续时间 10 秒
				ParallelConns: 4,  // 默认并行连接数 4
				Timeout:       60, // 默认超时时间 60 秒
				Ping: PingConfig{
					Count:    5,    // 默认 Ping 数据包数量 5
					Interval: 1000, // 默认 Ping 间隔 1000 毫秒
					Timeout:  10,   // 默认 Ping 超时 10 秒
				},
			},
			Librespeed: LibrespeedConfig{
				ServersPath: "librespeed-servers.json", // 默认服务器列表文件
				Timeout:     60,                        // 默认超时时间 60 秒
			},
			Timeout: 30, // 速度测试全局超时 30 秒
		},
		// 分页配置默认值
		Pagination: PaginationConfig{
			DefaultPage:      1,    // 默认页码 1
			DefaultPageSize:  20,   // 默认每页记录数 20
			MaxPageSize:      100,  // 最大每页记录数 100
			DefaultTimeRange: "1w", // 默认时间范围 1 周
			DefaultLimit:     20,   // 默认查询限制 20
		},
		// 会话配置默认值
		Session: SessionConfig{
			Secret: "", // 会话密钥默认为空，生成配置文件时自动生成
		},
		// GeoIP 配置默认值
		GeoIP: GeoIPConfig{
			CountryDatabasePath: "", // 默认不配置国家数据库
			ASNDatabasePath:     "", // 默认不配置 ASN 数据库
		},
		// 数据包丢失监控配置默认值
		PacketLoss: PacketLossConfig{
			Enabled:                  true,  // 默认启用
			DefaultInterval:          3600,  // 默认测试间隔 3600 秒（1 小时）
			DefaultPacketCount:       10,    // 默认每个测试的数据包数量 10
			MaxConcurrentMonitors:    10,    // 默认最大并发监控数 10
			PrivilegedMode:           true,  // 默认使用特权模式
			RestoreMonitorsOnStartup: false, // 默认不恢复监控
		},
		// Agent 配置默认值
		Agent: AgentConfig{
			Host:         "0.0.0.0",  // 默认监听所有接口
			Port:         8200,       // 默认端口 8200
			Interface:    "",         // 默认监控所有网络接口
			DiskIncludes: []string{}, // 默认包含所有磁盘
			DiskExcludes: []string{}, // 默认不排除任何磁盘
		},
		// 监控配置默认值
		Monitor: MonitorConfig{
			Enabled:           true,  // 默认启用监控
			ReconnectInterval: "30s", // 默认重连间隔 30 秒
		},
		// Tailscale 配置默认值
		Tailscale: TailscaleConfig{
			Enabled:           false,                       // 默认禁用
			Method:            "auto",                      // 默认自动模式
			AuthKey:           "",                          // 默认无认证密钥
			Hostname:          "",                          // 默认无自定义主机名
			Ephemeral:         false,                       // 默认不是临时节点
			StateDir:          "~/.config/netronome/tsnet", // 默认状态目录
			ControlURL:        "",                          // 默认无自定义控制服务器
			AgentPort:         8200,                        // 默认 Agent 端口 8200
			AutoDiscover:      true,                        // 默认启用自动发现
			DiscoveryInterval: "5m",                        // 默认发现间隔 5 分钟
			DiscoveryPort:     8200,                        // 默认发现端口 8200
			DiscoveryPrefix:   "",                          // 默认无发现前缀
			// 已废弃字段 - 用于迁移期间的兼容性
			Agent: TailscaleAgentConfig{
				Enabled:      false, // 默认禁用 Agent
				AcceptRoutes: true,  // 默认接受路由
				Port:         8200,  // 默认端口 8200
			},
			Monitor: TailscaleMonitorConfig{
				AutoDiscover:      true, // 默认启用自动发现
				DiscoveryInterval: "5m", // 默认发现间隔 5 分钟
				DiscoveryPrefix:   "",   // 默认无发现前缀
				DiscoveryPort:     8200, // 默认发现端口 8200
			},
		},
	}
}

// Load 从 TOML 文件和环境变量加载配置
// 该函数是配置加载的核心入口，负责合并默认配置、配置文件和环境变量
//
// 参数：
// - configPath: 配置文件路径（可选），如果提供，则只尝试从该路径加载
//
// 返回值：
// - *Config: 加载完成的配置实例指针
// - error: 如果加载过程中发生错误，则返回错误信息；否则返回 nil
//
// 加载流程：
// 1. 创建默认配置实例
// 2. 如果提供了配置文件路径，则尝试从该路径加载
// 3. 如果未提供配置文件路径，则尝试从默认路径列表中查找并加载
// 4. 处理相对路径（数据库文件路径和 Librespeed 服务器列表路径）
// 5. 使用环境变量覆盖配置值
//
// 配置优先级（从高到低）：
// 1. 环境变量
// 2. 配置文件
// 3. 默认值
//
// 相对路径处理：
// - 数据库文件路径如果是相对路径，则相对于配置文件所在目录
// - Librespeed 服务器列表路径固定为配置文件所在目录下的 "librespeed-servers.json"
//
// 错误处理：
// - 如果提供了具体配置文件路径但加载失败，返回错误
// - 如果使用默认路径列表但未找到配置文件，使用默认值继续运行
// - 环境变量加载失败会导致整个加载过程失败
func Load(configPath string) (*Config, error) {
	cfg := New()

	// 如果提供了具体配置文件路径，只尝试从该路径加载
	if configPath != "" {
		// 解码 TOML 配置文件到配置实例
		if _, err := toml.DecodeFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("加载配置文件失败 %s: %w", configPath, err)
		}
		// 确保 Librespeed 超时时间有合理默认值
		if cfg.SpeedTest.Librespeed.Timeout == 0 {
			cfg.SpeedTest.Librespeed.Timeout = 60
		}
		log.Info().
			Str("path", configPath).
			Msg("已加载配置文件")

		// 如果数据库路径是相对路径，将其转换为相对于配置文件的路径
		if !filepath.IsAbs(cfg.Database.Path) {
			cfg.Database.Path = filepath.Join(filepath.Dir(configPath), cfg.Database.Path)
		}
		// 设置 Librespeed 服务器列表路径为配置文件所在目录下的文件
		cfg.SpeedTest.Librespeed.ServersPath = filepath.Join(filepath.Dir(configPath), "librespeed-servers.json")
	} else {
		// 未提供配置文件路径，尝试从默认路径列表中查找
		found := false
		paths := DefaultConfigPaths()
		for _, path := range paths {
			log.Debug().
				Str("checking_path", path).
				Msg("正在检查配置文件")

			// 检查配置文件是否存在
			if _, err := os.Stat(path); err == nil {
				log.Debug().
					Str("found_at", path).
					Msg("找到配置文件")

				// 解码配置文件
				if _, err := toml.DecodeFile(path, cfg); err == nil {
					// 确保 Librespeed 超时时间有合理默认值
					if cfg.SpeedTest.Librespeed.Timeout == 0 {
						cfg.SpeedTest.Librespeed.Timeout = 60
					}
					log.Info().
						Str("path", path).
						Msg("已加载配置文件")
					found = true

					// 处理相对数据库路径
					if !filepath.IsAbs(cfg.Database.Path) {
						cfg.Database.Path = filepath.Join(filepath.Dir(path), cfg.Database.Path)
					}
					// 设置 Librespeed 服务器列表路径
					cfg.SpeedTest.Librespeed.ServersPath = filepath.Join(filepath.Dir(path), "librespeed-servers.json")
					break
				}
			}
		}
		// 未找到配置文件，使用默认值运行
		if !found {
			log.Info().
				Msg("未找到配置文件，使用默认值运行。使用 'netronome generate-config' 命令创建配置文件")
		}
	}

	// 使用环境变量覆盖配置值
	if err := cfg.loadFromEnv(); err != nil {
		return nil, fmt.Errorf("从环境变量加载配置失败: %w", err)
	}

	return cfg, nil
}

// loadFromEnv 从环境变量加载配置值
// 该函数是环境变量加载的入口点，会调用各个子配置的环境变量加载函数
//
// 返回值：
// - error: 总是返回 nil，因为各个子函数不返回错误
func (c *Config) loadFromEnv() error {
	c.loadDatabaseFromEnv()
	c.loadServerFromEnv()
	c.loadLoggingFromEnv()
	c.loadAuthFromEnv()
	c.loadOIDCFromEnv()
	c.loadSpeedTestFromEnv()
	c.loadPaginationFromEnv()
	c.loadSessionFromEnv()
	c.loadGeoIPFromEnv()
	c.loadPacketLossFromEnv()
	c.loadAgentFromEnv()
	c.loadMonitorFromEnv()
	c.loadTailscaleFromEnv()
	return nil
}

// loadDatabaseFromEnv 从环境变量加载数据库配置
// 支持的环境变量：
// - NETRONOME__DB_TYPE: 数据库类型（sqlite 或 postgres）
// - NETRONOME__DB_HOST: PostgreSQL 主机地址
// - NETRONOME__DB_PORT: PostgreSQL 端口
// - NETRONOME__DB_USER: PostgreSQL 用户名
// - NETRONOME__DB_PASSWORD: PostgreSQL 密码
// - NETRONOME__DB_NAME: PostgreSQL 数据库名
// - NETRONOME__DB_SSLMODE: PostgreSQL SSL 模式
// - NETRONOME__DB_PATH: SQLite 数据库文件路径
func (c *Config) loadDatabaseFromEnv() {
	if v := getEnv("DB_TYPE"); v != "" {
		c.Database.Type = DatabaseType(v)
	}
	if v := getEnv("DB_HOST"); v != "" {
		c.Database.Host = v
	}
	if v := getEnv("DB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Database.Port = port
		}
	}
	if v := getEnv("DB_USER"); v != "" {
		c.Database.User = v
	}
	if v := getEnv("DB_PASSWORD"); v != "" {
		c.Database.Password = v
	}
	if v := getEnv("DB_NAME"); v != "" {
		c.Database.DBName = v
	}
	if v := getEnv("DB_SSLMODE"); v != "" {
		c.Database.SSLMode = v
	}
	if v := getEnv("DB_PATH"); v != "" {
		c.Database.Path = v
	}
}

// loadServerFromEnv 从环境变量加载服务器配置
// 支持的环境变量：
// - NETRONOME__HOST: 服务器监听的主机地址
// - NETRONOME__PORT: 服务器监听的端口号
// - NETRONOME__BASE_URL: 应用程序的基础 URL 路径
// - NETRONOME__GIN_MODE: Gin 框架的运行模式（debug, release, test）
func (c *Config) loadServerFromEnv() {
	if v := getEnv("HOST"); v != "" {
		c.Server.Host = v
	}
	if v := getEnv("PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.Port = port
		}
	}
	if v := getEnv("BASE_URL"); v != "" {
		c.Server.BaseURL = strings.Trim(v, `"'`)
	}
	if v := getEnv("GIN_MODE"); v != "" {
		c.Server.GinMode = v
	}
}

// loadLoggingFromEnv 从环境变量加载日志配置
// 支持的环境变量：
// - NETRONOME__LOG_LEVEL: 日志级别（trace, debug, info, warn, error, fatal, panic）
func (c *Config) loadLoggingFromEnv() {
	if v := getEnv("LOG_LEVEL"); v != "" {
		c.Logging.Level = strings.ToLower(v)
	}
}

// loadAuthFromEnv 从环境变量加载认证配置
// 支持的环境变量：
// - NETRONOME__AUTH_WHITELIST: 无需认证即可访问的 IP 地址或 CIDR 范围列表，使用逗号分隔
func (c *Config) loadAuthFromEnv() {
	if v := getEnv("AUTH_WHITELIST"); v != "" {
		c.Auth.Whitelist = strings.Split(v, ",")
	}
}

// loadOIDCFromEnv 从环境变量加载 OpenID Connect 配置
// 支持的环境变量：
// - NETRONOME__OIDC_ISSUER: OIDC 提供商的发行者 URL
// - NETRONOME__OIDC_CLIENT_ID: 应用程序的客户端 ID
// - NETRONOME__OIDC_CLIENT_SECRET: 应用程序的客户端密钥
// - NETRONOME__OIDC_REDIRECT_URL: 认证成功后的重定向 URL
func (c *Config) loadOIDCFromEnv() {
	if v := getEnv("OIDC_ISSUER"); v != "" {
		c.OIDC.Issuer = v
	}
	if v := getEnv("OIDC_CLIENT_ID"); v != "" {
		c.OIDC.ClientID = v
	}
	if v := getEnv("OIDC_CLIENT_SECRET"); v != "" {
		c.OIDC.ClientSecret = v
	}
	if v := getEnv("OIDC_REDIRECT_URL"); v != "" {
		c.OIDC.RedirectURL = v
	}
}

// loadSpeedTestFromEnv 从环境变量加载速度测试配置
// 支持的环境变量：
// - NETRONOME__SPEEDTEST_TIMEOUT: 速度测试的全局超时时间（秒）
// - NETRONOME__IPERF_TEST_DURATION: IPerf 测试持续时间（秒）
// - NETRONOME__IPERF_PARALLEL_CONNS: IPerf 并行连接数
// - NETRONOME__IPERF_TIMEOUT: IPerf 测试的超时时间（秒）
// - NETRONOME__IPERF_PING_COUNT: Ping 测试的数据包数量
// - NETRONOME__IPERF_PING_INTERVAL: Ping 数据包之间的间隔时间（毫秒）
// - NETRONOME__IPERF_PING_TIMEOUT: Ping 测试的超时时间（秒）
// - NETRONOME__LIBRESPEED_TIMEOUT: Librespeed 测试的超时时间（秒）
func (c *Config) loadSpeedTestFromEnv() {
	if v := getEnv("SPEEDTEST_TIMEOUT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			c.SpeedTest.Timeout = val
		}
	}
	if v := getEnv("IPERF_TEST_DURATION"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			c.SpeedTest.IPerf.TestDuration = val
		}
	}
	if v := getEnv("IPERF_PARALLEL_CONNS"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			c.SpeedTest.IPerf.ParallelConns = val
		}
	}
	if v := getEnv("IPERF_TIMEOUT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			c.SpeedTest.IPerf.Timeout = val
		}
	}
	if v := getEnv("IPERF_PING_COUNT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			c.SpeedTest.IPerf.Ping.Count = val
		}
	}
	if v := getEnv("IPERF_PING_INTERVAL"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			c.SpeedTest.IPerf.Ping.Interval = val
		}
	}
	if v := getEnv("IPERF_PING_TIMEOUT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			c.SpeedTest.IPerf.Ping.Timeout = val
		}
	}
	if v := getEnv("LIBRESPEED_TIMEOUT"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			c.SpeedTest.Librespeed.Timeout = val
		}
	}
}

// loadPaginationFromEnv 从环境变量加载分页配置
// 支持的环境变量：
// - NETRONOME__DEFAULT_PAGE: 默认页码
// - NETRONOME__DEFAULT_PAGE_SIZE: 默认每页显示的记录数
// - NETRONOME__MAX_PAGE_SIZE: 允许的最大每页记录数
// - NETRONOME__DEFAULT_TIME_RANGE: 默认时间范围（用于查询历史数据）
// - NETRONOME__DEFAULT_LIMIT: 默认查询限制（用于非分页查询）
func (c *Config) loadPaginationFromEnv() {
	if v := getEnv("DEFAULT_PAGE"); v != "" {
		if page, err := strconv.Atoi(v); err == nil {
			c.Pagination.DefaultPage = page
		}
	}
	if v := getEnv("DEFAULT_PAGE_SIZE"); v != "" {
		if size, err := strconv.Atoi(v); err == nil {
			c.Pagination.DefaultPageSize = size
		}
	}
	if v := getEnv("MAX_PAGE_SIZE"); v != "" {
		if size, err := strconv.Atoi(v); err == nil {
			c.Pagination.MaxPageSize = size
		}
	}
	if v := getEnv("DEFAULT_TIME_RANGE"); v != "" {
		c.Pagination.DefaultTimeRange = v
	}
	if v := getEnv("DEFAULT_LIMIT"); v != "" {
		if limit, err := strconv.Atoi(v); err == nil {
			c.Pagination.DefaultLimit = limit
		}
	}
}

// loadSessionFromEnv 从环境变量加载会话配置
// 支持的环境变量：
// - NETRONOME__SESSION_SECRET: 会话密钥，用于加密会话数据
func (c *Config) loadSessionFromEnv() {
	if v := getEnv("SESSION_SECRET"); v != "" {
		c.Session.Secret = v
	}
}

// loadGeoIPFromEnv 从环境变量加载 GeoIP 配置
// 支持的环境变量：
// - NETRONOME__GEOIP_COUNTRY_DATABASE_PATH: GeoIP 国家数据库文件路径
// - NETRONOME__GEOIP_ASN_DATABASE_PATH: GeoIP ASN 数据库文件路径
func (c *Config) loadGeoIPFromEnv() {
	if v := getEnv("GEOIP_COUNTRY_DATABASE_PATH"); v != "" {
		c.GeoIP.CountryDatabasePath = v
	}
	if v := getEnv("GEOIP_ASN_DATABASE_PATH"); v != "" {
		c.GeoIP.ASNDatabasePath = v
	}
}

// loadPacketLossFromEnv 从环境变量加载数据包丢失监控配置
// 支持的环境变量：
// - NETRONOME__PACKETLOSS_ENABLED: 是否启用数据包丢失监控
// - NETRONOME__PACKETLOSS_DEFAULT_INTERVAL: 默认测试间隔（秒）
// - NETRONOME__PACKETLOSS_DEFAULT_PACKET_COUNT: 默认每个测试的数据包数量
// - NETRONOME__PACKETLOSS_MAX_CONCURRENT_MONITORS: 允许的最大并发监控数
// - NETRONOME__PACKETLOSS_PRIVILEGED_MODE: 是否使用特权模式（使用 ICMP 协议需要 root 权限）
// - NETRONOME__PACKETLOSS_RESTORE_MONITORS_ON_STARTUP: 是否在启动时恢复之前的监控配置
func (c *Config) loadPacketLossFromEnv() {
	if v := getEnv("PACKETLOSS_ENABLED"); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			c.PacketLoss.Enabled = enabled
		}
	}
	if v := getEnv("PACKETLOSS_DEFAULT_INTERVAL"); v != "" {
		if interval, err := strconv.Atoi(v); err == nil {
			c.PacketLoss.DefaultInterval = interval
		}
	}
	if v := getEnv("PACKETLOSS_DEFAULT_PACKET_COUNT"); v != "" {
		if count, err := strconv.Atoi(v); err == nil {
			c.PacketLoss.DefaultPacketCount = count
		}
	}
	if v := getEnv("PACKETLOSS_MAX_CONCURRENT_MONITORS"); v != "" {
		if max, err := strconv.Atoi(v); err == nil {
			c.PacketLoss.MaxConcurrentMonitors = max
		}
	}
	if v := getEnv("PACKETLOSS_PRIVILEGED_MODE"); v != "" {
		if privileged, err := strconv.ParseBool(v); err == nil {
			c.PacketLoss.PrivilegedMode = privileged
		}
	}
	if v := getEnv("PACKETLOSS_RESTORE_MONITORS_ON_STARTUP"); v != "" {
		if restore, err := strconv.ParseBool(v); err == nil {
			c.PacketLoss.RestoreMonitorsOnStartup = restore
		}
	}
}

// loadAgentFromEnv 从环境变量加载代理服务器配置
// 支持的环境变量：
// - NETRONOME__AGENT_HOST: 代理服务器监听的主机地址
// - NETRONOME__AGENT_PORT: 代理服务器监听的端口号
// - NETRONOME__AGENT_INTERFACE: 要监控的网络接口名称
// - NETRONOME__AGENT_API_KEY: 代理服务器的 API 密钥，用于认证请求
// - NETRONOME__AGENT_DISK_INCLUDES: 要包含的磁盘路径列表，使用逗号分隔
// - NETRONOME__AGENT_DISK_EXCLUDES: 要排除的磁盘路径列表，使用逗号分隔
func (c *Config) loadAgentFromEnv() {
	if v := getEnv("AGENT_HOST"); v != "" {
		c.Agent.Host = v
	}
	if v := getEnv("AGENT_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Agent.Port = port
		}
	}
	if v := getEnv("AGENT_INTERFACE"); v != "" {
		c.Agent.Interface = v
	}
	if v := getEnv("AGENT_API_KEY"); v != "" {
		c.Agent.APIKey = v
	}
	if v := getEnv("AGENT_DISK_INCLUDES"); v != "" {
		c.Agent.DiskIncludes = strings.Split(v, ",")
		for i := range c.Agent.DiskIncludes {
			c.Agent.DiskIncludes[i] = strings.TrimSpace(c.Agent.DiskIncludes[i])
		}
	}
	if v := getEnv("AGENT_DISK_EXCLUDES"); v != "" {
		c.Agent.DiskExcludes = strings.Split(v, ",")
		for i := range c.Agent.DiskExcludes {
			c.Agent.DiskExcludes[i] = strings.TrimSpace(c.Agent.DiskExcludes[i])
		}
	}
}

// loadMonitorFromEnv 从环境变量加载监控系统配置
// 支持的环境变量：
// - NETRONOME__MONITOR_ENABLED: 是否启用监控功能
// - NETRONOME__MONITOR_RECONNECT_INTERVAL: 重连间隔时间
func (c *Config) loadMonitorFromEnv() {
	if v := getEnv("MONITOR_ENABLED"); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			c.Monitor.Enabled = enabled
		}
	}
	if v := getEnv("MONITOR_RECONNECT_INTERVAL"); v != "" {
		c.Monitor.ReconnectInterval = v
	}
}

// loadTailscaleFromEnv 从环境变量加载 Tailscale 配置
// 该函数会调用 TailscaleConfig 结构体的 loadFromEnv 方法
func (c *Config) loadTailscaleFromEnv() {
	c.Tailscale.loadFromEnv()
}

// WriteToml 将配置写入 TOML 格式的文件
// 该函数生成一个新的配置文件，包含默认值、自动生成的会话密钥和适当的注释
//
// 参数：
// - w: io.Writer 接口，用于写入配置内容（可以是文件、内存缓冲区等）
//
// 返回值：
// - error: 如果写入过程中发生错误，则返回错误信息；否则返回 nil
func (c *Config) WriteToml(w io.Writer) error {
	// 创建新的默认配置实例
	cfg := New()
	// 设置相对数据库路径，便于用户迁移
	cfg.Database.Path = "netronome.db"

	// 生成 32 字节的安全会话密钥
	secret, err := utils.GenerateSecureToken(32)
	if err != nil {
		return fmt.Errorf("生成会话密钥失败: %w", err)
	}
	cfg.Session.Secret = secret

	// 如果在容器环境中运行，自动设置服务器监听所有接口
	if isRunningInContainer() {
		cfg.Server.Host = "0.0.0.0"
	}

	// 写入配置文件头和标题
	if _, err := fmt.Fprintln(w, "# Netronome 配置文件"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}

	// 数据库配置段
	if _, err := fmt.Fprintln(w, "[database]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "type = \"%s\"\n", cfg.Database.Type); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "path = \"%s\"\n", cfg.Database.Path); err != nil {
		return err
	}
	// PostgreSQL 选项（默认注释）
	if _, err := fmt.Fprintln(w, "# PostgreSQL 选项（如果使用 PostgreSQL，请取消注释并修改）"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#host = \"%s\"\n", cfg.Database.Host); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#port = %d\n", cfg.Database.Port); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#user = \"%s\"\n", cfg.Database.User); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#password = \"%s\"\n", cfg.Database.Password); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#dbname = \"%s\"\n", cfg.Database.DBName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#sslmode = \"%s\"\n\n", cfg.Database.SSLMode); err != nil {
		return err
	}

	// 服务器配置段
	if _, err := fmt.Fprintln(w, "[server]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "host = \"%s\"\n", cfg.Server.Host); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "port = %d\n", cfg.Server.Port); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#base_url = \"%s\"\n", cfg.Server.BaseURL); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}

	// 日志配置段
	if _, err := fmt.Fprintln(w, "[logging]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "level = \"%s\"  # trace, debug, info, warn, error, fatal, panic\n", cfg.Logging.Level); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}

	// 认证配置段
	if _, err := fmt.Fprintln(w, "[auth]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# 白名单特定网络以绕过认证，使用 CIDR 表示法。"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# 示例: whitelist = [\"127.0.0.1/32\"]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "whitelist = []"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}

	// OIDC 配置段（默认注释）
	if _, err := fmt.Fprintln(w, "#[oidc]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#issuer = \"%s\"\n", cfg.OIDC.Issuer); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#client_id = \"%s\"\n", cfg.OIDC.ClientID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#client_secret = \"%s\"\n", cfg.OIDC.ClientSecret); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#redirect_url = \"%s\"\n", cfg.OIDC.RedirectURL); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}

	// 速度测试配置段
	if _, err := fmt.Fprintln(w, "[speedtest]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "timeout = %d\n", cfg.SpeedTest.Timeout); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}

	// IPerf 速度测试配置段
	if _, err := fmt.Fprintln(w, "[speedtest.iperf]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "test_duration = %d\n", cfg.SpeedTest.IPerf.TestDuration); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "parallel_conns = %d\n", cfg.SpeedTest.IPerf.ParallelConns); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "timeout = %d\n", cfg.SpeedTest.IPerf.Timeout); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}

	// Librespeed 速度测试配置段
	if _, err := fmt.Fprintln(w, "[speedtest.librespeed]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "timeout = %d\n", cfg.SpeedTest.Librespeed.Timeout); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}

	// IPerf Ping 测试配置段
	if _, err := fmt.Fprintln(w, "[speedtest.iperf.ping]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "count = %d\n", cfg.SpeedTest.IPerf.Ping.Count); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "interval = %d\n", cfg.SpeedTest.IPerf.Ping.Interval); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "timeout = %d\n", cfg.SpeedTest.IPerf.Ping.Timeout); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}

	// 会话配置段
	if _, err := fmt.Fprintln(w, "[session]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "session_secret = \"%s\"\n", cfg.Session.Secret); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}

	// GeoIP 配置段（默认注释）
	if _, err := fmt.Fprintln(w, "# GeoIP 配置，用于 traceroute 中的国家标志和 ASN 信息"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# 取消注释并配置数据库路径以启用。请查看 README 获取设置说明。"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "#[geoip]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#country_database_path = \"/path/to/GeoLite2-Country.mmdb\"\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "#asn_database_path = \"/path/to/GeoLite2-ASN.mmdb\"\n"); err != nil {
		return err
	}

	// 数据包丢失监控配置段
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "[packetloss]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "enabled = %v\n", cfg.PacketLoss.Enabled); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "default_interval = %d # 测试间隔（秒）\n", cfg.PacketLoss.DefaultInterval); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "default_packet_count = %d # 每次测试的数据包数量\n", cfg.PacketLoss.DefaultPacketCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "max_concurrent_monitors = %d\n", cfg.PacketLoss.MaxConcurrentMonitors); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "privileged_mode = %v # 使用特权 ICMP 模式以获得更好的 MTR 支持（需要 root/sudo）\n", cfg.PacketLoss.PrivilegedMode); err != nil {
		return err
	}

	// 监控配置段
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "[monitor]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "enabled = %v\n", cfg.Monitor.Enabled); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "reconnect_interval = \"%s\"\n", cfg.Monitor.ReconnectInterval); err != nil {
		return err
	}

	// Tailscale 配置段
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# Tailscale 集成，用于安全的 agent-to-server 通信"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "[tailscale]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "enabled = %v\n", cfg.Tailscale.Enabled); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "method = \"%s\" # \"auto\"（默认）, \"host\", 或 \"tsnet\"\n", cfg.Tailscale.Method); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# TSNet 设置（当 method=\"tsnet\" 或自动检测时使用）"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# auth_key = \"\" # tsnet 模式必需"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# hostname = \"\" # 自定义主机名（可选）"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "# ephemeral = %v # 关闭时移除节点\n", cfg.Tailscale.Ephemeral); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "# state_dir = \"%s\" # tsnet 状态目录\n", cfg.Tailscale.StateDir); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# control_url = \"\" # 用于 Headscale（可选）"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# Agent 设置"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "agent_port = %d # Agent 监听端口（0 = 使用 agent 自身端口）\n", cfg.Tailscale.AgentPort); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# 服务器发现设置"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "auto_discover = %v # 自动发现 Tailscale 上的 agent\n", cfg.Tailscale.AutoDiscover); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "discovery_interval = \"%s\" # 检查新 agent 的频率\n", cfg.Tailscale.DiscoveryInterval); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "discovery_port = %d # 探测 agent 的端口\n", cfg.Tailscale.DiscoveryPort); err != nil {
		return err
	}

	return nil
}

// GetDefaultConfigPath 获取默认配置文件路径
// 该函数会尝试从默认配置目录中查找配置文件，如果找到则返回该路径，否则返回当前目录下的 "config.toml"
//
// 返回值：
// - string: 配置文件的默认路径
//
// 查找顺序：
// 1. 用户配置目录下的 netronome/config.toml
// 2. 当前目录下的 config.toml
//
// 使用场景：
// - 当用户未指定配置文件路径时，使用该函数获取默认路径
// - 用于配置文件的自动查找
func GetDefaultConfigPath() string {
	if configDir, err := os.UserConfigDir(); err == nil {
		configPath := filepath.Join(configDir, AppName, "config.toml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	return "config.toml"
}

// DefaultConfigPaths 获取默认配置文件路径列表
// 该函数返回一个配置文件路径列表，按照优先级从高到低排序
//
// 返回值：
// - []string: 配置文件路径列表，按优先级从高到低排序
//
// 路径顺序（优先级从高到低）：
// 1. 用户主目录下的 .config/netronome/config.toml
// 2. 用户配置目录下的 netronome/config.toml
// 3. 当前目录下的 config.toml
//
// 使用场景：
// - Load 函数用于查找配置文件
// - 用于配置文件的自动搜索和加载
// - 确保在不同操作系统和环境下都能找到正确的配置文件
func DefaultConfigPaths() []string {
	var paths []string

	if homeDir, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(homeDir, ".config", AppName, "config.toml"))
	}

	if configDir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(configDir, AppName, "config.toml"))
	}

	// 最后尝试当前目录
	paths = append(paths, "config.toml")

	return paths
}

// EnsureConfig 确保配置文件存在于指定路径或默认位置，如果不存在则生成一个
// 该函数是配置文件管理的核心函数，用于确保应用程序始终有可用的配置文件
//
// 参数：
// - configPath: 配置文件路径（可选），如果为空，则检查默认位置
//
// 返回值：
// - string: 实际使用的配置文件路径
// - error: 如果创建配置文件失败，则返回错误信息；否则返回 nil
//
// 处理流程：
//  1. 如果提供了具体配置文件路径：
//     a. 如果文件不存在，创建父目录和配置文件
//     b. 生成默认配置并写入文件
//     c. 返回该路径
//  2. 如果未提供配置文件路径：
//     a. 检查默认路径列表中是否存在配置文件
//     b. 如果找到，返回该路径
//     c. 如果未找到，尝试创建配置文件：
//     i. 优先尝试用户主目录下的 .config/netronome 目录
//     ii. 然后尝试平台特定的用户配置目录
//     iii. 最后回退到当前目录
//     iv. 生成默认配置并写入文件
//     v. 返回创建的配置文件路径
//
// 使用场景：
// - 应用程序启动时确保配置文件存在
// - 用于 "generate-config" 命令
// - 确保配置文件的可用性
func EnsureConfig(configPath string) (string, error) {
	if configPath != "" {
		// 如果提供了具体配置文件路径，检查是否存在，不存在则创建
		if _, err := os.Stat(configPath); err != nil {
			// 创建父目录结构
			if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
				return "", fmt.Errorf("创建配置目录失败: %w", err)
			}

			// 创建默认配置并写入文件
			cfg := New()
			f, err := os.Create(configPath)
			if err != nil {
				return "", fmt.Errorf("创建配置文件失败: %w", err)
			}
			defer f.Close()

			if err := cfg.WriteToml(f); err != nil {
				return "", fmt.Errorf("写入配置文件失败: %w", err)
			}

			log.Info().Str("path", configPath).Msg("已生成默认配置文件")
		}
		return configPath, nil
	}

	// 检查默认路径列表
	paths := DefaultConfigPaths()
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// 未找到配置文件，尝试创建
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configDir := filepath.Join(homeDir, ".config")
		netronomeDir := filepath.Join(configDir, AppName)
		if err := os.MkdirAll(netronomeDir, 0755); err == nil {
			configPath = filepath.Join(netronomeDir, "config.toml")
		} else {
			// 回退到平台特定的用户配置目录
			if configDir, err := os.UserConfigDir(); err == nil {
				netronomeDir := filepath.Join(configDir, AppName)
				if err := os.MkdirAll(netronomeDir, 0755); err == nil {
					configPath = filepath.Join(netronomeDir, "config.toml")
				} else {
					log.Warn().
						Err(err).
						Msg("无法创建配置目录，回退到工作目录")
					configPath = "config.toml"
				}
			} else {
				log.Warn().
					Err(err).
					Msg("无法确定用户配置目录，回退到工作目录")
				configPath = "config.toml"
			}
		}
	}

	// 创建配置文件
	cfg := New()
	f, err := os.Create(configPath)
	if err != nil {
		return "", fmt.Errorf("创建配置文件失败: %w", err)
	}
	defer f.Close()

	// 写入默认配置
	if err := cfg.WriteToml(f); err != nil {
		return "", fmt.Errorf("写入配置文件失败: %w", err)
	}

	log.Info().Str("path", configPath).Msg("已生成默认配置文件")
	return configPath, nil
}

// getEnv 获取带有应用前缀的环境变量值
// 该函数会自动在环境变量名前添加 EnvPrefix（NETRONOME__）
//
// 参数：
// - key: 环境变量的键名（不包含前缀）
//
// 返回值：
// - string: 环境变量的值，如果不存在则返回空字符串
func getEnv(key string) string {
	return os.Getenv(EnvPrefix + key)
}

// GetEffectiveMethod 根据配置返回有效的 Tailscale 方法
// 该函数是 Tailscale 方法确定的核心函数，会根据配置和自动检测逻辑返回实际使用的方法
//
// 返回值：
// - string: 实际使用的 Tailscale 方法（"host" 或 "tsnet"），如果未启用则返回空字符串
// - error: 如果配置无效则返回错误信息，否则返回 nil
//
// 方法确定逻辑：
//  1. 如果 Tailscale 未启用，返回空字符串
//  2. 如果方法为 "auto"：
//     a. 如果提供了认证密钥，使用 "tsnet" 模式
//     b. 否则使用 "host" 模式
//  3. 如果方法为 "host"，直接返回 "host"
//  4. 如果方法为 "tsnet"：
//     a. 检查是否提供了认证密钥
//     b. 如果未提供，返回错误
//     c. 否则返回 "tsnet"
//  5. 如果方法无效，返回错误
//
// 注意事项：
// - "tsnet" 模式需要提供认证密钥
// - "host" 模式使用已安装的 Tailscale 主机代理
func (t *TailscaleConfig) GetEffectiveMethod() (string, error) {
	if !t.Enabled {
		return "", nil
	}

	switch t.Method {
	case "auto":
		// 基于认证密钥的存在自动检测
		if t.AuthKey != "" {
			return "tsnet", nil
		}
		return "host", nil
	case "host":
		return "host", nil
	case "tsnet":
		// TSNet 需要认证密钥
		if t.AuthKey == "" {
			return "", fmt.Errorf("当 method 为 'tsnet' 时，auth_key 是必需的")
		}
		return "tsnet", nil
	default:
		return "", fmt.Errorf("无效的 method: %s", t.Method)
	}
}

// Validate 检查 Tailscale 配置是否有效
// 该函数是 Tailscale 配置验证的核心函数，确保配置的有效性和一致性
//
// 返回值：
// - error: 如果配置无效则返回错误信息，否则返回 nil
//
// 验证内容：
// 1. 如果 Tailscale 未启用，直接返回 nil
// 2. 验证方法是否有效（"auto", "host", "tsnet"）
// 3. 检查 tsnet 模式是否提供了认证密钥
// 4. 验证端口是否在有效范围内（0-65535）
// 5. 如果启用了自动发现，验证发现间隔是否为有效的时间格式
//
// 注意事项：
// - 端口验证包括 Agent 端口和发现端口
// - 时间格式验证使用 Go 的 time.ParseDuration 函数，支持的格式如 "30s", "1m", "5m"
func (t *TailscaleConfig) Validate() error {
	if !t.Enabled {
		return nil
	}

	// 验证方法
	if t.Method != "auto" && t.Method != "host" && t.Method != "tsnet" {
		return fmt.Errorf("无效的 method: %s", t.Method)
	}

	// 检查 tsnet 是否需要认证密钥
	method, err := t.GetEffectiveMethod()
	if err != nil {
		return err
	}
	_ = method

	// 验证端口
	if t.AgentPort < 0 || t.AgentPort > 65535 {
		return fmt.Errorf("无效的 agent 端口: %d", t.AgentPort)
	}

	if t.DiscoveryPort < 0 || t.DiscoveryPort > 65535 {
		return fmt.Errorf("无效的发现端口: %d", t.DiscoveryPort)
	}

	// 如果启用了自动发现，验证发现间隔
	if t.AutoDiscover && t.DiscoveryInterval != "" {
		if _, err := time.ParseDuration(t.DiscoveryInterval); err != nil {
			return fmt.Errorf("无效的发现间隔: %s", t.DiscoveryInterval)
		}
	}

	return nil
}

// MigrateFromOldFormat 从旧的双段格式迁移到新的统一格式
// 该函数用于处理配置格式的向后兼容性，将旧格式的配置迁移到新格式
//
// 返回值：
// - TailscaleConfig: 迁移后的新格式配置
//
// 迁移内容：
//  1. 迁移方法设置：
//     a. 如果未设置 Method，从旧字段确定
//     b. 如果 PreferHost 为 true，使用 "host" 方法
//     c. 如果提供了 AuthKey，使用 "tsnet" 方法
//     d. 否则默认使用 "host" 方法
//  2. 迁移 Agent 设置：
//     a. 如果旧 Agent 启用且新 AgentPort 为 0，迁移端口
//  3. 迁移 Monitor 设置：
//     a. 迁移 AutoDiscover 标志
//     b. 迁移 DiscoveryInterval
//     c. 迁移 DiscoveryPort
//     d. 迁移 DiscoveryPrefix
//
// 使用场景：
// - 处理旧版本配置文件的兼容性
// - 确保配置格式的平滑过渡
func (t *TailscaleConfig) MigrateFromOldFormat() TailscaleConfig {
	result := *t // 复制当前配置

	// 如果未设置 Method，从旧字段确定
	if result.Method == "" {
		if t.PreferHost {
			result.Method = "host"
		} else if t.AuthKey != "" {
			result.Method = "tsnet"
		} else {
			result.Method = "host" // 默认使用 host
		}
	}

	// 迁移 Agent 设置
	if t.Agent.Enabled && result.AgentPort == 0 {
		result.AgentPort = t.Agent.Port
	}

	// 迁移 Monitor 设置
	if result.AutoDiscover == false && t.Monitor.AutoDiscover {
		result.AutoDiscover = t.Monitor.AutoDiscover
	}
	if result.DiscoveryInterval == "" && t.Monitor.DiscoveryInterval != "" {
		result.DiscoveryInterval = t.Monitor.DiscoveryInterval
	}
	if result.DiscoveryPort == 0 && t.Monitor.DiscoveryPort != 0 {
		result.DiscoveryPort = t.Monitor.DiscoveryPort
	}
	if result.DiscoveryPrefix == "" && t.Monitor.DiscoveryPrefix != "" {
		result.DiscoveryPrefix = t.Monitor.DiscoveryPrefix
	}

	return result
}

// IsAgentMode 检查是否启用了用于 agent 连接的 Tailscale
// 该函数用于快速检查是否应该使用 Tailscale 进行 agent 连接
//
// 返回值：
// - bool: 如果启用了 Tailscale 且方法有效，则返回 true；否则返回 false
//
// 检查逻辑：
// 1. 检查 Tailscale 是否启用
// 2. 获取有效的 Tailscale 方法
// 3. 如果方法为 "host" 或 "tsnet"，返回 true
// 4. 否则返回 false
//
// 使用场景：
// - 确定是否使用 Tailscale 进行 agent 连接
// - 条件性地启用 Tailscale agent 功能
func (t *TailscaleConfig) IsAgentMode() bool {
	if !t.Enabled {
		return false
	}

	method, err := t.GetEffectiveMethod()
	if err != nil {
		return false
	}

	return method == "host" || method == "tsnet"
}

// IsServerDiscoveryMode 检查是否启用了 Tailscale 服务器发现
// 该函数用于快速检查是否应该使用 Tailscale 进行服务器发现
//
// 返回值：
// - bool: 如果启用了 Tailscale 且自动发现功能，则返回 true；否则返回 false
//
// 检查逻辑：
// 1. 检查 Tailscale 是否启用
// 2. 检查自动发现功能是否启用
//
// 使用场景：
// - 确定是否使用 Tailscale 进行服务器发现
// - 条件性地启用 Tailscale 发现功能
func (t *TailscaleConfig) IsServerDiscoveryMode() bool {
	return t.Enabled && t.AutoDiscover
}

// loadFromEnv 从环境变量加载 Tailscale 配置
// 该函数是 Tailscale 环境变量加载的核心函数，会加载所有与 Tailscale 相关的环境变量
//
// 支持的环境变量：
// - 核心设置：
//   - NETRONOME__TAILSCALE_ENABLED: 是否启用 Tailscale
//   - NETRONOME__TAILSCALE_METHOD: Tailscale 方法（"auto", "host", 或 "tsnet"）
//   - NETRONOME__TAILSCALE_AUTH_KEY: Tailscale 认证密钥
//
// - TSNet 设置：
//   - NETRONOME__TAILSCALE_HOSTNAME: 自定义主机名
//   - NETRONOME__TAILSCALE_EPHEMERAL: 是否为临时节点
//   - NETRONOME__TAILSCALE_STATE_DIR: 状态目录
//   - NETRONOME__TAILSCALE_CONTROL_URL: Headscale 控制服务器 URL
//
// - Agent 设置：
//   - NETRONOME__TAILSCALE_AGENT_PORT: Agent 监听端口
//
// - 服务器发现设置：
//   - NETRONOME__TAILSCALE_AUTO_DISCOVER: 是否启用自动发现
//   - NETRONOME__TAILSCALE_DISCOVERY_INTERVAL: 发现间隔时间
//   - NETRONOME__TAILSCALE_DISCOVERY_PORT: 发现探测端口
//   - NETRONOME__TAILSCALE_DISCOVERY_PREFIX: 发现前缀
//
// - 已废弃字段（仍支持向后兼容）：
//   - NETRONOME__TAILSCALE_PREFER_HOST: 优先使用 host 模式
//   - NETRONOME__TAILSCALE_AGENT_ENABLED: 启用 Agent
//   - NETRONOME__TAILSCALE_AGENT_ACCEPT_ROUTES: 接受路由
//   - NETRONOME__TAILSCALE_MONITOR_AUTO_DISCOVER: 监控自动发现
//   - NETRONOME__TAILSCALE_MONITOR_DISCOVERY_INTERVAL: 监控发现间隔
//   - NETRONOME__TAILSCALE_MONITOR_DISCOVERY_PORT: 监控发现端口
//   - NETRONOME__TAILSCALE_MONITOR_DISCOVERY_PREFIX: 监控发现前缀
//
// 特殊处理：
// - 对于 AUTH_KEY，即使值为空也会处理，以允许清空
// - 对于已废弃字段，会同时更新旧字段和新字段，确保向后兼容性
func (t *TailscaleConfig) loadFromEnv() {
	// 核心设置
	if v := getEnv("TAILSCALE_ENABLED"); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			t.Enabled = enabled
		}
	}
	if v := getEnv("TAILSCALE_METHOD"); v != "" {
		t.Method = v
	}
	// 总是处理 AUTH_KEY，即使为空也允许清空
	if v, exists := os.LookupEnv(EnvPrefix + "TAILSCALE_AUTH_KEY"); exists {
		t.AuthKey = v
	}

	// TSNet 设置
	if v := getEnv("TAILSCALE_HOSTNAME"); v != "" {
		t.Hostname = v
	}
	if v := getEnv("TAILSCALE_EPHEMERAL"); v != "" {
		if ephemeral, err := strconv.ParseBool(v); err == nil {
			t.Ephemeral = ephemeral
		}
	}
	if v := getEnv("TAILSCALE_STATE_DIR"); v != "" {
		t.StateDir = v
	}
	if v := getEnv("TAILSCALE_CONTROL_URL"); v != "" {
		t.ControlURL = v
	}

	// Agent 设置
	if v := getEnv("TAILSCALE_AGENT_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			t.AgentPort = port
		}
	}

	// 服务器发现设置
	if v := getEnv("TAILSCALE_AUTO_DISCOVER"); v != "" {
		if auto, err := strconv.ParseBool(v); err == nil {
			t.AutoDiscover = auto
		}
	}
	if v := getEnv("TAILSCALE_DISCOVERY_INTERVAL"); v != "" {
		t.DiscoveryInterval = v
	}
	if v := getEnv("TAILSCALE_DISCOVERY_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			t.DiscoveryPort = port
		}
	}
	if v := getEnv("TAILSCALE_DISCOVERY_PREFIX"); v != "" {
		t.DiscoveryPrefix = v
	}

	// 已废弃字段 - 仍支持向后兼容
	if v := getEnv("TAILSCALE_PREFER_HOST"); v != "" {
		if preferHost, err := strconv.ParseBool(v); err == nil {
			t.PreferHost = preferHost
			// 如果 prefer_host 为 true 且未显式设置 method，使用 "host"
			if preferHost && getEnv("TAILSCALE_METHOD") == "" {
				t.Method = "host"
			}
		}
	}
	if v := getEnv("TAILSCALE_AGENT_ENABLED"); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			t.Agent.Enabled = enabled
		}
	}
	if v := getEnv("TAILSCALE_AGENT_ACCEPT_ROUTES"); v != "" {
		if accept, err := strconv.ParseBool(v); err == nil {
			t.Agent.AcceptRoutes = accept
		}
	}
	if v := getEnv("TAILSCALE_AGENT_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			t.Agent.Port = port
			// 如果新字段尚未设置，也设置新字段
			if t.AgentPort == 0 {
				t.AgentPort = port
			}
		}
	}
	if v := getEnv("TAILSCALE_MONITOR_AUTO_DISCOVER"); v != "" {
		if auto, err := strconv.ParseBool(v); err == nil {
			t.Monitor.AutoDiscover = auto
			// 也设置新字段
			t.AutoDiscover = auto
		}
	}
	if v := getEnv("TAILSCALE_MONITOR_DISCOVERY_INTERVAL"); v != "" {
		t.Monitor.DiscoveryInterval = v
		// 也设置新字段
		t.DiscoveryInterval = v
	}
	if v := getEnv("TAILSCALE_MONITOR_DISCOVERY_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			t.Monitor.DiscoveryPort = port
			// 也设置新字段
			t.DiscoveryPort = port
		}
	}
	if v := getEnv("TAILSCALE_MONITOR_DISCOVERY_PREFIX"); v != "" {
		t.Monitor.DiscoveryPrefix = v
		// 也设置新字段
		t.DiscoveryPrefix = v
	}
}
