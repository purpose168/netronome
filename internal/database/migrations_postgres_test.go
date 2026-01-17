// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later

package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/netronome/internal/config"
)

// TestPostgreSQL_MigrationValidation 验证 PostgreSQL 特定功能的迁移
// 仅针对 PostgreSQL 数据库运行此测试
func TestPostgreSQL_MigrationValidation(t *testing.T) {
	// 仅为 PostgreSQL 设置测试数据库
	td := SetupTestDatabase(t, config.Postgres)
	defer td.Close() // 确保测试结束后关闭数据库连接

	ctx := context.Background() // 创建上下文用于数据库操作

	t.Run("ValidateDataTypes", func(t *testing.T) {
		// 验证 PostgreSQL 特定的数据类型
		rows, err := td.DB.QueryContext(ctx, `
			SELECT 
				table_name,
				column_name,
				data_type,
				is_nullable,
				column_default
			FROM information_schema.columns
			WHERE table_schema = 'public'
			AND table_name NOT IN ('schema_migrations')
			ORDER BY table_name, ordinal_position
		`)
		require.NoError(t, err) // 确保查询执行成功
		defer rows.Close()      // 确保函数退出时关闭结果集

		// 收集所有表的数据类型信息
		dataTypes := make(map[string][]string)
		for rows.Next() {
			var tableName, columnName, dataType, isNullable string
			var columnDefault sql.NullString
			err := rows.Scan(&tableName, &columnName, &dataType, &isNullable, &columnDefault)
			require.NoError(t, err)

			// 保存表名和列名:数据类型的映射
			dataTypes[tableName] = append(dataTypes[tableName],
				fmt.Sprintf("%s:%s", columnName, dataType))
		}

		// 记录一些数据类型信息用于调试
		for table, types := range dataTypes {
			if table == "users" || table == "packet_loss_monitors" {
				t.Logf("表 %s 的列: %v", table, types)
			}
		}

		// 验证预期的数据类型（对整数类型保持灵活）
		hasUserID := false
		hasUserCreatedAt := false
		hasPacketLossEnabled := false
		hasPacketLossThreshold := false

		// 检查 users 表的数据类型
		for _, col := range dataTypes["users"] {
			if strings.HasPrefix(col, "id:") && strings.Contains(col, "int") {
				hasUserID = true
			}
			if strings.HasPrefix(col, "created_at:timestamp") {
				hasUserCreatedAt = true
				// 注意：模式使用 "timestamp without time zone" 而不是 "with time zone"
				// 这可能是有意的，也可能在时区处理方面需要改进
			}
		}

		// 检查 packet_loss_monitors 表的数据类型
		for _, col := range dataTypes["packet_loss_monitors"] {
			if col == "enabled:boolean" {
				hasPacketLossEnabled = true
			}
			if col == "threshold:real" || col == "threshold:double precision" {
				hasPacketLossThreshold = true
			}
		}

		// 验证必要的数据类型是否存在
		assert.True(t, hasUserID, "users 表应该有 integer 类型的 id 列")
		assert.True(t, hasUserCreatedAt, "users 表应该有 timestamp 类型的 created_at 列")
		assert.True(t, hasPacketLossEnabled, "packet_loss_monitors 表应该有 boolean 类型的 enabled 列")
		assert.True(t, hasPacketLossThreshold, "packet_loss_monitors 表应该有 real/double precision 类型的 threshold 列")
	})

	t.Run("ValidateForeignKeys", func(t *testing.T) {
		// 获取所有外键约束
		rows, err := td.DB.QueryContext(ctx, `
			SELECT 
				tc.table_name,
				kcu.column_name,
				ccu.table_name AS foreign_table_name,
				ccu.column_name AS foreign_column_name,
				rc.delete_rule
			FROM information_schema.table_constraints AS tc 
			JOIN information_schema.key_column_usage AS kcu
				ON tc.constraint_name = kcu.constraint_name
				AND tc.table_schema = kcu.table_schema
			JOIN information_schema.constraint_column_usage AS ccu
				ON ccu.constraint_name = tc.constraint_name
				AND ccu.table_schema = tc.table_schema
			JOIN information_schema.referential_constraints AS rc
				ON rc.constraint_name = tc.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY' 
			AND tc.table_schema = 'public'
		`)
		require.NoError(t, err) // 确保查询执行成功
		defer rows.Close()      // 确保函数退出时关闭结果集

		fkCount := 0                 // 外键约束数量
		cascadeDeletes := []string{} // 级联删除关系列表

		// 遍历所有外键约束
		for rows.Next() {
			var tableName, columnName, foreignTable, foreignColumn, deleteRule string
			err := rows.Scan(&tableName, &columnName, &foreignTable, &foreignColumn, &deleteRule)
			require.NoError(t, err)

			fkCount++
			// 记录级联删除关系
			if deleteRule == "CASCADE" {
				cascadeDeletes = append(cascadeDeletes,
					fmt.Sprintf("%s.%s -> %s.%s", tableName, columnName, foreignTable, foreignColumn))
			}

			t.Logf("外键: %s.%s -> %s.%s (DELETE %s)",
				tableName, columnName, foreignTable, foreignColumn, deleteRule)
		}

		// 验证外键约束数量
		assert.Greater(t, fkCount, 5, "应该有多个外键约束")
		assert.NotEmpty(t, cascadeDeletes, "应该有 CASCADE DELETE 关系")
	})

	t.Run("ValidateIndexes", func(t *testing.T) {
		// 获取所有索引
		rows, err := td.DB.QueryContext(ctx, `
			SELECT 
				schemaname,
				tablename,
				indexname,
				indexdef
			FROM pg_indexes
			WHERE schemaname = 'public'
			AND indexname NOT LIKE '%_pkey'
			ORDER BY tablename, indexname
		`)
		require.NoError(t, err) // 确保查询执行成功
		defer rows.Close()      // 确保函数退出时关闭结果集

		indexes := make(map[string][]string) // 按表名分组的索引列表
		for rows.Next() {
			var schema, table, indexName, indexDef string
			err := rows.Scan(&schema, &table, &indexName, &indexDef)
			require.NoError(t, err)

			indexes[table] = append(indexes[table], indexName)

			// 检查重要的索引模式
			if strings.Contains(indexDef, "UNIQUE") {
				t.Logf("唯一索引: %s 在 %s 表上", indexName, table)
			}
		}

		// 验证存在索引
		assert.NotEmpty(t, indexes, "应该存在索引")

		// 检查特定表是否有索引
		if indices, ok := indexes["packet_loss_results"]; ok {
			assert.NotEmpty(t, indices, "packet_loss_results 表应该有索引")
			t.Logf("packet_loss_results 表的索引: %v", indices)
		}

		if indices, ok := indexes["notification_rules"]; ok {
			assert.NotEmpty(t, indices, "notification_rules 表应该有索引")
			t.Logf("notification_rules 表的索引: %v", indices)
		}
	})

	t.Run("ValidateTriggers", func(t *testing.T) {
		// 检查更新时间戳触发器
		rows, err := td.DB.QueryContext(ctx, `
			SELECT 
				trigger_name,
				event_object_table,
				action_timing,
				event_manipulation
			FROM information_schema.triggers
			WHERE trigger_schema = 'public'
		`)
		require.NoError(t, err) // 确保查询执行成功
		defer rows.Close()      // 确保函数退出时关闭结果集

		triggerCount := 0            // 触发器数量
		updateTriggers := []string{} // 更新触发器列表

		for rows.Next() {
			var triggerName, tableName, timing, event string
			err := rows.Scan(&triggerName, &tableName, &timing, &event)
			require.NoError(t, err)

			triggerCount++
			// 收集更新触发器
			if strings.Contains(triggerName, "update") && event == "UPDATE" {
				updateTriggers = append(updateTriggers, tableName)
			}
		}

		// 验证具有 updated_at 列的表存在更新触发器
		if triggerCount > 0 {
			assert.Contains(t, updateTriggers, "notification_channels")
			assert.Contains(t, updateTriggers, "notification_rules")
		}
	})

	t.Run("ValidateCheckConstraints", func(t *testing.T) {
		// 获取检查约束
		rows, err := td.DB.QueryContext(ctx, `
			SELECT 
				tc.table_name,
				tc.constraint_name,
				cc.check_clause
			FROM information_schema.table_constraints tc
			JOIN information_schema.check_constraints cc
				ON tc.constraint_name = cc.constraint_name
				AND tc.constraint_schema = cc.constraint_schema
			WHERE tc.constraint_type = 'CHECK'
			AND tc.table_schema = 'public'
		`)
		require.NoError(t, err) // 确保查询执行成功
		defer rows.Close()      // 确保函数退出时关闭结果集

		// 遍历所有检查约束
		for rows.Next() {
			var tableName, constraintName, checkClause string
			err := rows.Scan(&tableName, &constraintName, &checkClause)
			require.NoError(t, err)

			t.Logf("表 %s 上的检查约束: %s - %s", tableName, constraintName, checkClause)
		}
	})

	t.Run("ValidateSequences", func(t *testing.T) {
		// 验证 SERIAL 列的序列
		rows, err := td.DB.QueryContext(ctx, `
			SELECT 
				sequence_name,
				start_value,
				increment
			FROM information_schema.sequences
			WHERE sequence_schema = 'public'
		`)
		require.NoError(t, err) // 确保查询执行成功
		defer rows.Close()      // 确保函数退出时关闭结果集

		sequenceCount := 0 // 序列数量
		for rows.Next() {
			var seqName string
			var startValue, increment sql.NullInt64
			err := rows.Scan(&seqName, &startValue, &increment)
			require.NoError(t, err)
			sequenceCount++
			t.Logf("序列: %s (起始值: %d, 增量: %d)", seqName, startValue.Int64, increment.Int64)
		}

		// 每个具有 SERIAL 主键的表都应该有一个序列
		assert.Greater(t, sequenceCount, 10, "应该为 SERIAL 列创建序列")
	})

	t.Run("TestTransactionIsolation", func(t *testing.T) {
		// 测试事务是否正常工作
		tx, err := td.DB.BeginTx(ctx, nil)
		require.NoError(t, err) // 确保事务开始成功
		defer tx.Rollback()     // 确保函数退出时回滚事务（除非显式提交）

		// 在事务中插入测试数据
		_, err = tx.Exec(`
			INSERT INTO users (username, password_hash) 
			VALUES ($1, $2)
		`, "tx_test_user", "hash")
		require.NoError(t, err) // 确保插入成功

		// 验证数据在事务中可见
		var count int
		err = tx.QueryRow("SELECT COUNT(*) FROM users WHERE username = $1", "tx_test_user").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count) // 事务内应该能看到插入的数据

		// 回滚并验证数据已消失
		err = tx.Rollback()
		require.NoError(t, err) // 确保回滚成功

		// 验证数据已从数据库中删除
		err = td.DB.QueryRow("SELECT COUNT(*) FROM users WHERE username = $1", "tx_test_user").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count) // 回滚后数据应该不存在
	})

	t.Run("ValidatePerformanceFeatures", func(t *testing.T) {
		// 检查与性能相关的设置

		// 验证重要索引存在
		var indexCount int
		err := td.DB.QueryRow(`
			SELECT COUNT(*) 
			FROM pg_indexes 
			WHERE schemaname = 'public'
			AND tablename = 'packet_loss_results'
		`).Scan(&indexCount)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, indexCount, 2, "packet_loss_results 表应该有性能相关的索引")

		// 检查部分索引或其他优化
		rows, err := td.DB.QueryContext(ctx, `
			SELECT indexdef 
			FROM pg_indexes 
			WHERE schemaname = 'public'
			AND indexdef LIKE '%WHERE%'
		`)
		require.NoError(t, err) // 确保查询执行成功
		defer rows.Close()      // 确保函数退出时关闭结果集

		// 遍历所有部分索引
		for rows.Next() {
			var indexDef string
			err := rows.Scan(&indexDef)
			require.NoError(t, err)
			t.Logf("发现部分索引: %s", indexDef)
		}
	})
}

// TestMigrationRollbackPrevention 确保迁移不能被回滚
// 在两种数据库(SQLite 和 PostgreSQL)上运行此测试
func TestMigrationRollbackPrevention(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 尝试从 schema_migrations 删除（在生产环境中应该失败）
		// 这只是为了记录预期行为

		var count int
		err := td.DB.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
		require.NoError(t, err)
		assert.Greater(t, count, 0, "应该有迁移记录")

		// 在生产系统中，这应该由权限阻止
		// 对于测试，我们只验证表存在并包含记录
	})
}

// TestMigrationIdempotency 验证迁移可以安全地多次运行
func TestMigrationIdempotency(t *testing.T) {
	// 此测试需要访问迁移运行器
	// 当前我们只能验证迁移已应用一次
	t.Skip("需要访问迁移运行器")
}
