// Copyright (c) 2024-2025, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// 数据库包提供了核心的数据库功能和服务接口
// 包含数据库连接管理、表初始化、用户操作、速度测试结果存储等功能
// 支持 SQLite 和 PostgreSQL 两种数据库类型
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"

	"github.com/autobrr/netronome/internal/config"
	"github.com/autobrr/netronome/internal/database/migrations"
	"github.com/autobrr/netronome/internal/types"
	"github.com/autobrr/netronome/pkg/migrator"
)

// 通用错误定义
var (
	ErrNotFound     = fmt.Errorf("记录未找到") // 当请求的记录不存在时返回此错误
	ErrInvalidInput = fmt.Errorf("输入无效")  // 当输入参数不符合要求时返回此错误
)

// ZerologAdapter 将 zerolog.Logger 适配为 migrator.Logger 接口
// 用于在数据库迁移过程中记录日志信息
type ZerologAdapter struct {
	logger zerolog.Logger // 底层的 zerolog 日志记录器
	quiet  bool           // 是否为安静模式，测试环境下为 true
}

// Printf 实现了 migrator.Logger 接口的 Printf 方法
// 根据 quiet 模式控制日志输出级别：
// - quiet 模式：仅记录包含 "failed" 或 "error" 的错误信息
// - 非 quiet 模式：记录所有信息
func (z *ZerologAdapter) Printf(format string, args ...interface{}) {
	if z.quiet {
		// 测试期间仅记录错误和警告
		if strings.Contains(format, "failed") || strings.Contains(format, "error") {
			z.logger.Error().Msgf(format, args...)
		}
		return
	}
	z.logger.Info().Msgf(format, args...)
}

// Service 代表核心数据库功能接口
// 提供了数据库连接管理、表初始化、用户操作、速度测试结果存储等功能
// 支持 SQLite 和 PostgreSQL 两种数据库类型

type Service interface {
	// Health 检查数据库连接健康状态
	// 返回包含数据库状态、连接池统计等信息的映射
	Health() map[string]string

	// Close 关闭数据库连接
	// 返回关闭连接时可能发生的错误
	Close() error

	// InitializeTables 初始化数据库表结构
	// ctx: 上下文，用于控制操作超时
	// 返回初始化过程中可能发生的错误
	InitializeTables(ctx context.Context) error

	// QueryRow 执行查询并返回单个结果行
	// ctx: 上下文，用于控制操作超时
	// query: SQL 查询语句
	// args: 查询参数
	// 返回包含查询结果的 *sql.Row 对象
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row

	// User operations - 用户操作

	// CreateUser 创建新用户
	// ctx: 上下文，用于控制操作超时
	// username: 用户名
	// password: 用户密码
	// 返回创建的用户对象和可能发生的错误
	CreateUser(ctx context.Context, username, password string) (*User, error)

	// GetUserByUsername 根据用户名获取用户
	// ctx: 上下文，用于控制操作超时
	// username: 用户名
	// 返回用户对象和可能发生的错误
	GetUserByUsername(ctx context.Context, username string) (*User, error)

	// ValidatePassword 验证用户密码
	// user: 用户对象
	// password: 要验证的密码
	// 返回密码是否正确的布尔值
	ValidatePassword(user *User, password string) bool

	// UpdatePassword 更新用户密码
	// ctx: 上下文，用于控制操作超时
	// username: 用户名
	// newPassword: 新密码
	// 返回更新过程中可能发生的错误
	UpdatePassword(ctx context.Context, username, newPassword string) error

	// SpeedTest operations - 速度测试操作

	// SaveSpeedTest 保存速度测试结果
	// ctx: 上下文，用于控制操作超时
	// result: 速度测试结果对象
	// 返回保存后的结果和可能发生的错误
	SaveSpeedTest(ctx context.Context, result types.SpeedTestResult) (*types.SpeedTestResult, error)

	// GetSpeedTests 获取速度测试结果列表（分页）
	// ctx: 上下文，用于控制操作超时
	// timeRange: 时间范围过滤条件
	// page: 页码
	// limit: 每页记录数
	// 返回分页的速度测试结果和可能发生的错误
	GetSpeedTests(ctx context.Context, timeRange string, page int, limit int) (*types.PaginatedSpeedTests, error)

	// Schedule operations - 计划任务操作

	// CreateSchedule 创建新的测试计划
	// ctx: 上下文，用于控制操作超时
	// schedule: 计划任务对象
	// 返回创建的计划任务和可能发生的错误
	CreateSchedule(ctx context.Context, schedule types.Schedule) (*types.Schedule, error)

	// GetSchedules 获取所有计划任务
	// ctx: 上下文，用于控制操作超时
	// 返回计划任务列表和可能发生的错误
	GetSchedules(ctx context.Context) ([]types.Schedule, error)

	// UpdateSchedule 更新计划任务
	// ctx: 上下文，用于控制操作超时
	// schedule: 更新后的计划任务对象
	// 返回更新过程中可能发生的错误
	UpdateSchedule(ctx context.Context, schedule types.Schedule) error

	// DeleteSchedule 删除计划任务
	// ctx: 上下文，用于控制操作超时
	// id: 计划任务ID
	// 返回删除过程中可能发生的错误
	DeleteSchedule(ctx context.Context, id int64) error

	// IPerf operations - IPerf 服务器操作

	// SaveIperfServer 保存 IPerf 服务器信息
	// ctx: 上下文，用于控制操作超时
	// name: 服务器名称
	// host: 服务器地址
	// port: 服务器端口
	// 返回保存后的服务器信息和可能发生的错误
	SaveIperfServer(ctx context.Context, name, host string, port int) (*types.SavedIperfServer, error)

	// GetIperfServers 获取所有保存的 IPerf 服务器
	// ctx: 上下文，用于控制操作超时
	// 返回服务器列表和可能发生的错误
	GetIperfServers(ctx context.Context) ([]types.SavedIperfServer, error)

	// DeleteIperfServer 删除 IPerf 服务器
	// ctx: 上下文，用于控制操作超时
	// id: 服务器ID
	// 返回删除过程中可能发生的错误
	DeleteIperfServer(ctx context.Context, id int) error

	// Packet Loss operations - 丢包监控操作

	// GetPacketLossMonitor 根据ID获取丢包监控器
	// monitorID: 监控器ID
	// 返回监控器对象和可能发生的错误
	GetPacketLossMonitor(monitorID int64) (*types.PacketLossMonitor, error)

	// GetEnabledPacketLossMonitors 获取所有启用的丢包监控器
	// 返回监控器列表和可能发生的错误
	GetEnabledPacketLossMonitors() ([]*types.PacketLossMonitor, error)

	// SavePacketLossResult 保存丢包测试结果
	// result: 丢包测试结果对象
	// 返回保存过程中可能发生的错误
	SavePacketLossResult(result *types.PacketLossResult) error

	// GetLatestPacketLossResult 获取最新的丢包测试结果
	// monitorID: 监控器ID
	// 返回最新的测试结果和可能发生的错误
	GetLatestPacketLossResult(monitorID int64) (*types.PacketLossResult, error)

	// CreatePacketLossMonitor 创建新的丢包监控器
	// monitor: 监控器对象
	// 返回创建的监控器和可能发生的错误
	CreatePacketLossMonitor(monitor *types.PacketLossMonitor) (*types.PacketLossMonitor, error)

	// UpdatePacketLossMonitor 更新丢包监控器
	// monitor: 更新后的监控器对象
	// 返回更新过程中可能发生的错误
	UpdatePacketLossMonitor(monitor *types.PacketLossMonitor) error

	// DeletePacketLossMonitor 删除丢包监控器
	// monitorID: 监控器ID
	// 返回删除过程中可能发生的错误
	DeletePacketLossMonitor(monitorID int64) error

	// GetPacketLossMonitors 获取所有丢包监控器
	// 返回监控器列表和可能发生的错误
	GetPacketLossMonitors() ([]*types.PacketLossMonitor, error)

	// GetPacketLossResults 获取丢包测试结果列表
	// monitorID: 监控器ID
	// limit: 结果数量限制
	// 返回测试结果列表和可能发生的错误
	GetPacketLossResults(monitorID int64, limit int) ([]*types.PacketLossResult, error)

	// UpdatePacketLossMonitorState 更新丢包监控器状态
	// monitorID: 监控器ID
	// state: 新状态
	// 返回更新过程中可能发生的错误
	UpdatePacketLossMonitorState(monitorID int64, state string) error

	// Monitor operations - 监控代理操作

	// CreateMonitorAgent 创建新的监控代理
	// ctx: 上下文，用于控制操作超时
	// agent: 代理对象
	// 返回创建的代理和可能发生的错误
	CreateMonitorAgent(ctx context.Context, agent *types.MonitorAgent) (*types.MonitorAgent, error)

	// GetMonitorAgent 根据ID获取监控代理
	// ctx: 上下文，用于控制操作超时
	// agentID: 代理ID
	// 返回代理对象和可能发生的错误
	GetMonitorAgent(ctx context.Context, agentID int64) (*types.MonitorAgent, error)

	// GetMonitorAgents 获取监控代理列表
	// ctx: 上下文，用于控制操作超时
	// enabledOnly: 是否只返回启用的代理
	// 返回代理列表和可能发生的错误
	GetMonitorAgents(ctx context.Context, enabledOnly bool) ([]*types.MonitorAgent, error)

	// UpdateMonitorAgent 更新监控代理
	// ctx: 上下文，用于控制操作超时
	// agent: 更新后的代理对象
	// 返回更新过程中可能发生的错误
	UpdateMonitorAgent(ctx context.Context, agent *types.MonitorAgent) error

	// DeleteMonitorAgent 删除监控代理
	// ctx: 上下文，用于控制操作超时
	// agentID: 代理ID
	// 返回删除过程中可能发生的错误
	DeleteMonitorAgent(ctx context.Context, agentID int64) error

	// Monitor agent data operations - 监控代理数据操作

	// UpsertMonitorSystemInfo 插入或更新监控代理系统信息
	// ctx: 上下文，用于控制操作超时
	// agentID: 代理ID
	// info: 系统信息对象
	// 返回操作过程中可能发生的错误
	UpsertMonitorSystemInfo(ctx context.Context, agentID int64, info *types.MonitorSystemInfo) error

	// GetMonitorSystemInfo 获取监控代理系统信息
	// ctx: 上下文，用于控制操作超时
	// agentID: 代理ID
	// 返回系统信息对象和可能发生的错误
	GetMonitorSystemInfo(ctx context.Context, agentID int64) (*types.MonitorSystemInfo, error)

	// UpsertMonitorInterfaces 插入或更新监控代理网络接口信息
	// ctx: 上下文，用于控制操作超时
	// agentID: 代理ID
	// interfaces: 网络接口列表
	// 返回操作过程中可能发生的错误
	UpsertMonitorInterfaces(ctx context.Context, agentID int64, interfaces []types.MonitorInterface) error

	// GetMonitorInterfaces 获取监控代理网络接口信息
	// ctx: 上下文，用于控制操作超时
	// agentID: 代理ID
	// 返回网络接口列表和可能发生的错误
	GetMonitorInterfaces(ctx context.Context, agentID int64) ([]types.MonitorInterface, error)

	// UpsertMonitorPeakStats 插入或更新监控代理峰值统计信息
	// ctx: 上下文，用于控制操作超时
	// agentID: 代理ID
	// stats: 峰值统计信息对象
	// 返回操作过程中可能发生的错误
	UpsertMonitorPeakStats(ctx context.Context, agentID int64, stats *types.MonitorPeakStats) error

	// GetMonitorPeakStats 获取监控代理峰值统计信息
	// ctx: 上下文，用于控制操作超时
	// agentID: 代理ID
	// 返回峰值统计信息对象和可能发生的错误
	GetMonitorPeakStats(ctx context.Context, agentID int64) (*types.MonitorPeakStats, error)

	// SaveMonitorResourceStats 保存监控代理资源统计信息
	// ctx: 上下文，用于控制操作超时
	// agentID: 代理ID
	// stats: 资源统计信息对象
	// 返回操作过程中可能发生的错误
	SaveMonitorResourceStats(ctx context.Context, agentID int64, stats *types.MonitorResourceStats) error

	// GetMonitorResourceStats 获取监控代理资源统计信息
	// ctx: 上下文，用于控制操作超时
	// agentID: 代理ID
	// hours: 统计小时数
	// 返回资源统计信息列表和可能发生的错误
	GetMonitorResourceStats(ctx context.Context, agentID int64, hours int) ([]types.MonitorResourceStats, error)

	// SaveMonitorHistoricalSnapshot 保存监控代理历史快照
	// ctx: 上下文，用于控制操作超时
	// agentID: 代理ID
	// snapshot: 历史快照对象
	// 返回操作过程中可能发生的错误
	SaveMonitorHistoricalSnapshot(ctx context.Context, agentID int64, snapshot *types.MonitorHistoricalSnapshot) error

	// GetMonitorLatestSnapshot 获取监控代理最新的历史快照
	// ctx: 上下文，用于控制操作超时
	// agentID: 代理ID
	// periodType: 周期类型
	// 返回历史快照对象和可能发生的错误
	GetMonitorLatestSnapshot(ctx context.Context, agentID int64, periodType string) (*types.MonitorHistoricalSnapshot, error)

	// CleanupMonitorData 清理监控代理数据
	// ctx: 上下文，用于控制操作超时
	// 返回清理过程中可能发生的错误
	CleanupMonitorData(ctx context.Context) error

	// Embed NotificationService interface - 嵌入通知服务接口
	NotificationService
}

// service 结构体是 Service 接口的具体实现
// 包含数据库连接、配置和 SQL 构建器等核心组件
type service struct {
	db         *sql.DB                 // 底层的数据库连接对象
	config     config.DatabaseConfig   // 数据库配置信息
	sqlBuilder sq.StatementBuilderType // SQL 语句构建器，支持不同数据库的占位符格式
}

// 通用查询构建方法

// insert 执行插入操作
// ctx: 上下文，用于控制操作超时
// table: 表名
// data: 要插入的数据，键为列名，值为数据
// 返回 sql.Result 对象和可能发生的错误
func (s *service) insert(ctx context.Context, table string, data map[string]interface{}) (sql.Result, error) {
	cols := make([]string, 0, len(data))      // 列名切片
	vals := make([]interface{}, 0, len(data)) // 值切片

	// 遍历数据，构建列名和值切片
	for col, val := range data {
		cols = append(cols, col)
		vals = append(vals, val)
	}

	// 构建插入语句
	query := s.sqlBuilder.
		Insert(table).
		Columns(cols...).
		Values(vals...)

	// 执行插入操作并返回结果
	return query.RunWith(s.db).ExecContext(ctx)
}

// update 执行更新操作
// ctx: 上下文，用于控制操作超时
// table: 表名
// data: 要更新的数据，键为列名，值为数据
// where: 更新条件
// 返回 sql.Result 对象和可能发生的错误
func (s *service) update(ctx context.Context, table string, data map[string]interface{}, where sq.Eq) (sql.Result, error) {
	// 创建更新语句构建器
	query := s.sqlBuilder.Update(table)

	// 设置要更新的列和值
	for col, val := range data {
		query = query.Set(col, val)
	}

	// 设置更新条件并执行更新操作
	query = query.Where(where)
	return query.RunWith(s.db).ExecContext(ctx)
}

// delete 执行删除操作
// ctx: 上下文，用于控制操作超时
// table: 表名
// where: 删除条件
// 返回 sql.Result 对象和可能发生的错误
func (s *service) delete(ctx context.Context, table string, where sq.Eq) (sql.Result, error) {
	// 构建删除语句
	query := s.sqlBuilder.
		Delete(table).
		Where(where)

	// 执行删除操作并返回结果
	return query.RunWith(s.db).ExecContext(ctx)
}

// select_ 执行查询操作
// ctx: 上下文，用于控制操作超时
// table: 表名
// columns: 要查询的列名切片
// where: 查询条件
// 返回查询结果行集和可能发生的错误
func (s *service) select_(ctx context.Context, table string, columns []string, where sq.Eq) (*sql.Rows, error) {
	// 构建查询语句
	query := s.sqlBuilder.
		Select(columns...).
		From(table).
		Where(where)

	// 执行查询操作并返回结果行集
	return query.RunWith(s.db).QueryContext(ctx)
}

// count 执行计数查询
// ctx: 上下文，用于控制操作超时
// table: 表名
// where: 查询条件
// 返回记录数和可能发生的错误
func (s *service) count(ctx context.Context, table string, where sq.Eq) (int, error) {
	// 构建计数查询语句
	query := s.sqlBuilder.
		Select("COUNT(*)").
		From(table).
		Where(where)

	// 执行查询并扫描结果
	var count int
	err := query.RunWith(s.db).QueryRowContext(ctx).Scan(&count)
	return count, err
}

// QueryRow 执行查询并返回单个结果行
// ctx: 上下文，用于控制操作超时
// query: SQL 查询语句
// args: 查询参数
// 返回包含查询结果的 *sql.Row 对象
func (s *service) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

// dbInstance 是数据库服务的单例实例
// 确保整个应用中只有一个数据库连接实例
var dbInstance *service

// New 创建或返回数据库服务的单例实例
// 实现了单例模式，确保全局只有一个数据库连接
// cfg: 数据库配置信息
// 返回 Service 接口实现
func New(cfg config.DatabaseConfig) Service {
	// 单例模式检查：如果实例已存在，直接返回
	if dbInstance != nil {
		return dbInstance
	}

	var db *sql.DB                      // 数据库连接对象
	var err error                       // 错误变量
	var builder sq.StatementBuilderType // SQL 构建器

	// 根据数据库类型创建不同的连接
	switch cfg.Type {
	case config.Postgres:
		// 构建 PostgreSQL 连接字符串
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

		// 打开 PostgreSQL 数据库连接
		db, err = sql.Open("postgres", dsn)
		if err != nil {
			log.Fatal().Err(err).Msg("无法打开 PostgreSQL 数据库")
		}

		// PostgreSQL 使用 $ 作为占位符格式
		builder = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	case config.SQLite:
		// 获取 SQLite 数据库文件的绝对路径
		absPath, err := filepath.Abs(cfg.Path)
		if err != nil {
			log.Fatal().Err(err).Str("path", cfg.Path).Msg("无法获取数据库绝对路径")
		}
		cfg.Path = absPath
		dbDir := filepath.Dir(cfg.Path) // 数据库文件所在目录

		// 创建数据库目录（如果不存在）
		if err := os.MkdirAll(dbDir, 0o755); err != nil {
			log.Fatal().Err(err).Str("path", dbDir).Msg("无法创建数据库目录")
		}

		// 设置数据库目录权限
		if err := os.Chmod(dbDir, 0o755); err != nil {
			log.Fatal().Err(err).Msg("无法设置数据库目录权限")
		}

		// 打开 SQLite 数据库连接
		// 使用 shared cache、读写创建模式和外键约束
		db, err = sql.Open("sqlite", fmt.Sprintf("file:%s?cache=shared&mode=rwc&_foreign_keys=on", cfg.Path))
		if err != nil {
			log.Fatal().Err(err).Msg("无法打开 SQLite 数据库")
		}

		// 初始化 SQLite 数据库（设置 pragmas）
		if err := initializeSQLite(db); err != nil {
			log.Fatal().Err(err).Msg("无法初始化 SQLite 数据库")
		}

		// 设置数据库文件权限
		if err := os.Chmod(cfg.Path, 0o640); err != nil {
			log.Fatal().Err(err).Msg("无法设置数据库文件权限")
		}

		// SQLite 使用 ? 作为占位符格式
		builder = sq.StatementBuilder.PlaceholderFormat(sq.Question)
	}

	// 设置构建器使用的数据库连接
	builder = builder.RunWith(db)

	// 创建并返回数据库服务实例
	dbInstance = &service{
		db:         db,
		config:     cfg,
		sqlBuilder: builder,
	}

	return dbInstance
}

// initializeSQLite 初始化 SQLite 数据库的配置参数
// 优化数据库性能和功能
// db: SQLite 数据库连接对象
// 返回初始化过程中可能发生的错误
func initializeSQLite(db *sql.DB) error {
	// 检查数据库连接是否成功
	if err := db.Ping(); err != nil {
		return fmt.Errorf("无法创建数据库文件: %w", err)
	}

	// SQLite PRAGMA 参数列表，用于优化数据库性能和功能
	pragmas := []string{
		"PRAGMA busy_timeout = 5000",      // 设置忙超时为5秒，避免并发操作时的数据库锁定问题
		"PRAGMA journal_mode = wal",       // 使用 WAL (Write-Ahead Logging) 模式提高并发性能
		"PRAGMA analysis_limit = 400",     // 限制分析表的采样行数，提高分析速度
		"PRAGMA wal_checkpoint(TRUNCATE)", // 执行 WAL 检查点并截断日志文件
		"PRAGMA foreign_keys = ON",        // 启用外键约束
	}

	// 执行所有 PRAGMA 参数设置
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("执行 %s 失败: %w", pragma, err)
		}
	}

	return nil
}

// Health 检查数据库连接健康状态并返回详细统计信息
// 返回包含数据库状态和性能指标的映射
func (s *service) Health() map[string]string {
	// 创建一个1秒超时的上下文，用于数据库Ping操作
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel() // 确保函数退出时释放上下文资源

	// 初始化统计信息映射
	stats := make(map[string]string)

	// 尝试与数据库建立连接
	if err := s.db.PingContext(ctx); err != nil {
		stats["status"] = "down" // 数据库状态为不可用
		stats["error"] = fmt.Sprintf("数据库不可用: %v", err)
		log.Error().Err(err).Msg("数据库连接失败")
		return stats
	}

	// 数据库连接正常
	stats["status"] = "up"
	stats["message"] = "数据库连接正常"
	stats["type"] = string(s.config.Type) // 数据库类型

	// 获取数据库连接池统计信息
	dbStats := s.db.Stats()
	stats["open_connections"] = strconv.Itoa(dbStats.OpenConnections)               // 打开的连接数
	stats["in_use"] = strconv.Itoa(dbStats.InUse)                                   // 当前正在使用的连接数
	stats["idle"] = strconv.Itoa(dbStats.Idle)                                      // 空闲连接数
	stats["wait_count"] = strconv.FormatInt(dbStats.WaitCount, 10)                  // 等待连接的总次数
	stats["wait_duration"] = dbStats.WaitDuration.String()                          // 等待连接的总时间
	stats["max_idle_closed"] = strconv.FormatInt(dbStats.MaxIdleClosed, 10)         // 因空闲超时关闭的连接数
	stats["max_lifetime_closed"] = strconv.FormatInt(dbStats.MaxLifetimeClosed, 10) // 因生命周期超时关闭的连接数

	// 评估健康统计信息，根据指标调整状态信息
	s.evaluateHealthStats(dbStats, stats)

	return stats
}

// evaluateHealthStats 评估数据库连接池的健康统计信息
// 根据统计数据调整健康状态信息
// dbStats: 数据库连接池统计信息
// stats: 要更新的健康状态映射
func (s *service) evaluateHealthStats(dbStats sql.DBStats, stats map[string]string) {
	// 检查连接数是否过高
	if dbStats.OpenConnections > 40 {
		stats["message"] = "数据库正处于高负载状态。"
	}
	// 检查等待事件是否过多
	if dbStats.WaitCount > 1000 {
		stats["message"] = "数据库有大量等待事件，可能存在性能瓶颈。"
	}
	// 检查空闲连接关闭率是否过高
	if dbStats.MaxIdleClosed > int64(dbStats.OpenConnections)/2 {
		stats["message"] = "许多空闲连接正在被关闭，建议调整连接池设置。"
	}
	// 检查生命周期连接关闭率是否过高
	if dbStats.MaxLifetimeClosed > int64(dbStats.OpenConnections)/2 {
		stats["message"] = "许多连接因生命周期超时被关闭，建议增加最大生命周期。"
	}
}

// Close 关闭数据库连接并释放资源
// 返回关闭过程中可能发生的错误
func (s *service) Close() error {
	log.Info().
		Str("type", string(s.config.Type)).
		Msg("已断开与数据库的连接")
	return s.db.Close()
}

// getMigrationVersion 从迁移文件名中提取版本号
// fileName: 迁移文件的路径或名称
// 返回解析出的版本号，如果解析失败则返回0
func getMigrationVersion(fileName string) int {
	// 从路径中提取文件名
	parts := strings.Split(fileName, "/")
	if len(parts) > 0 {
		fileName = parts[len(parts)-1]
	}

	// 从文件名中提取版本号（假设文件名格式为 "version_description.sql"）
	parts = strings.Split(fileName, "_")
	if len(parts) > 0 {
		if v, err := strconv.Atoi(parts[0]); err == nil {
			return v
		}
	}
	return 0 // 如果无法解析版本号，返回0
}

// InitializeTables 初始化数据库表结构
// 应用所有必要的数据库迁移
// ctx: 上下文对象，用于控制函数执行
// 返回迁移过程中可能发生的错误
func (s *service) InitializeTables(ctx context.Context) error {
	// 检测是否处于测试环境，以减少日志输出
	isTest := strings.Contains(os.Args[0], ".test") || strings.HasSuffix(os.Args[0], "/test")
	// 创建日志适配器，在测试环境中减少日志输出
	logger := &ZerologAdapter{logger: log.Logger, quiet: isTest}
	// 创建迁移器实例
	m := migrator.NewMigrate(s.db,
		migrator.WithLogger(logger),                       // 设置日志记录器
		migrator.WithEmbedFS(migrations.SchemaMigrations), // 设置迁移文件的嵌入式文件系统
	)

	// 获取与当前数据库类型匹配的迁移文件
	migrationFiles, err := migrations.GetMigrationFiles(migrations.DatabaseType(s.config.Type))
	if err != nil {
		return fmt.Errorf("获取迁移文件失败: %w", err)
	}

	// 为每个迁移文件创建迁移对象并添加到迁移器
	for _, fileName := range migrationFiles {
		m.Add(&migrator.Migration{
			Name: fileName, // 迁移名称
			File: fileName, // 迁移文件路径
		})
	}

	// 应用所有迁移
	if err := m.Migrate(); err != nil {
		return fmt.Errorf("应用迁移失败: %w", err)
	}

	return nil
}
