// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 此文件包含通知系统的核心数据类型定义
// 包括结构体、常量和接口，用于通知系统的数据库操作
package database

import (
	"time"
)

// NotificationChannel 表示通知频道结构体
// 用于定义发送通知的目标渠道（如Discord、Slack等）
type NotificationChannel struct {
	ID        int64     `json:"id" db:"id"`                 // 通知频道的唯一标识符
	Name      string    `json:"name" db:"name"`             // 频道名称
	URL       string    `json:"url" db:"url"`               // 通知Webhook URL
	Enabled   bool      `json:"enabled" db:"enabled"`       // 频道是否启用
	CreatedAt time.Time `json:"created_at" db:"created_at"` // 创建时间戳
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"` // 更新时间戳
}

// NotificationEvent 表示通知事件结构体
// 定义系统中可以触发通知的各种事件类型
type NotificationEvent struct {
	ID                int64     `json:"id" db:"id"`                                 // 事件的唯一标识符
	Category          string    `json:"category" db:"category"`                     // 事件类别（如speedtest、packetloss、agent）
	EventType         string    `json:"event_type" db:"event_type"`                 // 事件类型（如complete、download_low、upload_low）
	Name              string    `json:"name" db:"name"`                             // 事件名称
	Description       *string   `json:"description" db:"description"`               // 事件描述（可选）
	DefaultEnabled    bool      `json:"default_enabled" db:"default_enabled"`       // 默认是否启用
	SupportsThreshold bool      `json:"supports_threshold" db:"supports_threshold"` // 是否支持阈值设置
	ThresholdUnit     *string   `json:"threshold_unit" db:"threshold_unit"`         // 阈值单位（如Mbps、%）
	CreatedAt         time.Time `json:"created_at" db:"created_at"`                 // 创建时间戳
}

// NotificationRule 表示通知规则结构体
// 定义事件与频道之间的关联关系及触发条件
type NotificationRule struct {
	ID                int64     `json:"id" db:"id"`                                 // 规则的唯一标识符
	ChannelID         int64     `json:"channel_id" db:"channel_id"`                 // 关联的频道ID
	EventID           int64     `json:"event_id" db:"event_id"`                     // 关联的事件ID
	Enabled           bool      `json:"enabled" db:"enabled"`                       // 规则是否启用
	ThresholdValue    *float64  `json:"threshold_value" db:"threshold_value"`       // 触发阈值（可选）
	ThresholdOperator *string   `json:"threshold_operator" db:"threshold_operator"` // 阈值运算符（可选：gt、lt、eq、gte、lte）
	CreatedAt         time.Time `json:"created_at" db:"created_at"`                 // 创建时间戳
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`                 // 更新时间戳

	// Joined fields for queries
	Channel *NotificationChannel `json:"channel,omitempty" db:"-"` // 查询时关联的频道信息
	Event   *NotificationEvent   `json:"event,omitempty" db:"-"`   // 查询时关联的事件信息
}

// NotificationHistory 表示通知历史记录结构体
// 记录每次通知发送的尝试结果
type NotificationHistory struct {
	ID           int64     `json:"id" db:"id"`                       // 历史记录的唯一标识符
	ChannelID    int64     `json:"channel_id" db:"channel_id"`       // 关联的频道ID
	EventID      int64     `json:"event_id" db:"event_id"`           // 关联的事件ID
	Success      bool      `json:"success" db:"success"`             // 通知发送是否成功
	ErrorMessage *string   `json:"error_message" db:"error_message"` // 错误信息（如果发送失败）
	Payload      *string   `json:"payload" db:"payload"`             // 发送的通知内容
	CreatedAt    time.Time `json:"created_at" db:"created_at"`       // 创建时间戳
}

// NotificationChannelInput 表示创建或更新通知频道的输入结构体
// 用于接收API请求中的频道数据
type NotificationChannelInput struct {
	Name    string `json:"name" validate:"required"` // 频道名称（必填）
	URL     string `json:"url" validate:"required"`  // 通知Webhook URL（必填）
	Enabled *bool  `json:"enabled"`                  // 频道是否启用（可选，默认true）
}

// NotificationRuleInput 表示创建或更新通知规则的输入结构体
// 用于接收API请求中的规则数据
type NotificationRuleInput struct {
	ChannelID         int64    `json:"channel_id" validate:"required"`                                 // 关联的频道ID（必填）
	EventID           int64    `json:"event_id" validate:"required"`                                   // 关联的事件ID（必填）
	Enabled           *bool    `json:"enabled"`                                                        // 规则是否启用（可选，默认true）
	ThresholdValue    *float64 `json:"threshold_value"`                                                // 触发阈值（可选）
	ThresholdOperator *string  `json:"threshold_operator" validate:"omitempty,oneof=gt lt eq gte lte"` // 阈值运算符（可选，只能是gt、lt、eq、gte、lte之一）
}

// NotificationEventCategory 通知事件类别常量
const (
	NotificationCategorySpeedtest  = "speedtest"  // 速度测试事件类别
	NotificationCategoryPacketLoss = "packetloss" // 丢包检测事件类别
	NotificationCategoryAgent      = "agent"      // 代理监控事件类别
)

// NotificationEventType 通知事件类型常量
const (
	// Speedtest events - 速度测试相关事件
	NotificationEventSpeedtestComplete    = "complete"     // 速度测试完成
	NotificationEventSpeedtestPingHigh    = "ping_high"    // 延迟过高
	NotificationEventSpeedtestDownloadLow = "download_low" // 下载速度过低
	NotificationEventSpeedtestUploadLow   = "upload_low"   // 上传速度过低
	NotificationEventSpeedtestFailed      = "failed"       // 速度测试失败

	// Packet loss events - 丢包检测相关事件
	NotificationEventPacketLossHigh      = "threshold_exceeded" // 丢包率超过阈值
	NotificationEventPacketLossDown      = "monitor_down"       // 监控目标下线
	NotificationEventPacketLossRecovered = "monitor_recovered"  // 监控目标恢复

	// Agent events - 代理监控相关事件
	NotificationEventAgentOffline       = "offline"          // 代理离线
	NotificationEventAgentOnline        = "online"           // 代理上线
	NotificationEventAgentHighBandwidth = "high_bandwidth"   // 带宽使用过高
	NotificationEventAgentLowDisk       = "disk_space_low"   // 磁盘空间过低
	NotificationEventAgentHighCPU       = "cpu_high"         // CPU使用率过高
	NotificationEventAgentHighMemory    = "memory_high"      // 内存使用率过高
	NotificationEventAgentHighTemp      = "temperature_high" // 温度过高
)

// ThresholdOperator 阈值运算符常量
const (
	ThresholdOperatorGT  = "gt"  // 大于 (greater than)
	ThresholdOperatorLT  = "lt"  // 小于 (less than)
	ThresholdOperatorEQ  = "eq"  // 等于 (equal)
	ThresholdOperatorGTE = "gte" // 大于等于 (greater than or equal)
	ThresholdOperatorLTE = "lte" // 小于等于 (less than or equal)
)

// NotificationService 表示通知系统的数据库操作接口
// 定义了所有与通知相关的数据库操作方法
type NotificationService interface {
	// Channels - 通知频道相关操作
	CreateChannel(input NotificationChannelInput) (*NotificationChannel, error)           // 创建新的通知频道
	GetChannel(id int64) (*NotificationChannel, error)                                    // 根据ID获取通知频道
	GetChannels() ([]NotificationChannel, error)                                          // 获取所有通知频道
	GetEnabledChannels() ([]NotificationChannel, error)                                   // 获取所有启用的通知频道
	UpdateChannel(id int64, input NotificationChannelInput) (*NotificationChannel, error) // 更新通知频道
	DeleteChannel(id int64) error                                                         // 删除通知频道

	// Events - 通知事件相关操作
	GetEvents() ([]NotificationEvent, error)                               // 获取所有通知事件
	GetEventsByCategory(category string) ([]NotificationEvent, error)      // 根据类别获取通知事件
	GetEvent(id int64) (*NotificationEvent, error)                         // 根据ID获取通知事件
	GetEventByType(category, eventType string) (*NotificationEvent, error) // 根据类别和类型获取通知事件

	// Rules - 通知规则相关操作
	CreateRule(input NotificationRuleInput) (*NotificationRule, error)              // 创建新的通知规则
	GetRule(id int64) (*NotificationRule, error)                                    // 根据ID获取通知规则
	GetRules() ([]NotificationRule, error)                                          // 获取所有通知规则
	GetRulesByChannel(channelID int64) ([]NotificationRule, error)                  // 根据频道ID获取通知规则
	GetRulesByEvent(eventID int64) ([]NotificationRule, error)                      // 根据事件ID获取通知规则
	GetEnabledRulesForEvent(category, eventType string) ([]NotificationRule, error) // 获取指定事件的所有启用规则
	UpdateRule(id int64, input NotificationRuleInput) (*NotificationRule, error)    // 更新通知规则
	DeleteRule(id int64) error                                                      // 删除通知规则

	// History - 通知历史相关操作
	LogNotification(channelID, eventID int64, success bool, errorMessage *string, payload *string) error // 记录通知发送历史
	GetNotificationHistory(limit int) ([]NotificationHistory, error)                                     // 获取通知历史记录（限制返回数量）

	// Utility - 工具方法
	CheckThreshold(rule *NotificationRule, value float64) bool // 检查值是否超过规则的阈值
}
