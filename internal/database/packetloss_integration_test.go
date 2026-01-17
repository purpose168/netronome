// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 此文件包含丢包监控系统的集成测试
// 测试范围包括丢包监控的CRUD操作、结果保存和查询等功能
//
// 所有测试都在SQLite和PostgreSQL两种数据库上运行，确保跨数据库兼容性
package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/netronome/internal/types"
)

// TestPacketLossMonitor_CRUD 测试丢包监控的CRUD操作
// 此测试验证丢包监控的创建、查询、更新和删除功能
// 测试步骤：
// 1. 创建一个新的丢包监控
// 2. 通过ID获取刚创建的监控
// 3. 更新监控信息
// 4. 验证更新后的监控信息
// 5. 删除监控
// 6. 验证监控已被删除
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestPacketLossMonitor_CRUD(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// Create
		monitor := &types.PacketLossMonitor{
			Name:        "Test Monitor",
			Host:        "8.8.8.8",
			Interval:    "60s",
			PacketCount: 10,
			Enabled:     true,
			Threshold:   5.0,
			LastState:   "healthy",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		created, err := td.Service.CreatePacketLossMonitor(monitor)
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Greater(t, created.ID, int64(0))
		assert.Equal(t, monitor.Name, created.Name)
		assert.Equal(t, monitor.Host, created.Host)

		// Read
		retrieved, err := td.Service.GetPacketLossMonitor(created.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, created.ID, retrieved.ID)
		assert.Equal(t, created.Name, retrieved.Name)

		// Update
		retrieved.Name = "Updated Monitor"
		retrieved.Enabled = false
		err = td.Service.UpdatePacketLossMonitor(retrieved)
		require.NoError(t, err)

		// Verify update
		updated, err := td.Service.GetPacketLossMonitor(retrieved.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Monitor", updated.Name)
		assert.False(t, updated.Enabled)

		// Delete
		err = td.Service.DeletePacketLossMonitor(created.ID)
		require.NoError(t, err)

		// Verify deletion
		deleted, err := td.Service.GetPacketLossMonitor(created.ID)
		assert.Error(t, err)
		assert.Nil(t, deleted)
	})
}

// TestGetEnabledPacketLossMonitors 测试获取所有启用的丢包监控
// 此测试验证获取所有启用的丢包监控的功能
// 测试步骤：
// 1. 创建多个丢包监控（包括启用和禁用的）
// 2. 获取所有启用的丢包监控
// 3. 验证返回的监控数量是否正确
// 4. 验证返回的所有监控都是启用状态
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestGetEnabledPacketLossMonitors(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// Create multiple monitors
		monitors := []types.PacketLossMonitor{
			{
				Name:        "Enabled Monitor 1",
				Host:        "8.8.8.8",
				Interval:    "30s",
				PacketCount: 5,
				Enabled:     true,
				Threshold:   10.0,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			{
				Name:        "Disabled Monitor",
				Host:        "1.1.1.1",
				Interval:    "60s",
				PacketCount: 10,
				Enabled:     false,
				Threshold:   5.0,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			{
				Name:        "Enabled Monitor 2",
				Host:        "4.4.4.4",
				Interval:    "120s",
				PacketCount: 20,
				Enabled:     true,
				Threshold:   15.0,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		}

		for _, m := range monitors {
			monitor := m // capture loop variable
			_, err := td.Service.CreatePacketLossMonitor(&monitor)
			require.NoError(t, err)
		}

		// Get only enabled monitors
		enabled, err := td.Service.GetEnabledPacketLossMonitors()
		require.NoError(t, err)
		assert.Len(t, enabled, 2)

		// Verify all returned monitors are enabled
		for _, m := range enabled {
			assert.True(t, m.Enabled)
		}
	})
}

// TestSavePacketLossResult 测试保存丢包测试结果
// 此测试验证保存丢包测试结果的功能
// 测试步骤：
// 1. 首先创建一个丢包监控
// 2. 创建一个丢包测试结果
// 3. 保存丢包测试结果
// 4. 验证结果保存成功
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSavePacketLossResult(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// Create a monitor first
		monitor := CreateTestPacketLossMonitor(t, td)

		// Save result
		result := &types.PacketLossResult{
			MonitorID:      monitor.ID,
			PacketLoss:     2.5,
			MinRTT:         10.1,
			MaxRTT:         50.5,
			AvgRTT:         25.3,
			StdDevRTT:      5.2,
			PacketsSent:    100,
			PacketsRecv:    97,
			UsedMTR:        true,
			HopCount:       10,
			MTRData:        stringPtr(`{"hops":[]}`),
			PrivilegedMode: true,
			CreatedAt:      time.Now(),
		}

		err := td.Service.SavePacketLossResult(result)
		require.NoError(t, err)
		assert.Greater(t, result.ID, int64(0))

		// Verify saved
		AssertRecordExists(t, td, "packet_loss_results", "id", result.ID)
	})
}

// TestGetLatestPacketLossResult 测试获取最新的丢包测试结果
// 此测试验证获取最新丢包测试结果的功能
// 测试步骤：
// 1. 首先创建一个丢包监控
// 2. 保存多个不同时间的丢包测试结果
// 3. 获取最新的丢包测试结果
// 4. 验证返回的结果是否是最新的
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestGetLatestPacketLossResult(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// Create a monitor
		monitor := CreateTestPacketLossMonitor(t, td)

		// Save multiple results
		results := []types.PacketLossResult{
			{
				MonitorID:   monitor.ID,
				PacketLoss:  1.0,
				AvgRTT:      20.0,
				PacketsSent: 100,
				PacketsRecv: 99,
				CreatedAt:   time.Now().Add(-2 * time.Hour),
			},
			{
				MonitorID:   monitor.ID,
				PacketLoss:  2.0,
				AvgRTT:      25.0,
				PacketsSent: 100,
				PacketsRecv: 98,
				CreatedAt:   time.Now().Add(-1 * time.Hour),
			},
			{
				MonitorID:   monitor.ID,
				PacketLoss:  0.0,
				AvgRTT:      15.0,
				PacketsSent: 100,
				PacketsRecv: 100,
				CreatedAt:   time.Now(),
			},
		}

		for _, r := range results {
			result := r // capture loop variable
			err := td.Service.SavePacketLossResult(&result)
			require.NoError(t, err)
		}

		// Get latest result
		latest, err := td.Service.GetLatestPacketLossResult(monitor.ID)
		require.NoError(t, err)
		require.NotNil(t, latest)

		// Should be the most recent one (0% packet loss)
		assert.Equal(t, 0.0, latest.PacketLoss)
		assert.Equal(t, 15.0, latest.AvgRTT)
	})
}

// TestGetPacketLossResults 测试获取丢包测试结果
// 此测试验证获取丢包测试结果（带数量限制）的功能
// 测试步骤：
// 1. 首先创建一个丢包监控
// 2. 保存10个不同时间的丢包测试结果
// 3. 获取最新的5个丢包测试结果
// 4. 验证返回的结果数量是否正确
// 5. 验证结果是否按时间降序排列（最新的在前）
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestGetPacketLossResults(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// Create a monitor
		monitor := CreateTestPacketLossMonitor(t, td)

		// Save multiple results
		for i := 0; i < 10; i++ {
			result := &types.PacketLossResult{
				MonitorID:   monitor.ID,
				PacketLoss:  float64(i),
				AvgRTT:      20.0 + float64(i),
				PacketsSent: 100,
				PacketsRecv: 100 - i,
				CreatedAt:   time.Now().Add(time.Duration(-i) * time.Hour),
			}
			err := td.Service.SavePacketLossResult(result)
			require.NoError(t, err)
		}

		// Get results with limit
		results, err := td.Service.GetPacketLossResults(monitor.ID, 5)
		require.NoError(t, err)
		assert.Len(t, results, 5)

		// Verify order (should be newest first)
		for i := 1; i < len(results); i++ {
			assert.True(t, results[i-1].CreatedAt.After(results[i].CreatedAt) ||
				results[i-1].CreatedAt.Equal(results[i].CreatedAt))
		}
	})
}

// TestUpdatePacketLossMonitorState 测试更新丢包监控状态
// 此测试验证更新丢包监控状态的功能
// 测试步骤：
// 1. 首先创建一个丢包监控
// 2. 更新监控状态
// 3. 获取更新后的监控
// 4. 验证状态是否已更新
// 5. 验证状态更新时间是否已设置
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestUpdatePacketLossMonitorState(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// Create a monitor
		monitor := CreateTestPacketLossMonitor(t, td)

		// Update state
		newState := "degraded"
		err := td.Service.UpdatePacketLossMonitorState(monitor.ID, newState)
		require.NoError(t, err)

		// Verify state change
		updated, err := td.Service.GetPacketLossMonitor(monitor.ID)
		require.NoError(t, err)
		assert.Equal(t, newState, updated.LastState)
		assert.NotNil(t, updated.LastStateChange)
	})
}

// TestPacketLossMonitor_NotFound 测试处理不存在的丢包监控
// 此测试验证处理不存在的丢包监控的功能
// 测试步骤：
// 1. 尝试获取不存在的丢包监控
// 2. 尝试更新不存在的丢包监控状态
// 3. 尝试删除不存在的丢包监控
// 4. 验证各种操作的错误处理
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestPacketLossMonitor_NotFound(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// Try to get non-existent monitor
		monitor, err := td.Service.GetPacketLossMonitor(99999)
		assert.Error(t, err)
		assert.Nil(t, monitor)

		// Try to update non-existent monitor
		err = td.Service.UpdatePacketLossMonitorState(99999, "healthy")
		assert.Error(t, err)

		// Try to delete non-existent monitor
		_ = td.Service.DeletePacketLossMonitor(99999)
		// Deletion might not return error for non-existent records
		// depending on implementation, so we explicitly ignore it
	})
}

// TestSavePacketLossResult_WithMTRData 测试保存带有MTR数据的丢包测试结果
// 此测试验证保存带有MTR数据的丢包测试结果的功能
// 测试步骤：
// 1. 首先创建一个丢包监控
// 2. 创建一个带有复杂MTR数据的丢包测试结果
// 3. 保存丢包测试结果
// 4. 获取最新的丢包测试结果
// 5. 验证MTR数据是否保存成功
// 6. 验证跳数是否正确
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSavePacketLossResult_WithMTRData(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// Create a monitor
		monitor := CreateTestPacketLossMonitor(t, td)

		// Complex MTR data
		mtrData := `{
			"hops": [
				{"addr": "192.168.1.1", "loss": 0, "avg": 1.2},
				{"addr": "10.0.0.1", "loss": 0, "avg": 5.3},
				{"addr": "8.8.8.8", "loss": 0, "avg": 15.4}
			]
		}`

		result := &types.PacketLossResult{
			MonitorID:      monitor.ID,
			PacketLoss:     0.0,
			MinRTT:         1.0,
			MaxRTT:         20.0,
			AvgRTT:         10.0,
			StdDevRTT:      2.5,
			PacketsSent:    100,
			PacketsRecv:    100,
			UsedMTR:        true,
			HopCount:       3,
			MTRData:        &mtrData,
			PrivilegedMode: true,
			CreatedAt:      time.Now(),
		}

		err := td.Service.SavePacketLossResult(result)
		require.NoError(t, err)

		// Verify MTR data was saved correctly
		latest, err := td.Service.GetLatestPacketLossResult(monitor.ID)
		require.NoError(t, err)
		require.NotNil(t, latest.MTRData)
		assert.Contains(t, *latest.MTRData, "192.168.1.1")
		assert.Equal(t, 3, latest.HopCount)
	})
}
