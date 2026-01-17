// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later

package database

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/netronome/internal/config"
)

// TestMigrations_FreshDatabase 测试新数据库上的迁移是否正确创建所有必要的表
// 使用 RunTestWithBothDatabases 在两种数据库(SQLite 和 PostgreSQL)上运行测试
func TestMigrations_FreshDatabase(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 动态从数据库中获取所有表名
		var tables []string

		// 根据数据库类型执行不同的查询来获取表名
		switch td.Config.Type {
		case config.Postgres:
			// PostgreSQL 查询：从 information_schema 获取所有表名
			rows, err := td.DB.Query(`
				SELECT table_name 
				FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_type = 'BASE TABLE'
				AND table_name != 'schema_migrations'
				ORDER BY table_name
			`)
			require.NoError(t, err) // 确保查询执行成功
			defer rows.Close()      // 确保函数退出时关闭结果集

			// 遍历结果集，收集表名
			for rows.Next() {
				var tableName string
				err := rows.Scan(&tableName)
				require.NoError(t, err)
				tables = append(tables, tableName)
			}
			require.NoError(t, rows.Err()) // 检查遍历过程中是否发生错误

		case config.SQLite:
			// SQLite 查询：从 sqlite_master 获取所有表名
			rows, err := td.DB.Query(`
				SELECT name FROM sqlite_master 
				WHERE type='table' 
				AND name NOT IN ('sqlite_sequence', 'schema_migrations')
				ORDER BY name
			`)
			require.NoError(t, err) // 确保查询执行成功
			defer rows.Close()      // 确保函数退出时关闭结果集

			// 遍历结果集，收集表名
			for rows.Next() {
				var tableName string
				err := rows.Scan(&tableName)
				require.NoError(t, err)
				tables = append(tables, tableName)
			}
			require.NoError(t, rows.Err()) // 检查遍历过程中是否发生错误
		}

		// 验证数据库中创建了足够数量的表
		assert.Greater(t, len(tables), 10, "迁移后至少应该有10个表")

		// 记录找到的表名，用于调试
		t.Logf("找到 %d 个表: %v", len(tables), tables)

		// 验证一些应该始终存在的核心表
		coreTablesMap := map[string]bool{
			"users":                false, // 用户表
			"speed_tests":          false, // 速度测试表
			"packet_loss_monitors": false, // 丢包监控表
			"monitor_agents":       false, // 监控代理表
		}

		// 检查核心表是否都存在
		for _, table := range tables {
			if _, exists := coreTablesMap[table]; exists {
				coreTablesMap[table] = true
			}
		}

		// 验证所有核心表都被找到
		for table, found := range coreTablesMap {
			assert.True(t, found, "核心表 '%s' 应该存在", table)
		}
	})
}

// TestMigrations_SchemaVersion 测试数据库迁移的版本管理是否正确
// 验证应用的迁移数量与迁移文件数量一致，并且迁移版本格式正确
func TestMigrations_SchemaVersion(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 检查 schema_migrations 表是否存在并包含迁移记录
		var count int
		err := td.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
		require.NoError(t, err)
		assert.Greater(t, count, 0, "应该有迁移记录")

		// 获取最新的版本字符串（完整文件名）
		var latestVersion string
		err = td.DB.QueryRow("SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&latestVersion)
		require.NoError(t, err)
		assert.NotEmpty(t, latestVersion, "应该有最新的迁移版本")

		// 记录版本格式，用于调试
		t.Logf("最新迁移版本格式: %s", latestVersion)

		// 计算实际的迁移文件数量，确保所有迁移都已执行
		migrationFiles, err := countMigrationFiles(td.Config.Type)
		require.NoError(t, err)

		// schema_migrations 中的记录数应该与迁移文件数量一致
		assert.Equal(t, migrationFiles, count,
			"已应用的迁移数量应该与迁移文件数量一致")

		// 验证最新版本具有预期的格式
		if td.Config.Type == config.Postgres {
			assert.Contains(t, latestVersion, "_postgres.sql", "PostgreSQL 迁移文件应该以 _postgres.sql 结尾")
		} else {
			assert.Contains(t, latestVersion, ".sql", "SQLite 迁移文件应该以 .sql 结尾")
			assert.NotContains(t, latestVersion, "_postgres", "SQLite 迁移文件不应该包含 _postgres")
		}
	})
}

// countMigrationFiles 计算指定数据库类型的迁移文件数量
// dbType: 数据库类型（SQLite 或 PostgreSQL）
// 返回迁移文件数量和可能发生的错误
// 辅助函数，用于测试数据库迁移
func countMigrationFiles(dbType config.DatabaseType) (int, error) {
	var suffix string // 文件后缀
	var dir string    // 目录名称

	// 根据数据库类型设置不同的文件后缀和目录
	switch dbType {
	case config.SQLite:
		suffix = ".sql"
		dir = "sqlite"
	case config.Postgres:
		suffix = "_postgres.sql"
		dir = "postgres"
	default:
		return 0, fmt.Errorf("不支持的数据库类型: %v", dbType)
	}

	// 读取迁移文件目录
	entries, err := os.ReadDir(fmt.Sprintf("./migrations/%s", dir))
	if err != nil {
		return 0, err
	}

	// 计算符合条件的迁移文件数量
	count := 0
	for _, entry := range entries {
		// 只计算符合后缀条件的文件（非目录）
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
			count++
		}
	}

	return count, nil
}
