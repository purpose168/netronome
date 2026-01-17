// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 此文件包含通知系统的集成测试
// 测试范围包括通知频道、通知规则和通知历史记录的CRUD操作
//
// 所有测试都在SQLite和PostgreSQL两种数据库上运行，确保跨数据库兼容性

package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNotificationChannel_CRUD 测试通知频道的CRUD操作
// 此测试验证通知频道的创建、查询、更新和删除功能
// 测试步骤：
// 1. 创建一个新的通知频道
// 2. 通过ID获取刚创建的频道
// 3. 更新频道信息
// 4. 删除频道
// 5. 验证频道已被删除
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestNotificationChannel_CRUD(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()
		_ = ctx

		// Create channel
		input := NotificationChannelInput{
			Name:    "Test Channel",
			URL:     "https://discord.com/api/webhooks/123456",
			Enabled: boolPtr(true),
		}

		created, err := td.Service.CreateChannel(input)
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Greater(t, created.ID, int64(0))
		assert.Equal(t, input.Name, created.Name)
		assert.Equal(t, input.URL, created.URL)
		assert.True(t, created.Enabled)
		assert.NotZero(t, created.CreatedAt)
		assert.NotZero(t, created.UpdatedAt)

		// Get channel by ID
		retrieved, err := td.Service.GetChannel(created.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, created.ID, retrieved.ID)
		assert.Equal(t, created.Name, retrieved.Name)

		// Update channel
		updateInput := NotificationChannelInput{
			Name:    "Updated Channel",
			URL:     "https://slack.com/api/webhooks/789",
			Enabled: boolPtr(false),
		}

		updated, err := td.Service.UpdateChannel(created.ID, updateInput)
		require.NoError(t, err)
		assert.Equal(t, "Updated Channel", updated.Name)
		assert.False(t, updated.Enabled)

		// Delete channel
		err = td.Service.DeleteChannel(created.ID)
		require.NoError(t, err)

		// Verify deletion
		deleted, err := td.Service.GetChannel(created.ID)
		assert.Error(t, err)
		assert.Nil(t, deleted)
	})
}

// TestNotificationChannel_GetAll 测试获取所有通知频道
// 此测试验证获取所有频道和仅获取启用频道的功能
// 测试步骤：
// 1. 创建多个通知频道（包括启用和禁用的）
// 2. 获取所有通知频道并验证数量
// 3. 获取仅启用的通知频道并验证数量
// 4. 验证返回的启用频道确实都是启用状态
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestNotificationChannel_GetAll(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()
		_ = ctx

		// Create multiple channels
		channels := []NotificationChannelInput{
			{
				Name:    "Discord",
				URL:     "https://discord.com/webhook1",
				Enabled: boolPtr(true),
			},
			{
				Name:    "Slack",
				URL:     "https://slack.com/webhook1",
				Enabled: boolPtr(true),
			},
			{
				Name:    "Disabled Channel",
				URL:     "https://example.com/webhook",
				Enabled: boolPtr(false),
			},
		}

		for _, ch := range channels {
			_, err := td.Service.CreateChannel(ch)
			require.NoError(t, err)
		}

		// Get all channels
		all, err := td.Service.GetChannels()
		require.NoError(t, err)
		assert.Len(t, all, 3)

		// Get enabled channels only
		enabled, err := td.Service.GetEnabledChannels()
		require.NoError(t, err)
		assert.Len(t, enabled, 2)

		// Verify all enabled channels are actually enabled
		for _, ch := range enabled {
			assert.True(t, ch.Enabled)
		}
	})
}

// TestNotificationRule_CRUD 测试通知规则的CRUD操作
// 此测试验证通知规则的创建、查询、更新和删除功能
// 测试步骤：
// 1. 首先创建一个通知频道
// 2. 获取一个速度测试事件
// 3. 创建一个通知规则
// 4. 通过ID获取刚创建的规则
// 5. 更新规则信息
// 6. 删除规则
// 7. 验证规则已被删除
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestNotificationRule_CRUD(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()
		_ = ctx

		// First create a channel
		channel, err := td.Service.CreateChannel(NotificationChannelInput{
			Name:    "Test Channel for Rules",
			URL:     "https://example.com/webhook",
			Enabled: boolPtr(true),
		})
		require.NoError(t, err)

		// Get a speedtest event
		event, err := td.Service.GetEventByType(NotificationCategorySpeedtest, NotificationEventSpeedtestDownloadLow)
		require.NoError(t, err)
		require.NotNil(t, event)

		// Create rule
		ruleInput := NotificationRuleInput{
			ChannelID:         channel.ID,
			EventID:           event.ID,
			Enabled:           boolPtr(true),
			ThresholdValue:    float64Ptr(100.0),
			ThresholdOperator: stringPtr("lt"),
		}

		created, err := td.Service.CreateRule(ruleInput)
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Greater(t, created.ID, int64(0))
		assert.Equal(t, channel.ID, created.ChannelID)
		assert.Equal(t, event.ID, created.EventID)
		assert.True(t, created.Enabled)
		assert.NotNil(t, created.ThresholdValue)
		assert.Equal(t, 100.0, *created.ThresholdValue)

		// Get rule by ID
		retrieved, err := td.Service.GetRule(created.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, created.ID, retrieved.ID)

		// Update rule
		updateInput := NotificationRuleInput{
			ChannelID:         channel.ID,
			EventID:           event.ID,
			Enabled:           boolPtr(false),
			ThresholdValue:    float64Ptr(200.0),
			ThresholdOperator: stringPtr("lt"),
		}

		updated, err := td.Service.UpdateRule(created.ID, updateInput)
		require.NoError(t, err)
		assert.False(t, updated.Enabled)
		assert.Equal(t, 200.0, *updated.ThresholdValue)

		// Delete rule
		err = td.Service.DeleteRule(created.ID)
		require.NoError(t, err)

		// Verify deletion
		deleted, err := td.Service.GetRule(created.ID)
		assert.Error(t, err)
		assert.Nil(t, deleted)
	})
}

// TestNotificationRule_GetByChannel 测试按频道获取通知规则
// 此测试验证可以按频道ID获取该频道的所有通知规则
// 测试步骤：
// 1. 创建两个通知频道
// 2. 获取不同类型的事件
// 3. 为第一个频道创建两条规则，为第二个频道创建一条规则
// 4. 分别按频道ID获取规则并验证数量
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestNotificationRule_GetByChannel(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()
		_ = ctx

		// Create two channels
		channel1, err := td.Service.CreateChannel(NotificationChannelInput{
			Name:    "Channel 1",
			URL:     "https://example.com/webhook1",
			Enabled: boolPtr(true),
		})
		require.NoError(t, err)

		channel2, err := td.Service.CreateChannel(NotificationChannelInput{
			Name:    "Channel 2",
			URL:     "https://example.com/webhook2",
			Enabled: boolPtr(true),
		})
		require.NoError(t, err)

		// Get some events
		speedtestEvent, err := td.Service.GetEventByType(NotificationCategorySpeedtest, NotificationEventSpeedtestComplete)
		require.NoError(t, err)

		packetlossEvent, err := td.Service.GetEventByType(NotificationCategoryPacketLoss, NotificationEventPacketLossHigh)
		require.NoError(t, err)

		// Create rules for channel 1
		_, err = td.Service.CreateRule(NotificationRuleInput{
			ChannelID: channel1.ID,
			EventID:   speedtestEvent.ID,
			Enabled:   boolPtr(true),
		})
		require.NoError(t, err)

		_, err = td.Service.CreateRule(NotificationRuleInput{
			ChannelID: channel1.ID,
			EventID:   packetlossEvent.ID,
			Enabled:   boolPtr(true),
		})
		require.NoError(t, err)

		// Create rule for channel 2
		_, err = td.Service.CreateRule(NotificationRuleInput{
			ChannelID: channel2.ID,
			EventID:   speedtestEvent.ID,
			Enabled:   boolPtr(true),
		})
		require.NoError(t, err)

		// Get rules by channel
		channel1Rules, err := td.Service.GetRulesByChannel(channel1.ID)
		require.NoError(t, err)
		assert.Len(t, channel1Rules, 2)

		channel2Rules, err := td.Service.GetRulesByChannel(channel2.ID)
		require.NoError(t, err)
		assert.Len(t, channel2Rules, 1)
	})
}

// TestNotificationEvent_GetByCategory 测试按类别获取通知事件
// 此测试验证可以按类别获取不同类型的通知事件
// 测试步骤：
// 1. 获取速度测试类别的事件
// 2. 验证返回的事件确实都是速度测试类别
// 3. 获取丢包类别的事件
// 4. 获取代理类别的事件
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestNotificationEvent_GetByCategory(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()
		_ = ctx

		// Get speedtest events
		speedtestEvents, err := td.Service.GetEventsByCategory(NotificationCategorySpeedtest)
		require.NoError(t, err)
		assert.NotEmpty(t, speedtestEvents)

		// Verify all are speedtest category
		for _, event := range speedtestEvents {
			assert.Equal(t, NotificationCategorySpeedtest, event.Category)
		}

		// Get packet loss events
		packetlossEvents, err := td.Service.GetEventsByCategory(NotificationCategoryPacketLoss)
		require.NoError(t, err)
		assert.NotEmpty(t, packetlossEvents)

		// Get agent events
		agentEvents, err := td.Service.GetEventsByCategory(NotificationCategoryAgent)
		require.NoError(t, err)
		assert.NotEmpty(t, agentEvents)
	})
}

// TestNotificationRule_EnabledNotifications 测试启用的通知规则
// 此测试验证仅返回同时满足频道和规则都启用的通知规则
// 测试步骤：
// 1. 创建一个启用的频道和一个禁用的频道
// 2. 获取不同类型的事件
// 3. 创建不同组合的规则：
//   - 启用频道 + 启用规则
//   - 禁用频道 + 启用规则
//   - 启用频道 + 禁用规则
//
// 4. 验证获取到的启用规则只包含频道和规则都启用的组合
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestNotificationRule_EnabledNotifications(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()
		_ = ctx

		// Create channels
		enabledChannel, err := td.Service.CreateChannel(NotificationChannelInput{
			Name:    "Enabled Channel",
			URL:     "https://example.com/enabled",
			Enabled: boolPtr(true),
		})
		require.NoError(t, err)

		disabledChannel, err := td.Service.CreateChannel(NotificationChannelInput{
			Name:    "Disabled Channel",
			URL:     "https://example.com/disabled",
			Enabled: boolPtr(false),
		})
		require.NoError(t, err)

		// Get different events for testing
		event1, err := td.Service.GetEventByType(NotificationCategorySpeedtest, NotificationEventSpeedtestComplete)
		require.NoError(t, err)

		event2, err := td.Service.GetEventByType(NotificationCategorySpeedtest, NotificationEventSpeedtestFailed)
		require.NoError(t, err)

		// Create rules with different combinations
		// Enabled channel, enabled rule for event1
		_, err = td.Service.CreateRule(NotificationRuleInput{
			ChannelID: enabledChannel.ID,
			EventID:   event1.ID,
			Enabled:   boolPtr(true),
		})
		require.NoError(t, err)

		// Disabled channel, enabled rule for event1
		_, err = td.Service.CreateRule(NotificationRuleInput{
			ChannelID: disabledChannel.ID,
			EventID:   event1.ID,
			Enabled:   boolPtr(true),
		})
		require.NoError(t, err)

		// Enabled channel, disabled rule for event2
		_, err = td.Service.CreateRule(NotificationRuleInput{
			ChannelID: enabledChannel.ID,
			EventID:   event2.ID,
			Enabled:   boolPtr(false),
		})
		require.NoError(t, err)

		// Get enabled rules for event1
		enabledRules, err := td.Service.GetEnabledRulesForEvent(event1.Category, event1.EventType)
		require.NoError(t, err)

		// Should only have one truly enabled rule (enabled channel + enabled rule)
		assert.Len(t, enabledRules, 1)
		assert.Equal(t, enabledChannel.ID, enabledRules[0].ChannelID)
		assert.True(t, enabledRules[0].Enabled)

		// Get enabled rules for event2
		enabledRulesEvent2, err := td.Service.GetEnabledRulesForEvent(event2.Category, event2.EventType)
		require.NoError(t, err)

		// Should have no enabled rules for event2 (rule is disabled)
		assert.Len(t, enabledRulesEvent2, 0)
	})
}

// TestNotificationHistory_Create 测试通知历史记录的创建
// 此测试验证可以记录通知发送尝试的历史记录
// 测试步骤：
// 1. 创建一个通知频道
// 2. 获取一个速度测试事件
// 3. 记录一条成功的通知尝试
// 4. 记录一条失败的通知尝试
// 5. 获取通知历史记录并验证记录数量
// 6. 验证成功和失败记录的属性
//
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestNotificationHistory_Create(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()
		_ = ctx

		// Create channel
		channel, err := td.Service.CreateChannel(NotificationChannelInput{
			Name:    "History Test Channel",
			URL:     "https://example.com/webhook",
			Enabled: boolPtr(true),
		})
		require.NoError(t, err)

		// Get event
		event, err := td.Service.GetEventByType(NotificationCategorySpeedtest, NotificationEventSpeedtestComplete)
		require.NoError(t, err)

		// Log notification attempts
		// Success
		payload := `{"message": "Test completed successfully"}`
		err = td.Service.LogNotification(channel.ID, event.ID, true, nil, &payload)
		require.NoError(t, err)

		// Failure
		errorMsg := "Failed to send notification: timeout"
		err = td.Service.LogNotification(channel.ID, event.ID, false, &errorMsg, &payload)
		require.NoError(t, err)

		// Get notification history
		history, err := td.Service.GetNotificationHistory(10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(history), 2)

		// Find our entries
		var successEntry, failEntry *NotificationHistory
		for i := range history {
			if history[i].ChannelID == channel.ID && history[i].EventID == event.ID {
				if history[i].Success {
					successEntry = &history[i]
				} else {
					failEntry = &history[i]
				}
			}
		}

		require.NotNil(t, successEntry)
		require.NotNil(t, failEntry)

		assert.True(t, successEntry.Success)
		assert.Nil(t, successEntry.ErrorMessage)
		assert.NotNil(t, successEntry.Payload)

		assert.False(t, failEntry.Success)
		assert.NotNil(t, failEntry.ErrorMessage)
		assert.Equal(t, errorMsg, *failEntry.ErrorMessage)
	})
}

// TestNotification_CheckThreshold 测试通知阈值检查功能
// 此测试验证不同阈值运算符的正确性
// 测试步骤：
// 1. 定义多个测试用例，包括不同的运算符、阈值和值
// 2. 遍历所有测试用例，验证CheckThreshold函数的返回结果是否符合预期
// 3. 测试无阈值的情况，验证是否始终返回true
//
// 测试的运算符包括：gt(大于)、lt(小于)、eq(等于)、gte(大于等于)、lte(小于等于)
// 该测试在SQLite和PostgreSQL两种数据库上运行
func TestNotification_CheckThreshold(t *testing.T) {
	RunTestWithBothDatabases(t, func(t *testing.T, td *TestDatabase) {
		ctx := context.Background()
		_ = ctx

		testCases := []struct {
			name      string
			operator  string
			threshold float64
			value     float64
			expected  bool
		}{
			{"Greater Than - True", "gt", 100.0, 150.0, true},
			{"Greater Than - False", "gt", 100.0, 50.0, false},
			{"Less Than - True", "lt", 100.0, 50.0, true},
			{"Less Than - False", "lt", 100.0, 150.0, false},
			{"Equal - True", "eq", 100.0, 100.0, true},
			{"Equal - False", "eq", 100.0, 99.0, false},
			{"Greater or Equal - True (Greater)", "gte", 100.0, 150.0, true},
			{"Greater or Equal - True (Equal)", "gte", 100.0, 100.0, true},
			{"Greater or Equal - False", "gte", 100.0, 99.0, false},
			{"Less or Equal - True (Less)", "lte", 100.0, 50.0, true},
			{"Less or Equal - True (Equal)", "lte", 100.0, 100.0, true},
			{"Less or Equal - False", "lte", 100.0, 101.0, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				rule := &NotificationRule{
					ThresholdValue:    &tc.threshold,
					ThresholdOperator: &tc.operator,
				}

				result := td.Service.CheckThreshold(rule, tc.value)
				assert.Equal(t, tc.expected, result)
			})
		}

		// Test with nil threshold - should always return true
		ruleNoThreshold := &NotificationRule{
			ThresholdValue:    nil,
			ThresholdOperator: nil,
		}
		assert.True(t, td.Service.CheckThreshold(ruleNoThreshold, 999.0))
	})
}

// float64Ptr 创建一个float64类型的指针
// 参数：
//
//	f: 要创建指针的float64值
//
// 返回值：
//
//	*float64: 指向f的指针
//
// 这是一个辅助函数，用于在测试中创建float64类型的指针
func float64Ptr(f float64) *float64 {
	return &f
}
