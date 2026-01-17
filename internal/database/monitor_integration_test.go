// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later

// database 包的集成测试文件
// 本文件包含了监控代理相关功能的集成测试，测试包括：
// - 代理的增删改查操作
// - 系统信息管理
// - 网络接口管理
// - 峰值统计管理
// - 资源使用统计
// - 历史快照管理
// - 数据清理功能
// - Tailscale 字段支持
package database

import (
	"context" // 上下文管理，用于控制测试请求的生命周期
	"testing" // Go语言测试框架
	"time"    // 时间处理

	"github.com/stretchr/testify/assert"  // 断言库，用于验证测试结果
	"github.com/stretchr/testify/require" // 断言库，用于验证关键条件，失败时终止测试

	"github.com/autobrr/netronome/internal/types" // 定义了所有数据结构
)

// boolPtr 布尔指针的辅助函数
// 用于将布尔值转换为布尔指针
// 参数:
//
//	b: 要转换的布尔值
//
// 返回值:
//
//	指向布尔值的指针
func boolPtr(b bool) *bool {
	return &b
}

// timePtr 时间指针的辅助函数
// 用于将时间值转换为时间指针
// 参数:
//
//	t: 要转换的时间值
//
// 返回值:
//
//	指向时间值的指针
func timePtr(t time.Time) *time.Time {
	return &t
}

// TestMonitorAgent_CRUD 测试监控代理的增删改查功能
// 此测试使用 RunTestWithBothDatabases 函数，在 SQLite 和 PostgreSQL 两种数据库上运行相同的测试逻辑
// 测试步骤：
// 1. 创建一个新的监控代理
// 2. 通过ID获取刚创建的代理
// 3. 更新代理信息
// 4. 验证更新后的信息
// 5. 删除代理
// 6. 验证代理已被删除
func TestMonitorAgent_CRUD(t *testing.T) {
	// 使用 RunTestWithBothDatabases 函数在两种数据库上运行测试
	// 这是一个测试辅助函数，用于确保代码在不同数据库上的兼容性
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建上下文，用于控制测试请求的生命周期
		ctx := context.Background()

		// 创建代理
		// 创建一个测试用的监控代理结构体，包含名称、URL、API密钥和启用状态
		agent := &types.MonitorAgent{
			Name:    "Test Agent",                    // 代理名称
			URL:     "http://agent.example.com:8080", // 代理URL
			APIKey:  stringPtr("test-api-key-123"),   // 代理API密钥（指针类型）
			Enabled: true,                            // 启用代理
		}

		// 调用CreateMonitorAgent方法创建代理
		created, err := td.Service.CreateMonitorAgent(ctx, agent)
		// 验证创建操作没有错误
		require.NoError(t, err)
		// 验证返回的代理对象不为nil
		require.NotNil(t, created)
		// 验证代理ID大于0（表示创建成功并分配了ID）
		assert.Greater(t, created.ID, int64(0))
		// 验证代理名称与输入一致
		assert.Equal(t, agent.Name, created.Name)
		// 验证代理URL与输入一致
		assert.Equal(t, agent.URL, created.URL)
		// 验证创建时间不为零值
		assert.NotZero(t, created.CreatedAt)
		// 验证更新时间不为零值
		assert.NotZero(t, created.UpdatedAt)

		// 通过ID获取代理
		// 使用刚创建的代理ID调用GetMonitorAgent方法获取代理信息
		retrieved, err := td.Service.GetMonitorAgent(ctx, created.ID)
		// 验证获取操作没有错误
		require.NoError(t, err)
		// 验证返回的代理对象不为nil
		require.NotNil(t, retrieved)
		// 验证返回的代理ID与创建时的ID一致
		assert.Equal(t, created.ID, retrieved.ID)
		// 验证返回的代理名称与创建时的名称一致
		assert.Equal(t, created.Name, retrieved.Name)

		// 更新代理
		// 修改代理的名称和启用状态
		retrieved.Name = "Updated Agent" // 更新代理名称
		retrieved.Enabled = false        // 禁用代理
		// 调用UpdateMonitorAgent方法更新代理信息
		err = td.Service.UpdateMonitorAgent(ctx, retrieved)
		// 验证更新操作没有错误
		require.NoError(t, err)

		// 验证更新
		// 获取更新后的代理信息
		updated, err := td.Service.GetMonitorAgent(ctx, retrieved.ID)
		// 验证获取操作没有错误
		require.NoError(t, err)
		// 验证代理名称已更新为新值
		assert.Equal(t, "Updated Agent", updated.Name)
		// 验证代理已被禁用
		assert.False(t, updated.Enabled)

		// 删除代理
		// 使用创建时的代理ID调用DeleteMonitorAgent方法删除代理
		err = td.Service.DeleteMonitorAgent(ctx, created.ID)
		// 验证删除操作没有错误
		require.NoError(t, err)

		// 验证删除
		// 尝试获取已删除的代理
		deleted, err := td.Service.GetMonitorAgent(ctx, created.ID)
		// 验证获取操作返回错误（表示代理不存在）
		assert.Error(t, err)
		// 验证返回的代理对象为nil
		assert.Nil(t, deleted)
	})
}

// TestMonitorAgent_GetEnabledOnly 测试只获取启用的代理
// 此测试验证 GetMonitorAgents 方法的 enabledOnly 参数是否正确工作
// 测试步骤：
// 1. 创建多个代理，包括启用和禁用的
// 2. 获取所有代理（不区分启用状态）
// 3. 只获取启用的代理
// 4. 验证返回的启用代理数量和状态
func TestMonitorAgent_GetEnabledOnly(t *testing.T) {
	// 使用 RunTestWithBothDatabases 函数在两种数据库上运行测试
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建上下文，用于控制测试请求的生命周期
		ctx := context.Background()

		// 创建多个代理
		// 创建一个包含3个代理的切片，其中2个启用，1个禁用
		agents := []*types.MonitorAgent{
			{
				Name:    "Enabled Agent 1", // 启用的代理1
				URL:     "http://agent1.example.com",
				APIKey:  stringPtr("key1"),
				Enabled: true, // 启用
			},
			{
				Name:    "Disabled Agent", // 禁用的代理
				URL:     "http://agent2.example.com",
				APIKey:  stringPtr("key2"),
				Enabled: false, // 禁用
			},
			{
				Name:    "Enabled Agent 2", // 启用的代理2
				URL:     "http://agent3.example.com",
				APIKey:  stringPtr("key3"),
				Enabled: true, // 启用
			},
		}

		// 遍历切片，创建所有代理
		for _, agent := range agents {
			_, err := td.Service.CreateMonitorAgent(ctx, agent)
			// 验证创建操作没有错误
			require.NoError(t, err)
		}

		// 获取所有代理
		// 调用GetMonitorAgents方法，enabledOnly参数为false表示获取所有代理
		allAgents, err := td.Service.GetMonitorAgents(ctx, false)
		// 验证获取操作没有错误
		require.NoError(t, err)
		// 验证返回的代理数量为3（所有创建的代理）
		assert.Len(t, allAgents, 3)

		// 只获取启用的代理
		// 调用GetMonitorAgents方法，enabledOnly参数为true表示只获取启用的代理
		enabledAgents, err := td.Service.GetMonitorAgents(ctx, true)
		// 验证获取操作没有错误
		require.NoError(t, err)
		// 验证返回的代理数量为2（只有启用的代理）
		assert.Len(t, enabledAgents, 2)

		// 验证所有返回的代理都是启用的
		// 遍历返回的启用代理切片，验证每个代理的Enabled字段都是true
		for _, agent := range enabledAgents {
			assert.True(t, agent.Enabled)
		}
	})
}

// TestMonitorAgent_SystemInfo 测试代理系统信息的管理功能
// 此测试验证代理系统信息的插入、获取和更新功能
// 测试步骤：
// 1. 创建一个监控代理
// 2. 创建系统信息结构体
// 3. 使用UpsertMonitorSystemInfo方法插入系统信息
// 4. 获取刚插入的系统信息
// 5. 更新系统信息
// 6. 验证更新后的系统信息
func TestMonitorAgent_SystemInfo(t *testing.T) {
	// 使用 RunTestWithBothDatabases 函数在两种数据库上运行测试
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建上下文，用于控制测试请求的生命周期
		ctx := context.Background()

		// 创建代理
		// 创建一个测试用的监控代理
		agent := &types.MonitorAgent{
			Name:    "Info Test Agent",          // 代理名称
			URL:     "http://agent.example.com", // 代理URL
			APIKey:  stringPtr("test-key"),      // 代理API密钥
			Enabled: true,                       // 启用代理
		}

		// 创建代理
		created, err := td.Service.CreateMonitorAgent(ctx, agent)
		// 验证创建操作没有错误
		require.NoError(t, err)

		// 创建系统信息
		// 定义代理版本字符串，用于后续系统信息结构体
		agentVersion := "1.0.0"
		// 创建系统信息结构体，包含主机名、内核版本、vnstat版本、代理版本等信息
		sysInfo := &types.MonitorSystemInfo{
			Hostname:      "test-host",           // 主机名
			Kernel:        "Linux 5.15.0 x86_64", // 内核版本
			VnstatVersion: "2.10",                // vnstat版本
			AgentVersion:  &agentVersion,         // 代理版本（指针类型）
			CPUModel:      "Intel Core i7",       // CPU型号
			CPUCores:      4,                     // CPU核心数
			CPUThreads:    8,                     // CPU线程数
			TotalMemory:   16000000000,           // 总内存（字节）
		}

		// 插入或更新系统信息
		// 使用UpsertMonitorSystemInfo方法为代理插入系统信息
		err = td.Service.UpsertMonitorSystemInfo(ctx, created.ID, sysInfo)
		// 验证插入操作没有错误
		require.NoError(t, err)

		// 获取系统信息
		// 使用GetMonitorSystemInfo方法获取代理的系统信息
		retrieved, err := td.Service.GetMonitorSystemInfo(ctx, created.ID)
		// 验证获取操作没有错误
		require.NoError(t, err)
		// 验证返回的系统信息不为nil
		require.NotNil(t, retrieved)
		// 验证返回的主机名与插入时的主机名一致
		assert.Equal(t, sysInfo.Hostname, retrieved.Hostname)
		// 验证返回的内核版本与插入时的内核版本一致
		assert.Equal(t, sysInfo.Kernel, retrieved.Kernel)

		// 更新系统信息
		// 修改系统信息的内核版本
		sysInfo.Kernel = "Linux 5.16.0 x86_64"
		// 使用UpsertMonitorSystemInfo方法更新系统信息
		err = td.Service.UpsertMonitorSystemInfo(ctx, created.ID, sysInfo)
		// 验证更新操作没有错误
		require.NoError(t, err)

		// 验证更新
		// 获取更新后的系统信息
		updated, err := td.Service.GetMonitorSystemInfo(ctx, created.ID)
		// 验证获取操作没有错误
		require.NoError(t, err)
		// 验证内核版本已更新为新值
		assert.Equal(t, "Linux 5.16.0 x86_64", updated.Kernel)
	})
}

// TestMonitorAgent_Interfaces 测试代理网络接口的管理功能
// 此测试验证代理网络接口的插入、获取和更新功能
// 测试步骤：
// 1. 创建一个监控代理
// 2. 创建网络接口切片，包含以太网和回环接口
// 3. 使用UpsertMonitorInterfaces方法插入网络接口
// 4. 获取刚插入的网络接口
// 5. 验证获取的接口数量和名称
// 6. 更新网络接口，添加WiFi接口
// 7. 验证更新后的接口数量
func TestMonitorAgent_Interfaces(t *testing.T) {
	// 使用 RunTestWithBothDatabases 函数在两种数据库上运行测试
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建上下文，用于控制测试请求的生命周期
		ctx := context.Background()

		// 创建代理
		// 创建一个测试用的监控代理
		agent := &types.MonitorAgent{
			Name:    "Interface Test Agent",     // 代理名称
			URL:     "http://agent.example.com", // 代理URL
			APIKey:  stringPtr("test-key"),      // 代理API密钥
			Enabled: true,                       // 启用代理
		}

		// 创建代理
		created, err := td.Service.CreateMonitorAgent(ctx, agent)
		// 验证创建操作没有错误
		require.NoError(t, err)

		// 创建网络接口
		// 创建一个网络接口切片，包含以太网和回环接口
		interfaces := []types.MonitorInterface{
			{
				Name:      "eth0",          // 接口名称：以太网接口
				Alias:     "Ethernet",      // 接口别名
				IPAddress: "192.168.1.100", // IP地址
				LinkSpeed: 1000,            // 链接速度（Mbps）
			},
			{
				Name:      "lo",        // 接口名称：回环接口
				Alias:     "Loopback",  // 接口别名
				IPAddress: "127.0.0.1", // IP地址
				LinkSpeed: 0,           // 链接速度（回环接口通常为0）
			},
		}

		// 插入或更新网络接口
		// 使用UpsertMonitorInterfaces方法为代理插入网络接口
		// 注意：UpsertMonitorInterfaces方法会先删除代理的所有现有接口，然后插入新的接口
		err = td.Service.UpsertMonitorInterfaces(ctx, created.ID, interfaces)
		// 验证插入操作没有错误
		require.NoError(t, err)

		// 获取网络接口
		// 使用GetMonitorInterfaces方法获取代理的网络接口
		retrieved, err := td.Service.GetMonitorInterfaces(ctx, created.ID)
		// 验证获取操作没有错误
		require.NoError(t, err)
		// 验证返回的接口数量为2（与插入的数量一致）
		assert.Len(t, retrieved, 2)

		// 验证接口数据
		// 创建一个映射来存储找到的接口名称
		foundInterfaces := make(map[string]bool)
		// 遍历返回的接口切片，将接口名称添加到映射中
		for _, iface := range retrieved {
			foundInterfaces[iface.Name] = true
		}
		// 验证eth0和lo接口都被找到了
		assert.True(t, foundInterfaces["eth0"])
		assert.True(t, foundInterfaces["lo"])

		// 更新接口 - 添加更多接口
		// 向接口切片添加一个WiFi接口
		interfaces = append(interfaces, types.MonitorInterface{
			Name:      "wlan0",         // 接口名称：WiFi接口
			Alias:     "WiFi",          // 接口别名
			IPAddress: "192.168.1.101", // IP地址
			LinkSpeed: 300,             // 链接速度（Mbps）
		})
		// 使用UpsertMonitorInterfaces方法更新网络接口
		err = td.Service.UpsertMonitorInterfaces(ctx, created.ID, interfaces)
		// 验证更新操作没有错误
		require.NoError(t, err)

		// 验证更新
		// 获取更新后的网络接口
		updated, err := td.Service.GetMonitorInterfaces(ctx, created.ID)
		// 验证获取操作没有错误
		require.NoError(t, err)
		// 验证返回的接口数量为3（包含新添加的WiFi接口）
		assert.Len(t, updated, 3)
	})
}

// TestMonitorAgent_PeakStats 测试代理峰值统计的管理功能
// 此测试验证代理峰值统计数据的插入、获取和更新功能
// 测试步骤：
// 1. 创建一个监控代理
// 2. 创建峰值统计结构体，包含峰值接收和发送字节数
// 3. 使用UpsertMonitorPeakStats方法插入峰值统计
// 4. 获取刚插入的峰值统计
// 5. 更新峰值统计（增加接收字节数）
// 6. 验证更新后的峰值统计
func TestMonitorAgent_PeakStats(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建上下文，用于控制测试请求的生命周期
		ctx := context.Background()

		// 创建代理
		agent := &types.MonitorAgent{
			Name:    "Peak Stats Test Agent",
			URL:     "http://agent.example.com",
			APIKey:  stringPtr("test-key"),
			Enabled: true,
		}

		created, err := td.Service.CreateMonitorAgent(ctx, agent)
		require.NoError(t, err)

		// 创建峰值统计
		now := time.Now()
		peakStats := &types.MonitorPeakStats{
			PeakRxBytes:     1000000000,
			PeakTxBytes:     500000000,
			PeakRxTimestamp: &now,
			PeakTxTimestamp: &now,
		}

		// 插入或更新峰值统计
		err = td.Service.UpsertMonitorPeakStats(ctx, created.ID, peakStats)
		require.NoError(t, err)

		// 获取峰值统计
		retrieved, err := td.Service.GetMonitorPeakStats(ctx, created.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, peakStats.PeakRxBytes, retrieved.PeakRxBytes)
		assert.Equal(t, peakStats.PeakTxBytes, retrieved.PeakTxBytes)

		// 更新峰值统计
		peakStats.PeakRxBytes = 2000000000
		err = td.Service.UpsertMonitorPeakStats(ctx, created.ID, peakStats)
		require.NoError(t, err)

		// 验证更新
		updated, err := td.Service.GetMonitorPeakStats(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(2000000000), updated.PeakRxBytes)
	})
}

// TestMonitorAgent_ResourceStats 测试代理资源统计的管理功能
// 此测试验证代理资源统计数据的保存和获取功能
// 测试步骤：
// 1. 创建一个监控代理
// 2. 循环创建并保存5个资源统计数据（CPU使用率、内存使用率等）
// 3. 获取过去24小时的资源统计数据
// 4. 验证返回的统计数据数量为5
// 5. 验证统计数据的时间顺序（降序排列）
func TestMonitorAgent_ResourceStats(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建上下文，用于控制测试请求的生命周期
		ctx := context.Background()

		// 创建代理
		agent := &types.MonitorAgent{
			Name:    "Resource Test Agent",
			URL:     "http://agent.example.com",
			APIKey:  stringPtr("test-key"),
			Enabled: true,
		}

		created, err := td.Service.CreateMonitorAgent(ctx, agent)
		require.NoError(t, err)

		// 保存多个资源统计数据
		for i := 0; i < 5; i++ {
			stats := &types.MonitorResourceStats{
				CPUUsagePercent:   float64(20 + i*5),
				MemoryUsedPercent: float64(40 + i*2),
				SwapUsedPercent:   float64(10 + i),
				DiskUsageJSON:     `{"disks":[{"path":"/","used":60,"total":100}]}`,
				TemperatureJSON:   `{"temps":[{"sensor":"cpu","temp":45}]}`,
				UptimeSeconds:     int64(3600 + i*100),
			}

			err = td.Service.SaveMonitorResourceStats(ctx, created.ID, stats)
			require.NoError(t, err)

			time.Sleep(10 * time.Millisecond)
		}

		// 获取过去24小时的统计数据以避免时区问题
		stats, err := td.Service.GetMonitorResourceStats(ctx, created.ID, 24)
		require.NoError(t, err)

		assert.Len(t, stats, 5)

		// 验证顺序和值
		for i := 1; i < len(stats); i++ {
			assert.True(t, stats[i-1].CreatedAt.After(stats[i].CreatedAt) ||
				stats[i-1].CreatedAt.Equal(stats[i].CreatedAt))
		}
	})
}

// TestMonitorAgent_HistoricalSnapshot 测试代理历史快照的管理功能
// 此测试验证代理历史快照数据的保存和获取功能
// 测试步骤：
// 1. 创建一个监控代理
// 2. 创建并保存每日历史快照
// 3. 获取最新的每日快照并验证
// 4. 创建并保存每月历史快照
// 5. 获取最新的每月快照并验证
func TestMonitorAgent_HistoricalSnapshot(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建上下文，用于控制测试请求的生命周期
		ctx := context.Background()

		// 创建代理
		agent := &types.MonitorAgent{
			Name:    "Snapshot Test Agent",
			URL:     "http://agent.example.com",
			APIKey:  stringPtr("test-key"),
			Enabled: true,
		}

		created, err := td.Service.CreateMonitorAgent(ctx, agent)
		require.NoError(t, err)

		// 保存每日快照
		dailySnapshot := &types.MonitorHistoricalSnapshot{
			InterfaceName: "eth0",
			PeriodType:    "daily",
			DataJSON: `{
				"rx_bytes": 1000000000,
				"tx_bytes": 500000000,
				"rx_packets": 1000000,
				"tx_packets": 500000,
				"avg_cpu_usage": 25.5,
				"avg_memory_usage": 45.0,
				"avg_temperature": 50.0,
				"peak_temperature": 65.0
			}`,
		}

		err = td.Service.SaveMonitorHistoricalSnapshot(ctx, created.ID, dailySnapshot)
		require.NoError(t, err)

		// 获取最新的每日快照
		retrieved, err := td.Service.GetMonitorLatestSnapshot(ctx, created.ID, "daily")
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, dailySnapshot.DataJSON, retrieved.DataJSON)
		assert.Equal(t, dailySnapshot.InterfaceName, retrieved.InterfaceName)

		// 保存每月快照
		monthlySnapshot := &types.MonitorHistoricalSnapshot{
			InterfaceName: "eth0",
			PeriodType:    "monthly",
			DataJSON: `{
				"rx_bytes": 30000000000,
				"tx_bytes": 15000000000,
				"rx_packets": 30000000,
				"tx_packets": 15000000,
				"avg_cpu_usage": 30.0,
				"avg_memory_usage": 50.0,
				"avg_temperature": 52.0,
				"peak_temperature": 70.0
			}`,
		}

		err = td.Service.SaveMonitorHistoricalSnapshot(ctx, created.ID, monthlySnapshot)
		require.NoError(t, err)

		// 获取最新的每月快照
		retrieved, err = td.Service.GetMonitorLatestSnapshot(ctx, created.ID, "monthly")
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, monthlySnapshot.DataJSON, retrieved.DataJSON)
	})
}

// TestMonitorAgent_CleanupData 测试代理数据清理功能
// 此测试验证代理数据清理功能的基本正确性
// 测试步骤：
// 1. 直接调用CleanupMonitorData方法
// 2. 验证清理操作没有错误
// 注意：此测试仅验证清理函数不会出错，不验证实际清理效果
// 在生产环境中，应创建旧数据并验证其是否被正确清理
func TestMonitorAgent_CleanupData(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建上下文，用于控制测试请求的生命周期
		ctx := context.Background()

		// 此测试验证清理函数不会出错
		// 在实际场景中，我们需要创建旧数据并验证其是否被清理
		err := td.Service.CleanupMonitorData(ctx)
		assert.NoError(t, err)
	})
}

// TestMonitorAgent_TailscaleFields 测试代理Tailscale字段的管理功能
// 此测试验证代理Tailscale相关字段的创建和获取功能
// 测试步骤：
// 1. 创建一个包含Tailscale字段的监控代理
// 2. 验证代理创建成功
// 3. 获取刚创建的代理
// 4. 验证Tailscale字段的正确性
func TestMonitorAgent_TailscaleFields(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建上下文，用于控制测试请求的生命周期
		ctx := context.Background()

		// 创建具有Tailscale字段的代理
		agent := &types.MonitorAgent{
			Name:              "Tailscale Agent",
			URL:               "http://agent.example.com",
			APIKey:            stringPtr("test-key"),
			Enabled:           true,
			IsTailscale:       true,
			TailscaleHostname: stringPtr("agent-ts"),
			Interface:         stringPtr("tailscale0"),
			DiscoveredAt:      timePtr(time.Now()),
		}

		created, err := td.Service.CreateMonitorAgent(ctx, agent)
		require.NoError(t, err)

		// 检索并验证Tailscale字段
		retrieved, err := td.Service.GetMonitorAgent(ctx, created.ID)
		require.NoError(t, err)
		assert.True(t, retrieved.IsTailscale)
		assert.NotNil(t, retrieved.TailscaleHostname)
		assert.Equal(t, *agent.TailscaleHostname, *retrieved.TailscaleHostname)
		assert.NotNil(t, retrieved.Interface)
		assert.Equal(t, *agent.Interface, *retrieved.Interface)
	})
}
