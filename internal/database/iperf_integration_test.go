// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later

package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIperfServer_CRUD 测试 Iperf 服务器的完整 CRUD 操作
// 使用 RunTestWithBothDatabases 在两种数据库(SQLite 和 PostgreSQL)上运行测试
func TestIperfServer_CRUD(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background() // 创建上下文用于数据库操作

		// 创建 Iperf 服务器
		server, err := td.Service.SaveIperfServer(ctx, "测试服务器", "iperf.example.com", 5201)
		require.NoError(t, err)                           // 确保创建操作没有错误
		require.NotNil(t, server)                         // 确保返回的服务器对象不为 nil
		assert.Greater(t, server.ID, 0)                   // 确保服务器 ID 大于 0
		assert.Equal(t, "测试服务器", server.Name)             // 验证服务器名称
		assert.Equal(t, "iperf.example.com", server.Host) // 验证服务器主机
		assert.Equal(t, 5201, server.Port)                // 验证服务器端口
		assert.NotZero(t, server.CreatedAt)               // 验证创建时间不为零

		// 获取所有服务器
		servers, err := td.Service.GetIperfServers(ctx)
		require.NoError(t, err)                       // 确保查询操作没有错误
		assert.Len(t, servers, 1)                     // 确保只有一个服务器
		assert.Equal(t, server.ID, servers[0].ID)     // 验证返回的服务器 ID 与创建的一致
		assert.Equal(t, server.Name, servers[0].Name) // 验证返回的服务器名称与创建的一致

		// 删除服务器
		err = td.Service.DeleteIperfServer(ctx, server.ID)
		require.NoError(t, err) // 确保删除操作没有错误

		// 验证服务器已删除
		servers, err = td.Service.GetIperfServers(ctx)
		require.NoError(t, err)   // 确保查询操作没有错误
		assert.Len(t, servers, 0) // 确保服务器列表为空
	})
}

// TestIperfServer_MultipleServers 测试管理多个 Iperf 服务器
// 验证可以创建多个服务器并正确管理它们
func TestIperfServer_MultipleServers(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background() // 创建上下文用于数据库操作

		// 定义要创建的多个服务器数据
		serverData := []struct {
			name string // 服务器名称
			host string // 服务器主机
			port int    // 服务器端口
		}{
			{"美国东部", "us-east.iperf.com", 5201},
			{"美国西部", "us-west.iperf.com", 5202},
			{"欧盟中部", "eu-central.iperf.com", 5203},
			{"亚太地区", "asia-pacific.iperf.com", 5204},
		}

		// 保存创建的服务器 ID 列表
		var createdIDs []int
		for _, data := range serverData {
			// 创建每个服务器
			server, err := td.Service.SaveIperfServer(ctx, data.name, data.host, data.port)
			require.NoError(t, err)                    // 确保创建操作没有错误
			createdIDs = append(createdIDs, server.ID) // 保存创建的服务器 ID
		}

		// 获取所有服务器
		servers, err := td.Service.GetIperfServers(ctx)
		require.NoError(t, err)                 // 确保查询操作没有错误
		assert.Len(t, servers, len(serverData)) // 验证服务器数量与创建的一致

		// 验证所有创建的服务器都存在
		foundServers := make(map[string]bool)
		for _, server := range servers {
			foundServers[server.Name] = true // 将找到的服务器名称添加到映射中
		}

		// 检查每个创建的服务器是否都在查询结果中
		for _, data := range serverData {
			assert.True(t, foundServers[data.name],
				"服务器 %s 应该存在", data.name)
		}

		// 删除特定服务器（第二个创建的服务器）
		err = td.Service.DeleteIperfServer(ctx, createdIDs[1])
		require.NoError(t, err) // 确保删除操作没有错误

		// 验证只有3个服务器剩余
		servers, err = td.Service.GetIperfServers(ctx)
		require.NoError(t, err)   // 确保查询操作没有错误
		assert.Len(t, servers, 3) // 验证服务器数量为3

		// 验证被删除的服务器不在结果中
		for _, server := range servers {
			assert.NotEqual(t, "美国西部", server.Name) // 确保"美国西部"服务器已被删除
		}
	})
}

// TestIperfServer_DuplicateHost 测试同一主机上创建多个不同端口的 Iperf 服务器
// 验证可以在同一主机上创建多个服务器，只要它们的端口不同
func TestIperfServer_DuplicateHost(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background() // 创建上下文用于数据库操作

		// 创建第一个服务器
		server1, err := td.Service.SaveIperfServer(ctx, "服务器 1", "duplicate.iperf.com", 5201)
		require.NoError(t, err)    // 确保创建操作没有错误
		require.NotNil(t, server1) // 确保返回的服务器对象不为 nil

		// 尝试在同一主机上创建第二个服务器
		// 由于端口不同，这个操作应该成功
		server2, err := td.Service.SaveIperfServer(ctx, "服务器 2", "duplicate.iperf.com", 5202)
		require.NoError(t, err)    // 确保创建操作没有错误
		require.NotNil(t, server2) // 确保返回的服务器对象不为 nil

		// 验证两个服务器都存在
		servers, err := td.Service.GetIperfServers(ctx)
		require.NoError(t, err)   // 确保查询操作没有错误
		assert.Len(t, servers, 2) // 验证服务器数量为 2
	})
}

// TestIperfServer_PortRange 测试不同端口号的 Iperf 服务器创建
// 验证可以使用各种端口号创建服务器，包括低端口、标准端口、高端口和自定义端口
func TestIperfServer_PortRange(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background() // 创建上下文用于数据库操作

		// 定义要测试的不同端口号
		portTests := []struct {
			name string // 测试名称
			port int    // 端口号
		}{
			{"低端口", 1024},         // 最低的可用端口
			{"标准 iPerf 端口", 5201}, // iPerf 默认端口
			{"高端口", 65535},        // 最大的 TCP 端口号
			{"自定义端口", 12345},      // 常用的自定义端口
		}

		// 使用不同端口创建服务器
		for _, test := range portTests {
			server, err := td.Service.SaveIperfServer(ctx, test.name, "port-test.iperf.com", test.port)
			require.NoError(t, err)                 // 确保创建操作没有错误
			assert.Equal(t, test.port, server.Port) // 验证端口号正确保存
		}

		// 验证所有服务器都已保存
		servers, err := td.Service.GetIperfServers(ctx)
		require.NoError(t, err)                // 确保查询操作没有错误
		assert.Len(t, servers, len(portTests)) // 验证服务器数量与测试用例数量一致
	})
}

// TestIperfServer_DeleteNonExistent 测试删除不存在的 Iperf 服务器
// 验证删除不存在的服务器不会导致程序崩溃，不同数据库对这种情况可能有不同的处理方式
func TestIperfServer_DeleteNonExistent(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background() // 创建上下文用于数据库操作

		// 尝试删除不存在的服务器（使用一个很大的ID）
		err := td.Service.DeleteIperfServer(ctx, 99999)
		// 有些数据库对不存在记录的DELETE操作不会返回错误
		// 所以这里只确保程序不会崩溃，不检查错误
		_ = err
	})
}

// TestIperfServer_EmptyList 测试当没有服务器时获取服务器列表
// 验证在没有创建任何服务器的情况下，GetIperfServers 方法返回空列表而不是 nil
func TestIperfServer_EmptyList(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background() // 创建上下文用于数据库操作

		// 当没有服务器存在时获取服务器列表
		servers, err := td.Service.GetIperfServers(ctx)
		require.NoError(t, err)   // 确保查询操作没有错误
		assert.NotNil(t, servers) // 确保返回的列表不为 nil
		assert.Len(t, servers, 0) // 确保返回的列表长度为 0
	})
}

// TestIperfServer_SpecialCharacters 测试包含特殊字符的 Iperf 服务器名称和主机
// 验证可以使用包含空格、破折号、下划线、点号、括号和非英语字符的名称和主机
func TestIperfServer_SpecialCharacters(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background() // 创建上下文用于数据库操作

		// 定义包含特殊字符的测试用例
		specialTests := []struct {
			name string // 服务器名称（包含特殊字符）
			host string // 服务器主机（包含特殊字符）
		}{
			{"带空格的服务器", "host with spaces.com"},
			{"带破折号的服务器", "host-with-dashes.com"},
			{"带下划线的服务器", "host_with_underscores.com"},
			{"带点号的服务器", "192.168.1.100"},
			{"带括号的服务器", "host.example.com"},
			{"日本語サーバー", "japan.iperf.com"},
		}

		// 使用特殊字符创建服务器
		for _, test := range specialTests {
			server, err := td.Service.SaveIperfServer(ctx, test.name, test.host, 5201)
			require.NoError(t, err)                 // 确保创建操作没有错误
			assert.Equal(t, test.name, server.Name) // 验证名称正确保存
			assert.Equal(t, test.host, server.Host) // 验证主机正确保存
		}

		// 验证所有服务器都已正确保存
		servers, err := td.Service.GetIperfServers(ctx)
		require.NoError(t, err)                   // 确保查询操作没有错误
		assert.Len(t, servers, len(specialTests)) // 验证服务器数量与测试用例数量一致

		// 验证名称被正确保留
		foundNames := make(map[string]bool)
		for _, server := range servers {
			foundNames[server.Name] = true // 将找到的服务器名称添加到映射中
		}

		for _, test := range specialTests {
			assert.True(t, foundNames[test.name],
				"服务器名称 %s 应该存在", test.name)
		}
	})
}
