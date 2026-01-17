// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 此文件包含调度系统的集成测试
// 测试调度系统的CRUD操作、多调度器支持、不同测试类型和时间戳行为等
// 所有测试都在SQLite和PostgreSQL两种数据库上运行

package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/netronome/internal/types"
)

// TestSchedule_CRUD 测试调度的基本CRUD操作
// 此测试验证调度的创建、读取、更新和删除功能
// 测试步骤：
// 1. 创建一个新的调度
// 2. 验证调度创建成功
// 3. 读取所有调度并验证数量
// 4. 更新调度信息
// 5. 验证更新成功
// 6. 删除调度
// 7. 验证调度已被删除
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSchedule_CRUD(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// Create schedule
		schedule := types.Schedule{
			ServerIDs: []string{"server-123", "server-456"},
			Interval:  "6h", // Every 6 hours
			NextRun:   time.Now().Add(6 * time.Hour),
			Enabled:   true,
			Options: types.TestOptions{
				UseIperf:       true,
				EnableDownload: true,
				EnableUpload:   true,
			},
		}

		created, err := td.Service.CreateSchedule(ctx, schedule)
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Greater(t, created.ID, int64(0))
		assert.Equal(t, schedule.ServerIDs, created.ServerIDs)
		assert.Equal(t, schedule.Interval, created.Interval)
		assert.NotZero(t, created.CreatedAt)
		// Schedule doesn't have UpdatedAt in the current struct

		// Read all schedules
		schedules, err := td.Service.GetSchedules(ctx)
		require.NoError(t, err)
		assert.Len(t, schedules, 1)
		assert.Equal(t, created.ID, schedules[0].ID)

		// Update schedule
		created.ServerIDs = append(created.ServerIDs, "server-789")
		created.Enabled = false
		created.Interval = "24h" // Daily

		err = td.Service.UpdateSchedule(ctx, *created)
		require.NoError(t, err)

		// Verify update
		schedules, err = td.Service.GetSchedules(ctx)
		require.NoError(t, err)
		require.Len(t, schedules, 1)
		assert.Len(t, schedules[0].ServerIDs, 3)
		assert.False(t, schedules[0].Enabled)
		assert.Equal(t, "24h", schedules[0].Interval)

		// Delete schedule
		err = td.Service.DeleteSchedule(ctx, created.ID)
		require.NoError(t, err)

		// Verify deletion
		schedules, err = td.Service.GetSchedules(ctx)
		require.NoError(t, err)
		assert.Len(t, schedules, 0)
	})
}

// TestSchedule_MultipleSchedules 测试多个调度的管理
// 此测试验证系统支持创建和管理多个调度
// 测试步骤：
// 1. 创建多个不同配置的调度
// 2. 验证所有调度都被成功创建
// 3. 验证所有调度都能被正确检索
// 4. 删除一个调度
// 5. 验证剩余调度数量正确
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSchedule_MultipleSchedules(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// Create multiple schedules
		testSchedules := []types.Schedule{
			{
				ServerIDs: []string{"srv1"},
				Interval:  "1h",
				NextRun:   time.Now().Add(1 * time.Hour),
				Enabled:   true,
				Options: types.TestOptions{
					EnableDownload: true,
					EnableUpload:   true,
					EnablePing:     true,
				},
			},
			{
				ServerIDs: []string{"srv2", "srv3"},
				Interval:  "24h",
				NextRun:   time.Now().Add(24 * time.Hour),
				Enabled:   true,
				Options: types.TestOptions{
					UseIperf:       true,
					EnableDownload: true,
					EnableUpload:   true,
				},
			},
			{
				ServerIDs: []string{"srv4"},
				Interval:  "168h", // Weekly
				NextRun:   time.Now().Add(168 * time.Hour),
				Enabled:   false,
				Options: types.TestOptions{
					UseLibrespeed:  true,
					EnableDownload: true,
					EnableUpload:   true,
				},
			},
		}

		// Create all schedules
		var createdIDs []int64
		for _, sched := range testSchedules {
			created, err := td.Service.CreateSchedule(ctx, sched)
			require.NoError(t, err)
			createdIDs = append(createdIDs, created.ID)
		}

		// Get all schedules
		schedules, err := td.Service.GetSchedules(ctx)
		require.NoError(t, err)
		assert.Len(t, schedules, len(testSchedules))

		// Verify all schedules are present by checking intervals
		foundIntervals := make(map[string]bool)
		for _, sched := range schedules {
			foundIntervals[sched.Interval] = true
		}

		expectedIntervals := []string{"1h", "24h", "168h"}
		for _, interval := range expectedIntervals {
			assert.True(t, foundIntervals[interval],
				"Schedule with interval %s should be present", interval)
		}

		// Delete specific schedule
		err = td.Service.DeleteSchedule(ctx, createdIDs[1])
		require.NoError(t, err)

		// Verify only 2 remain
		schedules, err = td.Service.GetSchedules(ctx)
		require.NoError(t, err)
		assert.Len(t, schedules, 2)
	})
}

// TestSchedule_UpdateNonExistent 测试更新不存在的调度
// 此测试验证系统对更新不存在调度的错误处理
// 测试步骤：
// 1. 尝试更新一个ID为99999的不存在调度
// 2. 验证系统返回适当的错误
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSchedule_UpdateNonExistent(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// Try to update non-existent schedule
		nonExistent := types.Schedule{
			ID:        99999,
			ServerIDs: []string{"non-existent"},
			Interval:  "1h",
			NextRun:   time.Now(),
			Enabled:   true,
			Options:   types.TestOptions{UseIperf: true},
		}

		err := td.Service.UpdateSchedule(ctx, nonExistent)
		assert.Error(t, err)
	})
}

// TestSchedule_DeleteNonExistent 测试删除不存在的调度
// 此测试验证系统对删除不存在调度的处理
// 测试步骤：
// 1. 尝试删除一个ID为99999的不存在调度
// 2. 验证系统不会因删除不存在的调度而崩溃
//
// 注意：不同数据库对DELETE操作无匹配记录的处理可能不同，
// 有些数据库会返回错误，有些则不会，因此我们仅确保不崩溃
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSchedule_DeleteNonExistent(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// Try to delete non-existent schedule
		err := td.Service.DeleteSchedule(ctx, 99999)
		// Some databases might not return error for DELETE with no matches
		// So we just ensure it doesn't panic
		_ = err
	})
}

// TestSchedule_DifferentTestTypes 测试不同类型的测试配置
// 此测试验证调度支持不同类型的网络测试配置
// 测试步骤：
// 1. 创建不同测试类型的调度：iperf3、speedtest和librespeed
// 2. 验证所有调度都能被成功创建
// 3. 验证每种测试类型的配置都被正确保存
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSchedule_DifferentTestTypes(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		testConfigs := []struct {
			name    string
			options types.TestOptions
		}{
			{
				"iperf3",
				types.TestOptions{
					UseIperf:       true,
					EnableDownload: true,
					EnableUpload:   true,
				},
			},
			{
				"speedtest",
				types.TestOptions{
					EnableDownload: true,
					EnableUpload:   true,
					EnablePing:     true,
					EnableJitter:   true,
				},
			},
			{
				"librespeed",
				types.TestOptions{
					UseLibrespeed:  true,
					EnableDownload: true,
					EnableUpload:   true,
				},
			},
		}

		for _, config := range testConfigs {
			schedule := types.Schedule{
				ServerIDs: []string{"srv-" + config.name},
				Interval:  "1h",
				NextRun:   time.Now().Add(1 * time.Hour),
				Enabled:   true,
				Options:   config.options,
			}

			created, err := td.Service.CreateSchedule(ctx, schedule)
			require.NoError(t, err)

			// Verify options were saved correctly
			switch config.name {
			case "iperf3":
				assert.True(t, created.Options.UseIperf)
			case "librespeed":
				assert.True(t, created.Options.UseLibrespeed)
			}
		}

		// Verify all schedules were created
		schedules, err := td.Service.GetSchedules(ctx)
		require.NoError(t, err)
		assert.Len(t, schedules, len(testConfigs))
	})
}

// TestSchedule_IntervalFormats 测试不同的间隔格式
// 此测试验证系统支持各种时间间隔格式
// 测试步骤：
// 1. 测试不同的时间间隔格式：分钟、小时、天和周
// 2. 验证每种间隔格式都能被正确保存
// 3. 创建后立即删除每个测试调度
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSchedule_IntervalFormats(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// Test various interval formats
		intervalTests := []struct {
			name     string
			interval string
		}{
			{"Every minute", "1m"},
			{"Every hour", "1h"},
			{"Every 6 hours", "6h"},
			{"Daily", "24h"},
			{"Weekly", "168h"},
			{"Every 30 minutes", "30m"},
			{"Every 12 hours", "12h"},
		}

		for _, test := range intervalTests {
			schedule := types.Schedule{
				ServerIDs: []string{"interval-test"},
				Interval:  test.interval,
				NextRun:   time.Now().Add(1 * time.Hour),
				Enabled:   true,
				Options: types.TestOptions{
					EnableDownload: true,
					EnableUpload:   true,
					EnablePing:     true,
				},
			}

			created, err := td.Service.CreateSchedule(ctx, schedule)
			require.NoError(t, err)
			assert.Equal(t, test.interval, created.Interval)

			// Clean up
			err = td.Service.DeleteSchedule(ctx, created.ID)
			require.NoError(t, err)
		}
	})
}

// TestSchedule_TimestampBehavior 测试时间戳的行为
// 此测试验证调度的时间戳在更新时的行为
// 测试步骤：
// 1. 创建一个调度
// 2. 记录创建时间和下次运行时间
// 3. 等待一段时间后更新调度
// 4. 验证创建时间保持不变
// 5. 验证下次运行时间已更新
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSchedule_TimestampBehavior(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// Create schedule
		schedule := types.Schedule{
			ServerIDs: []string{"ts-test"},
			Interval:  "1h",
			NextRun:   time.Now().Add(1 * time.Hour),
			Enabled:   true,
			Options: types.TestOptions{
				UseIperf:       true,
				EnableDownload: true,
				EnableUpload:   true,
			},
		}

		created, err := td.Service.CreateSchedule(ctx, schedule)
		require.NoError(t, err)

		originalCreatedAt := created.CreatedAt
		originalNextRun := created.NextRun

		// Sleep to ensure time difference
		time.Sleep(100 * time.Millisecond)

		// Update schedule
		created.Interval = "2h"
		created.NextRun = time.Now().Add(2 * time.Hour)
		err = td.Service.UpdateSchedule(ctx, *created)
		require.NoError(t, err)

		// Get updated schedule
		schedules, err := td.Service.GetSchedules(ctx)
		require.NoError(t, err)
		require.Len(t, schedules, 1)

		// CreatedAt should not change
		assert.Equal(t, originalCreatedAt.Unix(), schedules[0].CreatedAt.Unix())

		// NextRun should be updated
		assert.NotEqual(t, originalNextRun.Unix(), schedules[0].NextRun.Unix(),
			"NextRun should be different after update")
	})
}
