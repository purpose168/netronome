// Copyright (c) 2024-2025, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// 数据库核心集成测试
// 该文件包含了 Netronome 项目中数据库核心功能的集成测试
// 主要验证数据库的基本操作、事务处理、约束验证和性能等方面
//
// 测试内容包括：
// 1. 数据库健康状态检查 (TestDatabaseHealth)
// 2. 用户管理功能 (TestUserManagement)
// 3. 事务行为测试 (TestTransactionBehavior)
// 4. 数据库超时处理 (TestDatabaseTimeout)
// 5. QueryRow 方法测试 (TestQueryRowMethod)
// 6. 数据库约束验证 (TestDatabaseConstraints)
// 7. 大数据处理能力测试 (TestLargeDataHandling)
//
// 测试方法：
// - 所有测试都在 SQLite 和 PostgreSQL 两种数据库上运行
// - 使用 RunTestWithBothDatabases 函数确保跨数据库兼容性
// - 使用 testify 框架进行断言验证
// - 测试覆盖了正常和异常情况
package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/netronome/internal/types"
)

// TestDatabaseHealth 测试数据库健康状态检查功能
// 该函数验证数据库服务的健康状态报告是否正确
// 测试在 SQLite 和 PostgreSQL 两种数据库上运行
func TestDatabaseHealth(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 获取数据库健康状态信息
		health := td.Service.Health()

		// 验证健康状态基本信息
		assert.Equal(t, "up", health["status"])        // 状态应该是 "up"
		assert.Contains(t, health, "message")          // 应该包含消息
		assert.Contains(t, health, "type")             // 应该包含数据库类型
		assert.Contains(t, health, "open_connections") // 应该包含打开的连接数
		assert.Contains(t, health, "in_use")           // 应该包含正在使用的连接数
		assert.Contains(t, health, "idle")             // 应该包含空闲的连接数

		// 验证数据库类型报告正确
		if td.Config.Type == "postgres" {
			assert.Equal(t, "postgres", health["type"]) // PostgreSQL 数据库应该报告 "postgres"
		} else {
			assert.Equal(t, "sqlite", health["type"]) // SQLite 数据库应该报告 "sqlite"
		}
	})
}

// TestUserManagement 测试用户管理功能
// 该函数验证用户的创建、获取、密码验证和密码更新等功能
// 测试覆盖了正常和异常情况，确保用户管理功能的正确性
// 测试在 SQLite 和 PostgreSQL 两种数据库上运行
func TestUserManagement(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// 创建测试用户
		user, err := td.Service.CreateUser(ctx, "testuser", "testpass123")
		require.NoError(t, err)                    // 创建用户不应出错
		require.NotNil(t, user)                    // 用户对象不应为 nil
		assert.Equal(t, "testuser", user.Username) // 用户名应该匹配
		assert.NotEmpty(t, user.PasswordHash)      // 密码哈希不应为空
		assert.NotZero(t, user.ID)                 // 用户 ID 不应为 0

		// 通过用户名获取用户
		retrieved, err := td.Service.GetUserByUsername(ctx, "testuser")
		require.NoError(t, err)                            // 获取用户不应出错
		require.NotNil(t, retrieved)                       // 用户对象不应为 nil
		assert.Equal(t, user.ID, retrieved.ID)             // 用户 ID 应该匹配
		assert.Equal(t, user.Username, retrieved.Username) // 用户名应该匹配

		// 验证密码
		assert.True(t, td.Service.ValidatePassword(retrieved, "testpass123")) // 正确密码应该验证通过
		assert.False(t, td.Service.ValidatePassword(retrieved, "wrongpass"))  // 错误密码应该验证失败

		// 更新密码
		err = td.Service.UpdatePassword(ctx, "testuser", "newpass456")
		require.NoError(t, err) // 更新密码不应出错

		// 验证新密码
		updated, err := td.Service.GetUserByUsername(ctx, "testuser")
		require.NoError(t, err)                                              // 获取用户不应出错
		assert.True(t, td.Service.ValidatePassword(updated, "newpass456"))   // 新密码应该验证通过
		assert.False(t, td.Service.ValidatePassword(updated, "testpass123")) // 旧密码应该验证失败

		// 测试重复用户名
		_, err = td.Service.CreateUser(ctx, "testuser", "anotherpass")
		assert.Error(t, err) // 创建重复用户名应该出错

		// 测试不存在的用户
		_, err = td.Service.GetUserByUsername(ctx, "nonexistent")
		assert.Error(t, err) // 获取不存在的用户应该出错

		// 测试为不存在的用户更新密码
		err = td.Service.UpdatePassword(ctx, "nonexistent", "newpass")
		assert.Error(t, err) // 为不存在的用户更新密码应该出错
	})
}

// TestTransactionBehavior 测试数据库事务行为
// 该函数验证数据库操作在事务内的隔离性和一致性
// 测试创建用户和监视器，并验证它们的存在性
// 测试在 SQLite 和 PostgreSQL 两种数据库上运行
func TestTransactionBehavior(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// 测试操作在事务内的隔离性
		// 创建一个测试用户
		user, err := td.Service.CreateUser(ctx, "tx_test_user", "password")
		require.NoError(t, err) // 创建用户不应出错

		// 创建一个引用该用户的丢包监视器（通过上下文关联）
		monitor := CreateTestPacketLossMonitor(t, td)

		// 验证用户和监视器都存在
		_, err = td.Service.GetUserByUsername(ctx, "tx_test_user")
		require.NoError(t, err) // 获取用户不应出错

		_, err = td.Service.GetPacketLossMonitor(monitor.ID)
		require.NoError(t, err) // 获取监视器不应出错

		// 在实际的事务场景中，如果一个操作失败，所有操作都应该回滚
		// 这个测试验证了基本的一致性
		AssertRecordExists(t, td, "users", "id", user.ID)                   // 验证用户记录存在
		AssertRecordExists(t, td, "packet_loss_monitors", "id", monitor.ID) // 验证监视器记录存在
	})
}

// TestDatabaseTimeout 测试数据库超时处理功能
// 该函数验证当上下文超时被触发时，数据库操作是否会正确失败
// 测试通过创建一个极短的超时上下文，然后尝试执行数据库操作
// 测试在 SQLite 和 PostgreSQL 两种数据库上运行
func TestDatabaseTimeout(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建一个非常短的超时上下文（1纳秒）
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel() // 确保资源被释放

		// 睡眠10毫秒，确保上下文已经被取消
		time.Sleep(10 * time.Millisecond)

		// 尝试使用已取消的上下文执行数据库操作（创建用户）
		_, err := td.Service.CreateUser(ctx, "timeout_user", "password")
		assert.Error(t, err) // 操作应该因为超时而失败

		// 验证操作确实没有成功执行
		ctx2 := context.Background() // 创建一个新的正常上下文
		_, err = td.Service.GetUserByUsername(ctx2, "timeout_user")
		assert.Error(t, err) // 应该无法找到刚才尝试创建的用户
	})
}

// TestQueryRowMethod 测试 QueryRow 方法
// 该函数验证直接使用 QueryRow 方法执行 SQL 查询的功能
// 测试执行一个简单的计数查询，获取用户表中的记录数
// 测试在 SQLite 和 PostgreSQL 两种数据库上运行
func TestQueryRowMethod(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// 直接测试 QueryRow 方法
		var count int // 存储查询结果
		// 执行 SQL 查询，获取用户表中的记录数
		row := td.Service.QueryRow(ctx, "SELECT COUNT(*) FROM users")
		// 将查询结果扫描到变量中
		err := row.Scan(&count)
		require.NoError(t, err) // 查询不应出错
		// 验证记录数大于等于 0
		assert.GreaterOrEqual(t, count, 0)
	})
}

// TestDatabaseConstraints 测试数据库约束验证
// 该函数验证数据库的外键约束和唯一约束是否正常工作
// 测试包括尝试创建引用不存在资源的记录，以及尝试创建重复的唯一记录
// 测试在 SQLite 和 PostgreSQL 两种数据库上运行
func TestDatabaseConstraints(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 测试外键约束
		// 创建一个引用不存在监视器的丢包结果
		result := &types.PacketLossResult{
			MonitorID:  99999,      // 不存在的监视器 ID
			PacketLoss: 5.0,        // 丢包率
			CreatedAt:  time.Now(), // 创建时间
		}

		// 尝试保存丢包结果
		err := td.Service.SavePacketLossResult(result)
		assert.Error(t, err) // 应该因为外键约束失败

		// 测试唯一约束
		ctx := context.Background()
		// 创建一个用户
		_, err = td.Service.CreateUser(ctx, "unique_user", "password")
		require.NoError(t, err) // 创建用户不应出错

		// 尝试创建另一个具有相同用户名的用户
		_, err = td.Service.CreateUser(ctx, "unique_user", "different_password")
		assert.Error(t, err) // 应该因为唯一约束失败
	})
}

// TestLargeDataHandling 测试数据库大数据处理能力
// 该函数验证数据库是否能够正确保存和检索包含大量数据的记录
// 测试通过创建一个包含100个网络节点的大型MTR数据字符串，然后保存和检索
// 测试在 SQLite 和 PostgreSQL 两种数据库上运行
func TestLargeDataHandling(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建一个测试用的丢包监视器
		monitor := CreateTestPacketLossMonitor(t, td)

		// 创建一个大型的 MTR 数据字符串（包含100个网络节点）
		largeMTRData := `{
			"hops": [`

		// 生成100个网络节点数据
		for i := 0; i < 100; i++ {
			if i > 0 {
				largeMTRData += "," // 如果不是第一个节点，添加逗号分隔
			}
			// 添加节点数据，地址为10.0.0.x，x从0到9循环
			largeMTRData += `{"addr": "10.0.0.` + string(rune('0'+i%10)) + `", "loss": 0, "avg": 1.5}`
		}
		// 关闭 JSON 对象
		largeMTRData += `]}`

		// 保存包含大型数据的丢包结果
		result := &types.PacketLossResult{
			MonitorID:  monitor.ID,    // 关联到刚创建的监视器
			PacketLoss: 0.0,           // 丢包率为0%
			MTRData:    &largeMTRData, // 大型 MTR 数据
			UsedMTR:    true,          // 使用了 MTR 功能
			HopCount:   100,           // 跳数为100
			CreatedAt:  time.Now(),    // 创建时间
		}

		// 保存丢包结果到数据库
		err := td.Service.SavePacketLossResult(result)
		require.NoError(t, err) // 保存不应出错

		// 检索并验证结果
		latest, err := td.Service.GetLatestPacketLossResult(monitor.ID)
		require.NoError(t, err)           // 检索不应出错
		require.NotNil(t, latest.MTRData) // MTR 数据不应为 nil
		// 验证 MTR 数据中包含预期的地址信息
		assert.Contains(t, *latest.MTRData, "10.0.0")
		// 验证跳数是否正确
		assert.Equal(t, 100, latest.HopCount)
	})
}
