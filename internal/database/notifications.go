// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 此文件包含通知系统的核心数据库操作实现
// 包括通知频道、通知事件、通知规则和通知历史记录的CRUD操作
package database

import (
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/autobrr/netronome/internal/config"
)

// CreateChannel 创建一个新的通知频道
// 参数：
//
//	input: 通知频道输入结构体，包含频道名称、URL和启用状态
//
// 返回值：
//
//	*NotificationChannel: 创建成功的通知频道，包含自动生成的ID和时间戳
//	error: 如果创建过程中发生错误，返回错误信息
//
// 该函数会根据配置的数据库类型（PostgreSQL或SQLite）使用不同的插入方式
// PostgreSQL使用RETURNING子句获取插入的ID，SQLite使用LastInsertId方法
func (s *service) CreateChannel(input NotificationChannelInput) (*NotificationChannel, error) {
	now := time.Now()
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	query := s.sqlBuilder.Insert("notification_channels").
		Columns("name", "url", "enabled", "created_at", "updated_at").
		Values(input.Name, input.URL, enabled, now, now)

	if s.config.Type == config.Postgres {
		query = query.Suffix("RETURNING id")
	}

	if s.config.Type == config.SQLite {
		result, err := query.RunWith(s.db).Exec()
		if err != nil {
			return nil, fmt.Errorf("failed to create notification channel: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("failed to get last insert id: %w", err)
		}

		return &NotificationChannel{
			ID:        id,
			Name:      input.Name,
			URL:       input.URL,
			Enabled:   enabled,
			CreatedAt: now,
			UpdatedAt: now,
		}, nil
	} else {
		// PostgreSQL
		var id int64
		err := query.RunWith(s.db).QueryRow().Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("failed to create notification channel: %w", err)
		}

		return &NotificationChannel{
			ID:        id,
			Name:      input.Name,
			URL:       input.URL,
			Enabled:   enabled,
			CreatedAt: now,
			UpdatedAt: now,
		}, nil
	}
}

// GetChannels 获取所有通知频道
// 返回值：
//
//	[]NotificationChannel: 所有通知频道的切片，按创建时间降序排列
//	error: 如果获取过程中发生错误，返回错误信息
//
// 该函数查询数据库中的所有通知频道，并将结果映射到NotificationChannel结构体切片
func (s *service) GetChannels() ([]NotificationChannel, error) {
	var channels []NotificationChannel

	rows, err := s.sqlBuilder.Select("id", "name", "url", "enabled", "created_at", "updated_at").
		From("notification_channels").
		OrderBy("created_at DESC").
		RunWith(s.db).
		Query()
	if err != nil {
		return nil, fmt.Errorf("failed to get notification channels: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var channel NotificationChannel
		if err := rows.Scan(&channel.ID, &channel.Name, &channel.URL, &channel.Enabled, &channel.CreatedAt, &channel.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan channel: %w", err)
		}
		channels = append(channels, channel)
	}

	return channels, nil
}

// GetEnabledChannels 获取所有启用的通知频道
// 返回值：
//
//	[]NotificationChannel: 所有启用的通知频道的切片，按创建时间降序排列
//	error: 如果获取过程中发生错误，返回错误信息
//
// 该函数查询数据库中所有启用状态的通知频道
func (s *service) GetEnabledChannels() ([]NotificationChannel, error) {
	var channels []NotificationChannel

	rows, err := s.sqlBuilder.Select("id", "name", "url", "enabled", "created_at", "updated_at").
		From("notification_channels").
		Where(sq.Eq{"enabled": true}).
		OrderBy("created_at DESC").
		RunWith(s.db).
		Query()
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled notification channels: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var channel NotificationChannel
		if err := rows.Scan(&channel.ID, &channel.Name, &channel.URL, &channel.Enabled, &channel.CreatedAt, &channel.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan channel: %w", err)
		}
		channels = append(channels, channel)
	}

	return channels, nil
}

// GetChannel 根据ID获取通知频道
// 参数：
//
//	id: 通知频道的唯一标识符
//
// 返回值：
//
//	*NotificationChannel: 获取到的通知频道
//	error: 如果获取过程中发生错误，返回错误信息；如果未找到频道，返回ErrNotFound
//
// 该函数通过ID在数据库中查找通知频道，并将结果映射到NotificationChannel结构体
func (s *service) GetChannel(id int64) (*NotificationChannel, error) {
	var channel NotificationChannel

	err := s.sqlBuilder.Select("id", "name", "url", "enabled", "created_at", "updated_at").
		From("notification_channels").
		Where(sq.Eq{"id": id}).
		RunWith(s.db).
		QueryRow().
		Scan(&channel.ID, &channel.Name, &channel.URL, &channel.Enabled, &channel.CreatedAt, &channel.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get notification channel: %w", err)
	}

	return &channel, nil
}

// UpdateChannel 更新通知频道
// 参数：
//
//	id: 要更新的通知频道的唯一标识符
//	input: 通知频道输入结构体，包含要更新的频道名称、URL和启用状态
//
// 返回值：
//
//	*NotificationChannel: 更新后的通知频道
//	error: 如果更新过程中发生错误，返回错误信息；如果未找到频道，返回ErrNotFound
//
// 该函数会更新指定ID的通知频道，并返回更新后的频道信息
func (s *service) UpdateChannel(id int64, input NotificationChannelInput) (*NotificationChannel, error) {
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	result, err := s.sqlBuilder.Update("notification_channels").
		Set("name", input.Name).
		Set("url", input.URL).
		Set("enabled", enabled).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": id}).
		RunWith(s.db).
		Exec()

	if err != nil {
		return nil, fmt.Errorf("failed to update notification channel: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return s.GetChannel(id)
}

// DeleteChannel 删除通知频道
// 参数：
//
//	id: 要删除的通知频道的唯一标识符
//
// 返回值：
//
//	error: 如果删除过程中发生错误，返回错误信息；如果未找到频道，返回ErrNotFound
//
// 该函数会删除指定ID的通知频道
func (s *service) DeleteChannel(id int64) error {
	result, err := s.sqlBuilder.Delete("notification_channels").
		Where(sq.Eq{"id": id}).
		RunWith(s.db).
		Exec()

	if err != nil {
		return fmt.Errorf("failed to delete notification channel: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// GetEvents 获取所有通知事件
// 返回值：
//
//	[]NotificationEvent: 所有通知事件的切片，按类别和名称排序
//	error: 如果获取过程中发生错误，返回错误信息
//
// 该函数查询数据库中的所有通知事件，并将结果映射到NotificationEvent结构体切片
func (s *service) GetEvents() ([]NotificationEvent, error) {
	var events []NotificationEvent

	rows, err := s.sqlBuilder.Select("id", "category", "event_type", "name", "description", "default_enabled", "supports_threshold", "threshold_unit", "created_at").
		From("notification_events").
		OrderBy("category", "name").
		RunWith(s.db).
		Query()
	if err != nil {
		return nil, fmt.Errorf("failed to get notification events: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var event NotificationEvent
		var description, thresholdUnit sql.NullString

		if err := rows.Scan(&event.ID, &event.Category, &event.EventType, &event.Name, &description, &event.DefaultEnabled, &event.SupportsThreshold, &thresholdUnit, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if description.Valid {
			event.Description = &description.String
		}
		if thresholdUnit.Valid {
			event.ThresholdUnit = &thresholdUnit.String
		}

		events = append(events, event)
	}

	return events, nil
}

// GetEventsByCategory 根据类别获取通知事件
// 参数：
//
//	category: 通知事件的类别（如speedtest、packetloss、agent）
//
// 返回值：
//
//	[]NotificationEvent: 指定类别的通知事件切片，按名称排序
//	error: 如果获取过程中发生错误，返回错误信息
//
// 该函数查询数据库中指定类别的所有通知事件
func (s *service) GetEventsByCategory(category string) ([]NotificationEvent, error) {
	var events []NotificationEvent

	rows, err := s.sqlBuilder.Select("id", "category", "event_type", "name", "description", "default_enabled", "supports_threshold", "threshold_unit", "created_at").
		From("notification_events").
		Where(sq.Eq{"category": category}).
		OrderBy("name").
		RunWith(s.db).
		Query()
	if err != nil {
		return nil, fmt.Errorf("failed to get notification events: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var event NotificationEvent
		var description, thresholdUnit sql.NullString

		if err := rows.Scan(&event.ID, &event.Category, &event.EventType, &event.Name, &description, &event.DefaultEnabled, &event.SupportsThreshold, &thresholdUnit, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if description.Valid {
			event.Description = &description.String
		}
		if thresholdUnit.Valid {
			event.ThresholdUnit = &thresholdUnit.String
		}

		events = append(events, event)
	}

	return events, nil
}

// GetEvent 根据ID获取通知事件
// 参数：
//
//	id: 通知事件的唯一标识符
//
// 返回值：
//
//	*NotificationEvent: 获取到的通知事件
//	error: 如果获取过程中发生错误，返回错误信息；如果未找到事件，返回ErrNotFound
//
// 该函数通过ID在数据库中查找通知事件，并将结果映射到NotificationEvent结构体
func (s *service) GetEvent(id int64) (*NotificationEvent, error) {
	var event NotificationEvent
	var description, thresholdUnit sql.NullString

	err := s.sqlBuilder.Select("id", "category", "event_type", "name", "description", "default_enabled", "supports_threshold", "threshold_unit", "created_at").
		From("notification_events").
		Where(sq.Eq{"id": id}).
		RunWith(s.db).
		QueryRow().
		Scan(&event.ID, &event.Category, &event.EventType, &event.Name, &description, &event.DefaultEnabled, &event.SupportsThreshold, &thresholdUnit, &event.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get notification event: %w", err)
	}

	if description.Valid {
		event.Description = &description.String
	}
	if thresholdUnit.Valid {
		event.ThresholdUnit = &thresholdUnit.String
	}

	return &event, nil
}

// GetEventByType 根据类别和类型获取通知事件
// 参数：
//
//	category: 通知事件的类别
//	eventType: 通知事件的类型
//
// 返回值：
//
//	*NotificationEvent: 获取到的通知事件
//	error: 如果获取过程中发生错误，返回错误信息；如果未找到事件，返回ErrNotFound
//
// 该函数通过类别和类型在数据库中查找通知事件
func (s *service) GetEventByType(category, eventType string) (*NotificationEvent, error) {
	var event NotificationEvent
	var description, thresholdUnit sql.NullString

	err := s.sqlBuilder.Select("id", "category", "event_type", "name", "description", "default_enabled", "supports_threshold", "threshold_unit", "created_at").
		From("notification_events").
		Where(sq.And{
			sq.Eq{"category": category},
			sq.Eq{"event_type": eventType},
		}).
		RunWith(s.db).
		QueryRow().
		Scan(&event.ID, &event.Category, &event.EventType, &event.Name, &description, &event.DefaultEnabled, &event.SupportsThreshold, &thresholdUnit, &event.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get notification event: %w", err)
	}

	if description.Valid {
		event.Description = &description.String
	}
	if thresholdUnit.Valid {
		event.ThresholdUnit = &thresholdUnit.String
	}

	return &event, nil
}

// CreateRule 创建一个新的通知规则
// 参数：
//
//	input: 通知规则输入结构体，包含频道ID、事件ID、启用状态、阈值和阈值运算符
//
// 返回值：
//
//	*NotificationRule: 创建成功的通知规则，包含自动生成的ID和时间戳
//	error: 如果创建过程中发生错误，返回错误信息
//
// 该函数会根据配置的数据库类型（PostgreSQL或SQLite）使用不同的插入方式
// PostgreSQL使用RETURNING子句获取插入的ID，SQLite使用LastInsertId方法
func (s *service) CreateRule(input NotificationRuleInput) (*NotificationRule, error) {
	now := time.Now()
	enabled := false
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	query := s.sqlBuilder.Insert("notification_rules").
		Columns("channel_id", "event_id", "enabled", "threshold_value", "threshold_operator", "created_at", "updated_at").
		Values(input.ChannelID, input.EventID, enabled, input.ThresholdValue, input.ThresholdOperator, now, now)

	if s.config.Type == config.Postgres {
		query = query.Suffix("RETURNING id")
	}

	if s.config.Type == config.SQLite {
		result, err := query.RunWith(s.db).Exec()
		if err != nil {
			return nil, fmt.Errorf("failed to create notification rule: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("failed to get last insert id: %w", err)
		}

		return &NotificationRule{
			ID:                id,
			ChannelID:         input.ChannelID,
			EventID:           input.EventID,
			Enabled:           enabled,
			ThresholdValue:    input.ThresholdValue,
			ThresholdOperator: input.ThresholdOperator,
			CreatedAt:         now,
			UpdatedAt:         now,
		}, nil
	} else {
		// PostgreSQL
		var id int64
		err := query.RunWith(s.db).QueryRow().Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("failed to create notification rule: %w", err)
		}

		return &NotificationRule{
			ID:                id,
			ChannelID:         input.ChannelID,
			EventID:           input.EventID,
			Enabled:           enabled,
			ThresholdValue:    input.ThresholdValue,
			ThresholdOperator: input.ThresholdOperator,
			CreatedAt:         now,
			UpdatedAt:         now,
		}, nil
	}
}

// GetRules 获取所有通知规则
// 返回值：
//
//	[]NotificationRule: 所有通知规则的切片，按创建时间降序排列
//	error: 如果获取过程中发生错误，返回错误信息
//
// 该函数查询数据库中的所有通知规则，并关联查询对应的频道和事件信息
func (s *service) GetRules() ([]NotificationRule, error) {
	var rules []NotificationRule

	rows, err := s.sqlBuilder.Select(
		"r.id", "r.channel_id", "r.event_id", "r.enabled", "r.threshold_value", "r.threshold_operator", "r.created_at", "r.updated_at",
		"c.id", "c.name", "c.url", "c.enabled", "c.created_at", "c.updated_at",
		"e.id", "e.category", "e.event_type", "e.name", "e.description", "e.default_enabled", "e.supports_threshold", "e.threshold_unit", "e.created_at",
	).
		From("notification_rules r").
		LeftJoin("notification_channels c ON r.channel_id = c.id").
		LeftJoin("notification_events e ON r.event_id = e.id").
		OrderBy("r.created_at DESC").
		RunWith(s.db).
		Query()
	if err != nil {
		return nil, fmt.Errorf("failed to get notification rules: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rule NotificationRule
		var channel NotificationChannel
		var event NotificationEvent
		var thresholdValue sql.NullFloat64
		var thresholdOperator, eventDescription, eventThresholdUnit sql.NullString

		err := rows.Scan(
			&rule.ID, &rule.ChannelID, &rule.EventID, &rule.Enabled, &thresholdValue, &thresholdOperator, &rule.CreatedAt, &rule.UpdatedAt,
			&channel.ID, &channel.Name, &channel.URL, &channel.Enabled, &channel.CreatedAt, &channel.UpdatedAt,
			&event.ID, &event.Category, &event.EventType, &event.Name, &eventDescription, &event.DefaultEnabled, &event.SupportsThreshold, &eventThresholdUnit, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}

		if thresholdValue.Valid {
			rule.ThresholdValue = &thresholdValue.Float64
		}
		if thresholdOperator.Valid {
			rule.ThresholdOperator = &thresholdOperator.String
		}
		if eventDescription.Valid {
			event.Description = &eventDescription.String
		}
		if eventThresholdUnit.Valid {
			event.ThresholdUnit = &eventThresholdUnit.String
		}

		rule.Channel = &channel
		rule.Event = &event

		rules = append(rules, rule)
	}

	return rules, nil
}

// GetRulesByChannel 根据频道ID获取通知规则
// 参数：
//
//	channelID: 通知频道的唯一标识符
//
// 返回值：
//
//	[]NotificationRule: 指定频道的通知规则切片，按事件类别和名称排序
//	error: 如果获取过程中发生错误，返回错误信息
//
// 该函数查询数据库中指定频道的所有通知规则，并关联查询对应的事件信息
func (s *service) GetRulesByChannel(channelID int64) ([]NotificationRule, error) {
	var rules []NotificationRule

	rows, err := s.sqlBuilder.Select(
		"r.id", "r.channel_id", "r.event_id", "r.enabled", "r.threshold_value", "r.threshold_operator", "r.created_at", "r.updated_at",
		"e.id", "e.category", "e.event_type", "e.name", "e.description", "e.default_enabled", "e.supports_threshold", "e.threshold_unit", "e.created_at",
	).
		From("notification_rules r").
		LeftJoin("notification_events e ON r.event_id = e.id").
		Where(sq.Eq{"r.channel_id": channelID}).
		OrderBy("e.category", "e.name").
		RunWith(s.db).
		Query()
	if err != nil {
		return nil, fmt.Errorf("failed to get notification rules: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rule NotificationRule
		var event NotificationEvent
		var thresholdValue sql.NullFloat64
		var thresholdOperator, eventDescription, eventThresholdUnit sql.NullString

		err := rows.Scan(
			&rule.ID, &rule.ChannelID, &rule.EventID, &rule.Enabled, &thresholdValue, &thresholdOperator, &rule.CreatedAt, &rule.UpdatedAt,
			&event.ID, &event.Category, &event.EventType, &event.Name, &eventDescription, &event.DefaultEnabled, &event.SupportsThreshold, &eventThresholdUnit, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}

		if thresholdValue.Valid {
			rule.ThresholdValue = &thresholdValue.Float64
		}
		if thresholdOperator.Valid {
			rule.ThresholdOperator = &thresholdOperator.String
		}
		if eventDescription.Valid {
			event.Description = &eventDescription.String
		}
		if eventThresholdUnit.Valid {
			event.ThresholdUnit = &eventThresholdUnit.String
		}

		rule.Event = &event

		rules = append(rules, rule)
	}

	return rules, nil
}

// UpdateRule 更新通知规则
// 参数：
//
//	id: 要更新的通知规则的唯一标识符
//	input: 通知规则输入结构体，包含要更新的启用状态、阈值和阈值运算符
//
// 返回值：
//
//	*NotificationRule: 更新后的通知规则
//	error: 如果更新过程中发生错误，返回错误信息；如果未找到规则，返回ErrNotFound
//
// 该函数会更新指定ID的通知规则，并返回更新后的规则信息
func (s *service) UpdateRule(id int64, input NotificationRuleInput) (*NotificationRule, error) {
	update := s.sqlBuilder.Update("notification_rules").
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": id})

	if input.Enabled != nil {
		update = update.Set("enabled", *input.Enabled)
	}
	if input.ThresholdValue != nil {
		update = update.Set("threshold_value", *input.ThresholdValue)
	}
	if input.ThresholdOperator != nil {
		update = update.Set("threshold_operator", *input.ThresholdOperator)
	}

	result, err := update.RunWith(s.db).Exec()
	if err != nil {
		return nil, fmt.Errorf("failed to update notification rule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	// Get the updated rule
	var rule NotificationRule
	err = s.sqlBuilder.Select("id", "channel_id", "event_id", "enabled", "threshold_value", "threshold_operator", "created_at", "updated_at").
		From("notification_rules").
		Where(sq.Eq{"id": id}).
		RunWith(s.db).
		QueryRow().
		Scan(&rule.ID, &rule.ChannelID, &rule.EventID, &rule.Enabled, &rule.ThresholdValue, &rule.ThresholdOperator, &rule.CreatedAt, &rule.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to get updated rule: %w", err)
	}

	return &rule, nil
}

// GetRule 根据ID获取通知规则
// 参数：
//
//	id: 通知规则的唯一标识符
//
// 返回值：
//
//	*NotificationRule: 获取到的通知规则
//	error: 如果获取过程中发生错误，返回错误信息；如果未找到规则，返回ErrNotFound
//
// 该函数通过ID在数据库中查找通知规则
func (s *service) GetRule(id int64) (*NotificationRule, error) {
	var rule NotificationRule
	var thresholdValue sql.NullFloat64
	var thresholdOperator sql.NullString

	err := s.sqlBuilder.Select("id", "channel_id", "event_id", "enabled", "threshold_value", "threshold_operator", "created_at", "updated_at").
		From("notification_rules").
		Where(sq.Eq{"id": id}).
		RunWith(s.db).
		QueryRow().
		Scan(&rule.ID, &rule.ChannelID, &rule.EventID, &rule.Enabled, &thresholdValue, &thresholdOperator, &rule.CreatedAt, &rule.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get notification rule: %w", err)
	}

	if thresholdValue.Valid {
		rule.ThresholdValue = &thresholdValue.Float64
	}
	if thresholdOperator.Valid {
		rule.ThresholdOperator = &thresholdOperator.String
	}

	return &rule, nil
}

// DeleteRule 删除通知规则
// 参数：
//
//	id: 要删除的通知规则的唯一标识符
//
// 返回值：
//
//	error: 如果删除过程中发生错误，返回错误信息；如果未找到规则，返回ErrNotFound
//
// 该函数会删除指定ID的通知规则
func (s *service) DeleteRule(id int64) error {
	result, err := s.sqlBuilder.Delete("notification_rules").
		Where(sq.Eq{"id": id}).
		RunWith(s.db).
		Exec()

	if err != nil {
		return fmt.Errorf("failed to delete notification rule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// GetRulesByEvent 根据事件ID获取通知规则
// 参数：
//
//	eventID: 通知事件的唯一标识符
//
// 返回值：
//
//	[]NotificationRule: 指定事件的通知规则切片，按创建时间降序排列
//	error: 如果获取过程中发生错误，返回错误信息
//
// 该函数查询数据库中指定事件的所有通知规则，并关联查询对应的频道信息
func (s *service) GetRulesByEvent(eventID int64) ([]NotificationRule, error) {
	var rules []NotificationRule

	rows, err := s.sqlBuilder.Select(
		"r.id", "r.channel_id", "r.event_id", "r.enabled", "r.threshold_value", "r.threshold_operator", "r.created_at", "r.updated_at",
		"c.id", "c.name", "c.url", "c.enabled", "c.created_at", "c.updated_at",
	).
		From("notification_rules r").
		LeftJoin("notification_channels c ON r.channel_id = c.id").
		Where(sq.Eq{"r.event_id": eventID}).
		OrderBy("r.created_at DESC").
		RunWith(s.db).
		Query()
	if err != nil {
		return nil, fmt.Errorf("failed to get notification rules by event: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rule NotificationRule
		var channel NotificationChannel
		var thresholdValue sql.NullFloat64
		var thresholdOperator sql.NullString

		err := rows.Scan(
			&rule.ID, &rule.ChannelID, &rule.EventID, &rule.Enabled, &thresholdValue, &thresholdOperator, &rule.CreatedAt, &rule.UpdatedAt,
			&channel.ID, &channel.Name, &channel.URL, &channel.Enabled, &channel.CreatedAt, &channel.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}

		if thresholdValue.Valid {
			rule.ThresholdValue = &thresholdValue.Float64
		}
		if thresholdOperator.Valid {
			rule.ThresholdOperator = &thresholdOperator.String
		}

		rule.Channel = &channel
		rules = append(rules, rule)
	}

	return rules, nil
}

// GetEnabledRulesForEvent 获取指定事件的所有启用通知规则
// 参数：
//
//	category: 通知事件的类别
//	eventType: 通知事件的类型
//
// 返回值：
//
//	[]NotificationRule: 指定事件的所有启用通知规则切片
//	error: 如果获取过程中发生错误，返回错误信息
//
// 该函数查询数据库中指定事件的所有启用通知规则，且关联的频道也必须是启用状态
func (s *service) GetEnabledRulesForEvent(category, eventType string) ([]NotificationRule, error) {
	var rules []NotificationRule

	rows, err := s.sqlBuilder.Select(
		"r.id", "r.channel_id", "r.event_id", "r.enabled", "r.threshold_value", "r.threshold_operator", "r.created_at", "r.updated_at",
		"c.id", "c.name", "c.url", "c.enabled", "c.created_at", "c.updated_at",
	).
		From("notification_rules r").
		Join("notification_channels c ON r.channel_id = c.id").
		Join("notification_events e ON r.event_id = e.id").
		Where(sq.And{
			sq.Eq{"r.enabled": true},
			sq.Eq{"c.enabled": true},
			sq.Eq{"e.category": category},
			sq.Eq{"e.event_type": eventType},
		}).
		RunWith(s.db).
		Query()
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled rules: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rule NotificationRule
		var channel NotificationChannel
		var thresholdValue sql.NullFloat64
		var thresholdOperator sql.NullString

		err := rows.Scan(
			&rule.ID, &rule.ChannelID, &rule.EventID, &rule.Enabled, &thresholdValue, &thresholdOperator, &rule.CreatedAt, &rule.UpdatedAt,
			&channel.ID, &channel.Name, &channel.URL, &channel.Enabled, &channel.CreatedAt, &channel.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}

		if thresholdValue.Valid {
			rule.ThresholdValue = &thresholdValue.Float64
		}
		if thresholdOperator.Valid {
			rule.ThresholdOperator = &thresholdOperator.String
		}

		rule.Channel = &channel
		rules = append(rules, rule)
	}

	return rules, nil
}

// LogNotification 记录通知发送尝试
// 参数：
//
//	channelID: 通知频道的唯一标识符
//	eventID: 通知事件的唯一标识符
//	success: 通知发送是否成功
//	errorMessage: 错误信息（如果发送失败）
//	payload: 发送的通知内容
//
// 返回值：
//
//	error: 如果记录过程中发生错误，返回错误信息
//
// 该函数会记录每次通知发送的尝试结果，包括成功与否、错误信息和发送内容
func (s *service) LogNotification(channelID, eventID int64, success bool, errorMessage, payload *string) error {
	_, err := s.sqlBuilder.Insert("notification_history").
		Columns("channel_id", "event_id", "success", "error_message", "payload", "created_at").
		Values(channelID, eventID, success, errorMessage, payload, time.Now()).
		RunWith(s.db).
		Exec()

	if err != nil {
		return fmt.Errorf("failed to log notification: %w", err)
	}

	return nil
}

// CheckThreshold 检查值是否满足阈值条件
// 参数：
//
//	rule: 通知规则，包含阈值和阈值运算符
//	value: 要检查的值
//
// 返回值：
//
//	bool: 如果值满足阈值条件，返回true；否则返回false
//
// 该函数支持以下阈值运算符：
//
//	gt: 大于
//	lt: 小于
//	eq: 等于
//	gte: 大于等于
//	lte: 小于等于
//
// 如果未配置阈值或运算符未知，默认返回true
func (s *service) CheckThreshold(rule *NotificationRule, value float64) bool {
	if rule.ThresholdValue == nil || rule.ThresholdOperator == nil {
		return true // No threshold configured, always pass
	}

	threshold := *rule.ThresholdValue
	operator := *rule.ThresholdOperator

	switch operator {
	case "gt":
		return value > threshold
	case "lt":
		return value < threshold
	case "eq":
		return value == threshold
	case "gte":
		return value >= threshold
	case "lte":
		return value <= threshold
	default:
		return true // Unknown operator, default to pass
	}
}

// GetNotificationHistory 获取通知历史记录
// 参数：
//
//	limit: 返回的历史记录数量限制，如果为0则返回所有记录
//
// 返回值：
//
//	[]NotificationHistory: 通知历史记录切片，按创建时间降序排列
//	error: 如果获取过程中发生错误，返回错误信息
//
// 该函数查询数据库中的通知历史记录，并关联查询对应的频道和事件信息
// 结果按创建时间降序排列，可以通过limit参数限制返回的记录数量
func (s *service) GetNotificationHistory(limit int) ([]NotificationHistory, error) {
	query := s.sqlBuilder.Select(
		"h.id", "h.channel_id", "h.event_id", "h.success", "h.error_message", "h.payload", "h.created_at",
		"c.name", "c.url",
		"e.category", "e.event_type", "e.name",
	).
		From("notification_history h").
		LeftJoin("notification_channels c ON h.channel_id = c.id").
		LeftJoin("notification_events e ON h.event_id = e.id").
		OrderBy("h.created_at DESC")

	if limit > 0 {
		query = query.Limit(uint64(limit))
	}

	rows, err := query.RunWith(s.db).Query()
	if err != nil {
		return nil, fmt.Errorf("failed to get notification history: %w", err)
	}
	defer rows.Close()

	var history []NotificationHistory
	for rows.Next() {
		var h NotificationHistory
		var errorMessage, payload sql.NullString
		var channelName, channelURL string
		var eventCategory, eventType, eventName string

		err := rows.Scan(
			&h.ID, &h.ChannelID, &h.EventID, &h.Success, &errorMessage, &payload, &h.CreatedAt,
			&channelName, &channelURL,
			&eventCategory, &eventType, &eventName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan history: %w", err)
		}

		if errorMessage.Valid {
			h.ErrorMessage = &errorMessage.String
		}
		if payload.Valid {
			h.Payload = &payload.String
		}

		// Optional: Add channel and event info to the history object
		// This would require extending the NotificationHistory struct

		history = append(history, h)
	}

	return history, nil
}
