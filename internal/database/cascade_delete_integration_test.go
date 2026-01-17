// Copyright (c) 2024-2025, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// 数据库级联删除集成测试
// 该文件包含了 Netronome 项目中数据库级联删除功能的集成测试
// 主要验证删除父记录时，相关的子记录能否正确地被级联删除
//
// 测试内容包括：
// 1. 丢包监视器 (PacketLossMonitor) 的级联删除
// 2. 监控代理 (MonitorAgent) 的级联删除
// 3. 通知渠道 (NotificationChannel) 的级联删除
// 4. 通知事件 (NotificationEvent) 的级联删除（已跳过，因为事件是种子数据）
// 5. 级联删除的性能测试
//
// 测试方法：
// - 所有测试都在 SQLite 和 PostgreSQL 两种数据库上运行
// - 使用 RunTestWithBothDatabases 函数确保兼容性
// - 验证数据创建、读取、删除的完整流程
// - 验证级联删除后相关数据的完整性
package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/netronome/internal/types"
)

// TestPacketLossMonitor_CascadeDelete 测试丢包监视器的级联删除功能
// 该函数验证当删除一个丢包监视器时，它的所有结果记录是否会被正确级联删除
// 测试在 SQLite 和 PostgreSQL 两种数据库上运行，确保跨数据库兼容性
func TestPacketLossMonitor_CascadeDelete(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建一个测试用的丢包监视器
		monitor := CreateTestPacketLossMonitor(t, td)

		// 为该监视器创建5个测试结果
		resultIDs := make([]int64, 5)
		for i := 0; i < 5; i++ {
			// 创建MTR数据（网络路径数据）
			mtrData := `{"hops": [{"addr": "192.168.1.1", "loss": 0}]}`
			// 构建丢包结果对象
			result := &types.PacketLossResult{
				MonitorID:   monitor.ID,                                    // 关联到刚创建的监视器
				PacketLoss:  float64(i),                                    // 丢包率从0%到4%
				MinRTT:      10.0 + float64(i),                             // 最小往返时间递增
				MaxRTT:      20.0 + float64(i),                             // 最大往返时间递增
				AvgRTT:      15.0 + float64(i),                             // 平均往返时间递增
				PacketsSent: 10,                                            // 发送的数据包数量
				PacketsRecv: 10 - i,                                        // 接收的数据包数量（递减，模拟不同的丢包情况）
				MTRData:     &mtrData,                                      // 网络路径数据
				UsedMTR:     true,                                          // 是否使用了MTR功能
				HopCount:    1,                                             // 跳数
				CreatedAt:   time.Now().Add(time.Duration(-i) * time.Hour), // 创建时间（每小时递减）
			}

			// 保存丢包结果到数据库
			err := td.Service.SavePacketLossResult(result)
			require.NoError(t, err)
			// 记录创建的结果ID，用于后续验证
			resultIDs[i] = result.ID
		}

		// 验证所有结果都已成功创建
		for _, id := range resultIDs {
			AssertRecordExists(t, td, "packet_loss_results", "id", id)
		}

		// 验证可以通过监视器ID检索到所有结果
		results, err := td.Service.GetPacketLossResults(monitor.ID, 10)
		require.NoError(t, err)
		assert.Len(t, results, 5) // 应该有5个结果

		// 删除监视器
		err = td.Service.DeletePacketLossMonitor(monitor.ID)
		require.NoError(t, err)

		// 验证监视器已被删除
		AssertRecordNotExists(t, td, "packet_loss_monitors", "id", monitor.ID)

		// 验证所有结果都已被级联删除
		for _, id := range resultIDs {
			AssertRecordNotExists(t, td, "packet_loss_results", "id", id)
		}

		// 再次检查：尝试通过监视器ID检索结果，应该为空
		results, err = td.Service.GetPacketLossResults(monitor.ID, 10)
		require.NoError(t, err)
		assert.Empty(t, results) // 结果列表应该为空
	})
}

// TestMonitorAgent_CascadeDelete 测试监控代理的级联删除功能
// 该函数验证当删除一个监控代理时，它的所有相关数据（资源统计、峰值统计、历史快照）是否会被正确级联删除
// 测试在 SQLite 和 PostgreSQL 两种数据库上运行，确保跨数据库兼容性
func TestMonitorAgent_CascadeDelete(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()

		// 创建一个测试用的监控代理
		agent := &types.MonitorAgent{
			Name:    "级联删除测试代理",
			URL:     "http://test-agent.local",
			APIKey:  stringPtr("test-api-key"),
			Enabled: true,
		}

		// 保存代理到数据库
		created, err := td.Service.CreateMonitorAgent(ctx, agent)
		require.NoError(t, err)

		// 为该代理保存各种类型的数据

		// 1. 资源统计数据（CPU、内存、磁盘等）
		resourceStats := &types.MonitorResourceStats{
			CPUUsagePercent:   25.5,                                                                                           // CPU使用率25.5%
			MemoryUsedPercent: 45.5,                                                                                           // 内存使用率45.5%
			SwapUsedPercent:   10.0,                                                                                           // 交换分区使用率10.0%
			DiskUsageJSON:     `[{"path":"/","total":100000000000,"used":50000000000,"free":50000000000,"usedPercent":50.0}]`, // 磁盘使用情况
			TemperatureJSON:   `[{"sensorKey":"cpu","temperature":55.0}]`,                                                     // CPU温度55.0°C
			UptimeSeconds:     3600,                                                                                           // 运行时间3600秒（1小时）
		}

		// 保存资源统计数据
		err = td.Service.SaveMonitorResourceStats(ctx, created.ID, resourceStats)
		require.NoError(t, err)

		// 2. 峰值统计数据（网络流量峰值）
		now := time.Now()
		peakStats := &types.MonitorPeakStats{
			PeakRxBytes:     1000000000, // 峰值接收字节数（1GB）
			PeakTxBytes:     500000000,  // 峰值发送字节数（500MB）
			PeakRxTimestamp: &now,       // 峰值接收时间戳
			PeakTxTimestamp: &now,       // 峰值发送时间戳
		}

		// 更新或插入峰值统计数据
		err = td.Service.UpsertMonitorPeakStats(ctx, created.ID, peakStats)
		require.NoError(t, err)

		// 3. 历史快照数据
		snapshot := &types.MonitorHistoricalSnapshot{
			PeriodType: "daily",                 // 周期类型：每日
			DataJSON:   `{"detailed": "stats"}`, // 快照数据内容
		}

		// 保存历史快照
		err = td.Service.SaveMonitorHistoricalSnapshot(ctx, created.ID, snapshot)
		require.NoError(t, err)

		// 验证所有数据都已成功创建
		AssertRecordExists(t, td, "monitor_agents", "id", created.ID)
		AssertRecordExists(t, td, "monitor_resource_stats", "agent_id", created.ID)
		AssertRecordExists(t, td, "monitor_peak_stats", "agent_id", created.ID)
		AssertRecordExists(t, td, "monitor_historical_snapshots", "agent_id", created.ID)

		// 验证可以通过代理ID检索到所有数据

		// 使用24小时避免SQLite的时区问题
		retrievedStats, err := td.Service.GetMonitorResourceStats(ctx, created.ID, 24)
		require.NoError(t, err)
		assert.Len(t, retrievedStats, 1) // 应该有1条资源统计记录

		// 验证可以获取峰值统计数据
		retrievedPeaks, err := td.Service.GetMonitorPeakStats(ctx, created.ID)
		require.NoError(t, err)
		assert.NotNil(t, retrievedPeaks) // 应该返回非空的峰值统计数据

		// 验证可以获取历史快照
		retrievedSnapshot, err := td.Service.GetMonitorLatestSnapshot(ctx, created.ID, "daily")
		require.NoError(t, err)
		assert.NotNil(t, retrievedSnapshot) // 应该返回非空的历史快照

		// 删除监控代理
		err = td.Service.DeleteMonitorAgent(ctx, created.ID)
		require.NoError(t, err)

		// 验证代理已被删除
		AssertRecordNotExists(t, td, "monitor_agents", "id", created.ID)

		// 验证所有相关数据都已被级联删除
		AssertRecordNotExists(t, td, "monitor_resource_stats", "agent_id", created.ID)
		AssertRecordNotExists(t, td, "monitor_peak_stats", "agent_id", created.ID)
		AssertRecordNotExists(t, td, "monitor_historical_snapshots", "agent_id", created.ID)

		// 再次检查：尝试检索数据，验证确实已删除

		// 使用24小时避免SQLite的时区问题
		retrievedStats, err = td.Service.GetMonitorResourceStats(ctx, created.ID, 24)
		require.NoError(t, err)
		assert.Empty(t, retrievedStats) // 资源统计应该为空

		// 尝试获取峰值统计，应该返回错误
		retrievedPeaks, err = td.Service.GetMonitorPeakStats(ctx, created.ID)
		assert.ErrorIs(t, err, ErrNotFound) // 应该返回"未找到"错误
		assert.Nil(t, retrievedPeaks)       // 应该返回nil

		// 尝试获取历史快照，应该返回错误
		retrievedSnapshot, err = td.Service.GetMonitorLatestSnapshot(ctx, created.ID, "daily")
		assert.ErrorIs(t, err, ErrNotFound) // 应该返回"未找到"错误
		assert.Nil(t, retrievedSnapshot)    // 应该返回nil
	})
}

// TestNotificationChannel_CascadeDelete 测试通知渠道的级联删除功能
// 该函数验证当删除一个通知渠道时，它的所有规则和历史记录是否会被正确级联删除
// 同时确保其他通知渠道不受影响
// 测试在 SQLite 和 PostgreSQL 两种数据库上运行，确保跨数据库兼容性
func TestNotificationChannel_CascadeDelete(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()
		_ = ctx

		// 创建两个通知渠道
		channel1, err := td.Service.CreateChannel(NotificationChannelInput{
			Name:    "级联测试渠道 1",
			URL:     "https://webhook.example.com/1",
			Enabled: boolPtr(true),
		})
		require.NoError(t, err)

		channel2, err := td.Service.CreateChannel(NotificationChannelInput{
			Name:    "级联测试渠道 2",
			URL:     "https://webhook.example.com/2",
			Enabled: boolPtr(true),
		})
		require.NoError(t, err)

		// 获取不同类型的通知事件
		// 测速完成事件
		speedtestEvent, err := td.Service.GetEventByType(NotificationCategorySpeedtest, NotificationEventSpeedtestComplete)
		require.NoError(t, err)

		// 丢包率高事件
		packetlossEvent, err := td.Service.GetEventByType(NotificationCategoryPacketLoss, NotificationEventPacketLossHigh)
		require.NoError(t, err)

		// 代理离线事件
		agentEvent, err := td.Service.GetEventByType(NotificationCategoryAgent, NotificationEventAgentOffline)
		require.NoError(t, err)

		// 为渠道1创建多个规则
		rule1, err := td.Service.CreateRule(NotificationRuleInput{
			ChannelID: channel1.ID,       // 关联到渠道1
			EventID:   speedtestEvent.ID, // 测速完成事件
			Enabled:   boolPtr(true),     // 启用规则
		})
		require.NoError(t, err)

		rule2, err := td.Service.CreateRule(NotificationRuleInput{
			ChannelID: channel1.ID,        // 关联到渠道1
			EventID:   packetlossEvent.ID, // 丢包率高事件
			Enabled:   boolPtr(true),      // 启用规则
		})
		require.NoError(t, err)

		rule3, err := td.Service.CreateRule(NotificationRuleInput{
			ChannelID: channel1.ID,   // 关联到渠道1
			EventID:   agentEvent.ID, // 代理离线事件
			Enabled:   boolPtr(true), // 启用规则
		})
		require.NoError(t, err)

		// 为渠道2创建一个规则（应该不受删除渠道1的影响）
		rule4, err := td.Service.CreateRule(NotificationRuleInput{
			ChannelID: channel2.ID,       // 关联到渠道2
			EventID:   speedtestEvent.ID, // 测速完成事件
			Enabled:   boolPtr(true),     // 启用规则
		})
		require.NoError(t, err)

		// 为渠道1记录通知历史
		payload1 := "测试通知 1"
		err = td.Service.LogNotification(channel1.ID, speedtestEvent.ID, true, nil, &payload1)
		require.NoError(t, err)

		// 记录一个发送失败的通知
		errorMsg := "发送失败"
		payload2 := "测试通知 2"
		err = td.Service.LogNotification(channel1.ID, packetlossEvent.ID, false, &errorMsg, &payload2)
		require.NoError(t, err)

		payload3 := "测试通知 3"
		err = td.Service.LogNotification(channel1.ID, agentEvent.ID, true, nil, &payload3)
		require.NoError(t, err)

		// 为渠道2记录通知历史
		payload4 := "测试通知 4"
		err = td.Service.LogNotification(channel2.ID, speedtestEvent.ID, true, nil, &payload4)
		require.NoError(t, err)

		// 验证所有数据都已成功创建
		AssertRecordExists(t, td, "notification_channels", "id", channel1.ID)
		AssertRecordExists(t, td, "notification_rules", "id", rule1.ID)
		AssertRecordExists(t, td, "notification_rules", "id", rule2.ID)
		AssertRecordExists(t, td, "notification_rules", "id", rule3.ID)
		AssertRecordExists(t, td, "notification_rules", "id", rule4.ID)

		// 验证渠道1有3条通知历史记录
		var historyCount int
		query := "SELECT COUNT(*) FROM notification_history WHERE channel_id = ?"
		if td.Config.Type == "postgres" {
			query = "SELECT COUNT(*) FROM notification_history WHERE channel_id = $1"
		}
		err = td.DB.QueryRow(query, channel1.ID).Scan(&historyCount)
		require.NoError(t, err)
		assert.Equal(t, 3, historyCount)

		// 删除渠道1
		err = td.Service.DeleteChannel(channel1.ID)
		require.NoError(t, err)

		// 验证渠道1已被删除
		AssertRecordNotExists(t, td, "notification_channels", "id", channel1.ID)

		// 验证渠道1的所有规则都已被级联删除
		AssertRecordNotExists(t, td, "notification_rules", "id", rule1.ID)
		AssertRecordNotExists(t, td, "notification_rules", "id", rule2.ID)
		AssertRecordNotExists(t, td, "notification_rules", "id", rule3.ID)

		// 验证渠道2及其规则不受影响
		AssertRecordExists(t, td, "notification_channels", "id", channel2.ID)
		AssertRecordExists(t, td, "notification_rules", "id", rule4.ID)

		// 验证渠道1的通知历史已被级联删除
		err = td.DB.QueryRow(query, channel1.ID).Scan(&historyCount)
		require.NoError(t, err)
		assert.Equal(t, 0, historyCount)

		// 验证渠道2的通知历史仍然存在
		if td.Config.Type == "postgres" {
			query = "SELECT COUNT(*) FROM notification_history WHERE channel_id = $1"
		}
		err = td.DB.QueryRow(query, channel2.ID).Scan(&historyCount)
		require.NoError(t, err)
		assert.Equal(t, 1, historyCount)

		// 再次检查：尝试通过渠道ID检索规则
		rules, err := td.Service.GetRulesByChannel(channel1.ID)
		require.NoError(t, err)
		assert.Empty(t, rules) // 渠道1的规则应该为空

		rules, err = td.Service.GetRulesByChannel(channel2.ID)
		require.NoError(t, err)
		assert.Len(t, rules, 1) // 渠道2应该有1条规则
	})
}

// TestNotificationEvent_CascadeDelete 测试通知事件的级联删除功能
// 该函数验证删除通知事件时，相关的规则和历史记录是否会被级联删除
// 注意：在实际应用中，通知事件通常是种子数据，不应该被删除
// 这个测试仅用于记录如果事件被删除时的级联行为
func TestNotificationEvent_CascadeDelete(t *testing.T) {
	// 注意：在实际应用中，事件通常是种子数据，不应该被删除。
	// 这个测试仅用于记录如果事件被删除时的级联行为。
	t.Skip("通知事件是种子数据，在正常操作中不应该被删除")
}

// TestCascadeDelete_Performance 测试级联删除的性能
// 该函数验证级联删除在处理大数据集时的效率
// 测试创建大量记录并测量级联删除的执行时间
func TestCascadeDelete_Performance(t *testing.T) {
	// 如果在短模式下运行，则跳过性能测试
	if testing.Short() {
		t.Skip("在短模式下跳过性能测试")
	}

	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		// 创建一个测试用的丢包监视器
		monitor := CreateTestPacketLossMonitor(t, td)

		// 创建1000个测试结果，模拟大数据集
		start := time.Now()
		for i := 0; i < 1000; i++ {
			result := &types.PacketLossResult{
				MonitorID:   monitor.ID,                                      // 关联到刚创建的监视器
				PacketLoss:  float64(i%100) / 10.0,                           // 丢包率从0%到9.9%循环
				MinRTT:      10.0,                                            // 最小往返时间固定为10ms
				MaxRTT:      20.0,                                            // 最大往返时间固定为20ms
				AvgRTT:      15.0,                                            // 平均往返时间固定为15ms
				PacketsSent: 10,                                              // 发送的数据包数量
				PacketsRecv: 10,                                              // 接收的数据包数量（无丢包）
				CreatedAt:   time.Now().Add(time.Duration(-i) * time.Minute), // 创建时间（每分钟递减）
			}

			// 保存丢包结果到数据库
			err := td.Service.SavePacketLossResult(result)
			require.NoError(t, err)
		}

		// 记录插入1000条记录的耗时
		insertDuration := time.Since(start)
		t.Logf("插入1000条记录耗时: %v", insertDuration)

		// 验证创建的记录数量
		var count int
		query := "SELECT COUNT(*) FROM packet_loss_results WHERE monitor_id = ?"
		if td.Config.Type == "postgres" {
			query = "SELECT COUNT(*) FROM packet_loss_results WHERE monitor_id = $1"
		}
		err := td.DB.QueryRow(query, monitor.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1000, count) // 应该有1000条记录

		// 删除监视器并测量级联删除的耗时
		start = time.Now()
		err = td.Service.DeletePacketLossMonitor(monitor.ID)
		require.NoError(t, err)
		deleteDuration := time.Since(start)

		// 记录级联删除1000条记录的耗时
		t.Logf("级联删除1000条记录耗时: %v", deleteDuration)

		// 验证所有记录都已被级联删除
		err = td.DB.QueryRow(query, monitor.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count) // 记录数量应该为0

		// 级联删除应该相当快（即使是1000条记录也应该在5秒内完成）
		assert.Less(t, deleteDuration, 5*time.Second)
	})
}
