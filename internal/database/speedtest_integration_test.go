// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 此文件包含测速系统的集成测试
// 测试测速结果的保存、分页查询、时间范围过滤、不同测试类型等功能
// 所有测试都在SQLite和PostgreSQL两种数据库上运行

package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/netronome/internal/config"
	"github.com/autobrr/netronome/internal/types"
)

// TestSpeedTest_Save 测试保存测速结果
// 此测试验证测速结果的保存功能和基本字段的正确性
// 测试步骤：
// 1. 创建一个测速结果对象
// 2. 保存测速结果到数据库
// 3. 验证保存成功并返回了正确的ID
// 4. 验证数据库中存在该记录
// 5. 查询返回的结果并验证创建时间已被设置
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSpeedTest_Save(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		serverHost := "speedtest.example.com"
		jitter := 2.5

		speedTest := types.SpeedTestResult{
			ServerName:    "Test Server",
			ServerID:      "test-123",
			ServerHost:    &serverHost,
			TestType:      "iperf3",
			DownloadSpeed: 100.5,
			UploadSpeed:   50.25,
			Latency:       "10.0ms",
			Jitter:        &jitter,
			IsScheduled:   false,
		}

		// Save speed test
		saved, err := td.Service.SaveSpeedTest(ctx, speedTest)
		require.NoError(t, err)
		require.NotNil(t, saved)
		assert.Greater(t, saved.ID, int64(0))
		assert.Equal(t, speedTest.ServerName, saved.ServerName)
		assert.Equal(t, speedTest.DownloadSpeed, saved.DownloadSpeed)

		// Verify saved in database
		AssertRecordExists(t, td, "speed_tests", "id", saved.ID)

		// Query back to verify created_at was set
		results, err := td.Service.GetSpeedTests(ctx, "all", 1, 10)
		require.NoError(t, err)
		require.Len(t, results.Data, 1)
		assert.NotZero(t, results.Data[0].CreatedAt)
	})
}

// TestSpeedTest_GetWithPagination 测试分页获取测速结果
// 此测试验证分页查询功能的正确性
// 测试步骤：
// 1. 创建25个不同的测速结果
// 2. 使用分页查询获取第一页（10条记录）
// 3. 验证第一页结果数量和总数
// 4. 获取第二页和第三页并验证结果
// 5. 验证结果按创建时间降序排列
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSpeedTest_GetWithPagination(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// Create multiple speed tests
		baseTime := time.Now()
		for i := 0; i < 25; i++ {
			speedTest := types.SpeedTestResult{
				ServerName:    "Server " + string(rune('A'+i)),
				ServerID:      "srv-" + string(rune('0'+i)),
				ServerHost:    stringPtr("host" + string(rune('0'+i)) + ".example.com"),
				TestType:      "speedtest",
				DownloadSpeed: float64(100 + i),
				UploadSpeed:   float64(50 + i),
				Latency:       string(rune('0'+i)) + "0ms",
				IsScheduled:   i%2 == 0,
				CreatedAt:     baseTime.Add(time.Duration(-i) * time.Hour),
			}
			_, err := td.Service.SaveSpeedTest(ctx, speedTest)
			require.NoError(t, err)
		}

		// Test pagination - first page
		page1, err := td.Service.GetSpeedTests(ctx, "all", 1, 10)
		require.NoError(t, err)
		assert.Len(t, page1.Data, 10)
		assert.Equal(t, 25, page1.Total)
		// Calculate total pages
		totalPages := (page1.Total + page1.Limit - 1) / page1.Limit
		assert.Equal(t, 3, totalPages)
		assert.Equal(t, 1, page1.Page)

		// Test pagination - second page
		page2, err := td.Service.GetSpeedTests(ctx, "all", 2, 10)
		require.NoError(t, err)
		assert.Len(t, page2.Data, 10)
		assert.Equal(t, 2, page2.Page)

		// Test pagination - last page
		page3, err := td.Service.GetSpeedTests(ctx, "all", 3, 10)
		require.NoError(t, err)
		assert.Len(t, page3.Data, 5)
		assert.Equal(t, 3, page3.Page)

		// Verify ordering (newest first)
		for i := 1; i < len(page1.Data); i++ {
			assert.True(t,
				page1.Data[i-1].CreatedAt.After(page1.Data[i].CreatedAt) ||
					page1.Data[i-1].CreatedAt.Equal(page1.Data[i].CreatedAt),
			)
		}
	})
}

// TestSpeedTest_TimeRangeFilters 测试按时间范围过滤测速结果
// 此测试验证不同时间范围过滤器的功能
// 测试步骤：
// 1. 创建4个不同时间的测速结果（1小时前、1.5天前、8天前、35天前）
// 2. 使用"24h"过滤器查询（应返回1条记录）
// 3. 使用"week"过滤器查询（应返回2条记录）
// 4. 使用"month"过滤器查询（应返回3条记录）
// 5. 使用"all"过滤器查询（应返回4条记录）
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSpeedTest_TimeRangeFilters(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// Create speed tests at different times
		now := time.Now()
		times := []time.Duration{
			-1 * time.Hour,       // 1 hour ago
			-36 * time.Hour,      // 1.5 days ago (clearly outside 24h)
			-8 * 24 * time.Hour,  // Last week (8 days ago)
			-35 * 24 * time.Hour, // Last month (35 days ago)
		}

		for i, duration := range times {
			createdAt := now.Add(duration)
			speedTest := types.SpeedTestResult{
				ServerName:    "Server" + string(rune('0'+i)),
				ServerID:      "id" + string(rune('0'+i)),
				TestType:      "iperf3",
				DownloadSpeed: 100.0,
				UploadSpeed:   50.0,
				CreatedAt:     createdAt,
			}
			_, err := td.Service.SaveSpeedTest(ctx, speedTest)
			require.NoError(t, err)
		}

		// Test "24h" filter
		results24h, err := td.Service.GetSpeedTests(ctx, "24h", 1, 100)
		require.NoError(t, err)

		assert.Equal(t, 1, results24h.Total) // Only the 1 hour ago test

		// Test "week" filter
		resultsWeek, err := td.Service.GetSpeedTests(ctx, "week", 1, 100)
		require.NoError(t, err)
		assert.Equal(t, 2, resultsWeek.Total) // 1 hour and 1.5 days ago

		// Test "month" filter
		resultsMonth, err := td.Service.GetSpeedTests(ctx, "month", 1, 100)
		require.NoError(t, err)
		assert.Equal(t, 3, resultsMonth.Total) // All except the oldest

		// Test "all" filter
		resultsAll, err := td.Service.GetSpeedTests(ctx, "all", 1, 100)
		require.NoError(t, err)
		assert.Equal(t, 4, resultsAll.Total) // All tests
	})
}

// TestSpeedTest_DifferentTestTypes 测试不同类型的测速
// 此测试验证系统支持不同类型的测速（iperf3、speedtest、librespeed）
// 测试步骤：
// 1. 为每种测试类型创建一个测速结果
// 2. 验证所有测试结果都保存成功
// 3. 查询所有测试结果
// 4. 验证所有测试类型都存在于结果中
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSpeedTest_DifferentTestTypes(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		testTypes := []string{"iperf3", "speedtest", "librespeed"}

		for _, testType := range testTypes {
			speedTest := types.SpeedTestResult{
				ServerName:    testType + " Server",
				ServerID:      testType + "-123",
				ServerHost:    stringPtr(testType + ".example.com"),
				TestType:      testType,
				DownloadSpeed: 100.0,
				UploadSpeed:   50.0,
				Latency:       "15.0ms",
			}

			saved, err := td.Service.SaveSpeedTest(ctx, speedTest)
			require.NoError(t, err)
			assert.Equal(t, testType, saved.TestType)
		}

		// Verify all test types were saved
		results, err := td.Service.GetSpeedTests(ctx, "all", 1, 100)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, results.Total, len(testTypes))

		// Check that all test types are present
		foundTypes := make(map[string]bool)
		for _, test := range results.Data {
			foundTypes[test.TestType] = true
		}

		for _, testType := range testTypes {
			assert.True(t, foundTypes[testType], "Test type %s should be present", testType)
		}
	})
}

// TestSpeedTest_ScheduledVsManual 测试计划测速与手动测速
// 此测试验证系统能够区分计划测速和手动测速
// 测试步骤：
// 1. 创建一个计划测速结果（IsScheduled=true）
// 2. 创建一个手动测速结果（IsScheduled=false）
// 3. 验证两种测速都保存成功
// 4. 直接查询数据库验证IsScheduled字段的值
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSpeedTest_ScheduledVsManual(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// Create scheduled test
		scheduledTest := types.SpeedTestResult{
			ServerName:    "Scheduled Server",
			ServerID:      "sched-123",
			TestType:      "iperf3",
			DownloadSpeed: 150.0,
			UploadSpeed:   75.0,
			IsScheduled:   true,
		}

		savedScheduled, err := td.Service.SaveSpeedTest(ctx, scheduledTest)
		require.NoError(t, err)
		assert.True(t, savedScheduled.IsScheduled)

		// Create manual test
		manualTest := types.SpeedTestResult{
			ServerName:    "Manual Server",
			ServerID:      "manual-123",
			TestType:      "speedtest",
			DownloadSpeed: 200.0,
			UploadSpeed:   100.0,
			IsScheduled:   false,
		}

		savedManual, err := td.Service.SaveSpeedTest(ctx, manualTest)
		require.NoError(t, err)
		assert.False(t, savedManual.IsScheduled)

		// Verify both are saved correctly
		var isScheduled bool

		// Build query based on database type
		var query string
		if td.Config.Type == config.Postgres {
			query = "SELECT is_scheduled FROM speed_tests WHERE id = $1"
		} else {
			query = "SELECT is_scheduled FROM speed_tests WHERE id = ?"
		}

		err = td.DB.QueryRow(query, savedScheduled.ID).Scan(&isScheduled)
		require.NoError(t, err)
		assert.True(t, isScheduled)

		err = td.DB.QueryRow(query, savedManual.ID).Scan(&isScheduled)
		require.NoError(t, err)
		assert.False(t, isScheduled)
	})
}

// TestSpeedTest_NullableFields 测试可空字段的处理
// 此测试验证系统能够正确处理可空字段（如Latency和Jitter）
// 测试步骤：
// 1. 创建一个只有必要字段的测速结果（Latency和Jitter可能为null）
// 2. 验证保存成功
// 3. 查询返回的结果
// 4. 验证必要字段的值
// 5. 验证可空字段的处理（Jitter如果不为null，应大于等于0）
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestSpeedTest_NullableFields(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// Create test with minimal fields (some fields might be nullable)
		minimalTest := types.SpeedTestResult{
			ServerName:    "Minimal Server",
			ServerID:      "min-123",
			TestType:      "speedtest",
			DownloadSpeed: 50.0,
			UploadSpeed:   25.0,
			// Latency and Jitter might be 0/null
		}

		saved, err := td.Service.SaveSpeedTest(ctx, minimalTest)
		require.NoError(t, err)

		// Retrieve and verify
		results, err := td.Service.GetSpeedTests(ctx, "all", 1, 10)
		require.NoError(t, err)
		require.Greater(t, len(results.Data), 0)

		// Find our test
		var found bool
		for _, test := range results.Data {
			if test.ID == saved.ID {
				found = true
				assert.Equal(t, minimalTest.ServerName, test.ServerName)
				assert.Equal(t, minimalTest.DownloadSpeed, test.DownloadSpeed)
				// Latency should be empty string if not set
				// Jitter could be nil or >= 0
				if test.Jitter != nil {
					assert.GreaterOrEqual(t, *test.Jitter, 0.0)
				}
				break
			}
		}
		assert.True(t, found, "Should find the saved test")
	})
}
