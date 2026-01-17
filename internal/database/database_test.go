// Copyright (c) 2024-2025, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// 数据库包包含数据库测试和测试辅助函数
//
// 常见测试模式：
//
// 1. 时间范围查询测试：
//    测试基于时间的查询（24小时、周、月）时，在以下时间点创建记录：
//    - 当前时间、23小时前、25小时前（用于24小时边界测试）
//    - 6天前、8天前（用于周边界测试）
//    - 29天前、32天前（用于月边界测试）
//
// 2. 级联删除测试：
//    - 创建父记录
//    - 创建多个子记录
//    - 删除父记录
//    - 验证子记录已被删除
//
// 3. 唯一约束测试：
//    - 插入第一条记录（应该成功）
//    - 插入重复记录（应该失败）
//    - 插入不同值的记录（应该成功）
//
// 这些模式在以下文件中进行了全面测试：
// - 时间范围：speedtest_integration_test.go
// - 级联删除：monitor_integration_test.go, packetloss_integration_test.go
// - 约束验证：core_integration_test.go
//
// 请避免在新文件中重复这些测试。
//
// 性能提示：
// - 对于不需要PostgreSQL的简单测试，使用RunTestWithSQLiteOnly()
// - 在本地开发时，设置SKIP_POSTGRES_TESTS=1来跳过PostgreSQL测试
// - 由于嵌入式数据库初始化，每个PostgreSQL测试大约需要6秒

package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/netronome/internal/config"
	"github.com/autobrr/netronome/internal/types"
)

// 所有测试共享的PostgreSQL实例
var (
	sharedPostgres    *embeddedpostgres.EmbeddedPostgres
	sharedConfig      config.DatabaseConfig
	sharedInitOnce    sync.Once
	sharedInitError   error
	sharedCleanupOnce sync.Once
)

// TestMain 为整个测试套件管理共享的 PostgreSQL 实例
func TestMain(m *testing.M) {
	// 设置共享的 PostgreSQL 实例
	setupSharedPostgreSQL()

	// 运行所有测试
	code := m.Run()

	// 清理共享的 PostgreSQL 实例
	cleanupSharedPostgreSQL()

	os.Exit(code)
}

// setupSharedPostgreSQL 初始化共享的 PostgreSQL 实例，确保只初始化一次
func setupSharedPostgreSQL() {
	sharedInitOnce.Do(func() {
		// 如果禁用了 PostgreSQL 测试，则跳过
		if os.Getenv("SKIP_POSTGRES_TESTS") != "" {
			log.Println("跳过 PostgreSQL 设置 (SKIP_POSTGRES_TESTS 已设置)")
			return
		}

		// 获取可用端口
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			sharedInitError = fmt.Errorf("无法找到可用端口: %w", err)
			return
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()

		// 为共享的 postgres 创建临时目录
		tempDir, err := os.MkdirTemp("", "netronome-shared-postgres-*")
		if err != nil {
			sharedInitError = fmt.Errorf("无法创建临时目录: %w", err)
			return
		}

		log.Printf("在端口 %d 上启动共享 PostgreSQL", port)

		// 设置嵌入式 PostgreSQL
		postgres := embeddedpostgres.NewDatabase(
			embeddedpostgres.DefaultConfig().
				Username("test").
				Password("test").
				Database("testdb").
				Port(uint32(port)).
				RuntimePath(filepath.Join(tempDir, "postgres-runtime")),
		)

		if err := postgres.Start(); err != nil {
			sharedInitError = fmt.Errorf("无法启动嵌入式 postgres: %w", err)
			return
		}

		sharedPostgres = postgres
		sharedConfig = config.DatabaseConfig{
			Type:     config.Postgres,
			Host:     "localhost",
			Port:     port,
			User:     "test",
			Password: "test",
			DBName:   "testdb",
			SSLMode:  "disable",
		}

		// 初始化一次 schema
		log.Println("正在初始化共享 PostgreSQL schema...")
		dbInstance = nil // 重置单例以进行干净初始化
		service := New(sharedConfig)
		defer service.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if err := service.InitializeTables(ctx); err != nil {
			sharedInitError = fmt.Errorf("无法初始化表: %w", err)
			postgres.Stop()
			sharedPostgres = nil
			return
		}

		log.Println("共享 PostgreSQL 已准备好用于测试")
	})
}

// ensureSharedPostgreSQL 检查共享的 PostgreSQL 实例是否已准备好
func ensureSharedPostgreSQL() error {
	if sharedInitError != nil {
		return sharedInitError
	}
	if sharedPostgres == nil {
		return fmt.Errorf("共享 PostgreSQL 未初始化")
	}
	return nil
}

// cleanupSharedPostgreSQL 停止共享的 PostgreSQL 实例
func cleanupSharedPostgreSQL() {
	sharedCleanupOnce.Do(func() {
		if sharedPostgres != nil {
			log.Println("正在停止共享 PostgreSQL...")
			if err := sharedPostgres.Stop(); err != nil {
				log.Printf("警告: 无法停止共享 PostgreSQL: %v", err)
			}
			sharedPostgres = nil
		}
	})
}

// 测试之间需要清理的表（按依赖顺序排列以处理外键）
var testTablesToClear = []string{
	"notification_history",
	"notification_rules",
	"notification_channels",
	"packet_loss_results",
	"packet_loss_monitors",
	"monitor_historical_snapshots",
	"monitor_resource_stats",
	"monitor_peak_stats",
	"monitor_agent_interfaces",
	"monitor_agent_system_info",
	"monitor_agents",
	"speed_tests",
	"saved_iperf_servers",
	"users",
	// 保留：notification_events, notification_categories, schema_migrations, registration_status
}

// cleanTestData 移除所有测试数据，但保留 schema 和种子数据
func cleanTestData(t *testing.T, db *sql.DB) {
	t.Helper()

	// 使用 TRUNCATE CASCADE 提高速度 (PostgreSQL)
	for _, table := range testTablesToClear {
		_, err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			// 有些表可能不存在或没有数据，这没关系
			t.Logf("注意: TRUNCATE %s 失败 (可能为空): %v", table, err)
		}
	}

	// 重置序列以确保测试间的 ID 一致性
	resetSequences(t, db)
}

// resetSequences 将 PostgreSQL 序列重置为从 1 开始
func resetSequences(t *testing.T, db *sql.DB) {
	t.Helper()

	// 获取数据库中的所有序列
	rows, err := db.Query(`
		SELECT schemaname, sequencename 
		FROM pg_sequences 
		WHERE schemaname = 'public'
	`)
	if err != nil {
		t.Logf("警告: 无法获取序列: %v", err)
		return
	}
	defer rows.Close()

	var sequences []string
	for rows.Next() {
		var schema, seqName string
		if err := rows.Scan(&schema, &seqName); err != nil {
			continue
		}
		sequences = append(sequences, seqName)
	}

	// 将每个序列重置为从 1 开始
	for _, seq := range sequences {
		_, err := db.Exec(fmt.Sprintf("ALTER SEQUENCE %s RESTART WITH 1", seq))
		if err != nil {
			t.Logf("警告: 无法重置序列 %s: %v", seq, err)
		}
	}
}

// TestDatabase 提供一个测试数据库实例
type TestDatabase struct {
	Service  Service                            // 数据库服务接口，用于执行数据库操作
	DB       *sql.DB                            // 底层的 *sql.DB 实例，用于直接执行SQL
	Config   config.DatabaseConfig              // 数据库配置信息
	postgres *embeddedpostgres.EmbeddedPostgres // PostgreSQL 实例引用（仅用于PostgreSQL测试）
	cleanup  func()                             // 清理函数，用于释放资源
}

// Close 清理测试数据库资源
func (td *TestDatabase) Close() error {
	if td.cleanup != nil {
		td.cleanup()
	}
	if td.Service != nil {
		td.Service.Close()
	}
	if td.postgres != nil {
		return td.postgres.Stop()
	}
	return nil
}

// SetupTestDatabase 创建一个测试数据库实例
func SetupTestDatabase(t *testing.T, dbType config.DatabaseType) *TestDatabase {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	td := &TestDatabase{}

	switch dbType {
	case config.Postgres:
		// 使用共享的 PostgreSQL 实例
		if err := ensureSharedPostgreSQL(); err != nil {
			if os.Getenv("SKIP_POSTGRES_TESTS") != "" {
				t.Skip("PostgreSQL 测试已跳过 (SKIP_POSTGRES_TESTS 已设置)")
			}
			t.Fatalf("共享 PostgreSQL 不可用: %v", err)
		}

		td.Config = sharedConfig
		// 不设置 postgres 字段 - 使用共享实例，不需要单独清理

	case config.SQLite:
		// 在临时目录中设置 SQLite
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "test.db")

		td.Config = config.DatabaseConfig{
			Type: config.SQLite,
			Path: dbPath,
		}

		td.cleanup = func() {
			os.Remove(dbPath)
		}

	default:
		t.Fatalf("不支持的数据库类型: %v", dbType)
	}

	// 重置单例以便测试
	dbInstance = nil

	// 创建服务实例
	td.Service = New(td.Config)

	// 获取底层的 *sql.DB 以便直接访问（如果需要）
	if svc, ok := td.Service.(*service); ok {
		td.DB = svc.db
	}

	// 根据数据库类型初始化或清理表
	if td.Config.Type == config.Postgres {
		// 对于 PostgreSQL，清理现有数据而不是初始化新表
		cleanTestData(t, td.DB)
	} else {
		// 对于 SQLite，像以前一样初始化新表
		if err := td.Service.InitializeTables(ctx); err != nil {
			t.Fatalf("初始化表失败: %v", err)
		}
	}

	return td
}

// RunTestWithBothDatabases 在 SQLite 和 PostgreSQL 上运行测试函数
func RunTestWithBothDatabases(t *testing.T, testFunc func(t *testing.T, td *TestDatabase)) {
	t.Run("SQLite", func(t *testing.T) {
		td := SetupTestDatabase(t, config.SQLite)
		defer td.Close()
		testFunc(t, td)
	})

	// 如果设置了 SKIP_POSTGRES_TESTS，则跳过 PostgreSQL 测试（用于更快的本地开发）
	if os.Getenv("SKIP_POSTGRES_TESTS") != "" {
		t.Log("跳过 PostgreSQL 测试 (SKIP_POSTGRES_TESTS 已设置)")
		return
	}

	t.Run("PostgreSQL", func(t *testing.T) {
		td := SetupTestDatabase(t, config.Postgres)
		defer td.Close()
		testFunc(t, td)
	})
}

// RunTestWithSQLiteOnly 仅在 SQLite 上运行测试函数（用于更快的开发）
func RunTestWithSQLiteOnly(t *testing.T, testFunc func(t *testing.T, td *TestDatabase)) {
	td := SetupTestDatabase(t, config.SQLite)
	defer td.Close()
	testFunc(t, td)
}

// AssertRecordExists 检查数据库中是否存在记录
func AssertRecordExists(t *testing.T, td *TestDatabase, table string, column string, value any) {
	t.Helper()

	// 根据数据库类型构建查询
	var query string
	if td.Config.Type == config.Postgres {
		query = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = $1", table, column)
	} else {
		query = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", table, column)
	}

	var count int
	err := td.DB.QueryRow(query, value).Scan(&count)
	require.NoError(t, err)
	require.Greater(t, count, 0, "预期记录应存在于 %s 表中，条件为 %s = %v", table, column, value)
}

// AssertRecordNotExists 检查数据库中是否不存在记录
func AssertRecordNotExists(t *testing.T, td *TestDatabase, table string, column string, value any) {
	t.Helper()

	// 根据数据库类型构建查询
	var query string
	if td.Config.Type == config.Postgres {
		query = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = $1", table, column)
	} else {
		query = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", table, column)
	}

	var count int
	err := td.DB.QueryRow(query, value).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count, "预期 %s 表中不应存在满足条件 %s = %v 的记录", table, column, value)
}

// CreateTestUser 在数据库中创建一个测试用户
func CreateTestUser(t *testing.T, td *TestDatabase, username, password string) *User {
	t.Helper()

	ctx := context.Background()
	user, err := td.Service.CreateUser(ctx, username, password)
	require.NoError(t, err)
	require.NotNil(t, user)
	return user
}

// CreateTestPacketLossMonitor 创建一个测试的丢包监控器
func CreateTestPacketLossMonitor(t *testing.T, td *TestDatabase) *types.PacketLossMonitor {
	t.Helper()

	monitor := &types.PacketLossMonitor{
		Name:        "测试监控器",
		Host:        "8.8.8.8",
		Interval:    "60s",
		PacketCount: 10,
		Enabled:     true,
		Threshold:   5.0,
		LastState:   "healthy",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	result, err := td.Service.CreatePacketLossMonitor(monitor)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Greater(t, result.ID, int64(0))

	return result
}
