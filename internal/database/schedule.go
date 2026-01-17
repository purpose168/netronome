// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 此文件包含调度系统的数据库操作接口实现
// 提供了对schedules表的CRUD操作
// 支持JSON数据的序列化和反序列化

package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	"github.com/autobrr/netronome/internal/types"
)

// CreateSchedule 创建新的调度
// 参数：
//
//	ctx - 上下文，用于控制请求的生命周期
//	schedule - 调度信息结构体，包含服务器ID、间隔、下次运行时间等
//
// 返回值：
//
//	*types.Schedule - 创建后的调度信息，包含自动生成的ID和创建时间
//	error - 操作错误，如输入无效、JSON序列化失败等
//
// 实现细节：
//   - 验证服务器ID不为空
//   - 将服务器ID和选项序列化为JSON字符串
//   - 使用RETURNING子句获取自动生成的ID和创建时间
func (s *service) CreateSchedule(ctx context.Context, schedule types.Schedule) (*types.Schedule, error) {
	if len(schedule.ServerIDs) == 0 {
		return nil, fmt.Errorf("%w: server IDs required", ErrInvalidInput)
	}

	serverIDs, err := json.Marshal(schedule.ServerIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal server IDs: %w", err)
	}

	options, err := json.Marshal(schedule.Options)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal options: %w", err)
	}

	data := map[string]interface{}{
		"server_ids": string(serverIDs),
		"interval":   schedule.Interval,
		"next_run":   schedule.NextRun,
		"enabled":    schedule.Enabled,
		"options":    string(options),
		"created_at": sq.Expr("CURRENT_TIMESTAMP"),
	}

	query := s.sqlBuilder.
		Insert("schedules").
		SetMap(data).
		Suffix("RETURNING id, created_at")

	err = query.RunWith(s.db).QueryRowContext(ctx).Scan(&schedule.ID, &schedule.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}

	return &schedule, nil
}

// GetSchedules 获取所有调度
// 参数：
//
//	ctx - 上下文，用于控制请求的生命周期
//
// 返回值：
//
//	[]types.Schedule - 所有调度的列表，按创建时间降序排列
//	error - 操作错误，如查询失败、JSON反序列化失败等
//
// 实现细节：
//   - 从schedules表中查询所有字段
//   - 将JSON字符串反序列化为服务器ID和选项
//   - 处理可能的空值（如last_run）
func (s *service) GetSchedules(ctx context.Context) ([]types.Schedule, error) {
	query := s.sqlBuilder.
		Select(
			"id",
			"server_ids",
			"interval",
			"last_run",
			"next_run",
			"enabled",
			"options",
			"created_at",
		).
		From("schedules").
		OrderBy("created_at DESC")

	rows, err := query.RunWith(s.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedules: %w", err)
	}
	defer rows.Close()

	var schedules []types.Schedule
	for rows.Next() {
		var schedule types.Schedule
		var serverIDsJSON, optionsJSON string
		var lastRun sql.NullTime

		err := rows.Scan(
			&schedule.ID,
			&serverIDsJSON,
			&schedule.Interval,
			&lastRun,
			&schedule.NextRun,
			&schedule.Enabled,
			&optionsJSON,
			&schedule.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %w", err)
		}

		if lastRun.Valid {
			schedule.LastRun = &lastRun.Time
		}

		if err := json.Unmarshal([]byte(serverIDsJSON), &schedule.ServerIDs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal server IDs: %w", err)
		}

		if err := json.Unmarshal([]byte(optionsJSON), &schedule.Options); err != nil {
			return nil, fmt.Errorf("failed to unmarshal options: %w", err)
		}

		schedules = append(schedules, schedule)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating schedules: %w", err)
	}

	return schedules, nil
}

// UpdateSchedule 更新调度信息
// 参数：
//
//	ctx - 上下文，用于控制请求的生命周期
//	schedule - 调度信息结构体，包含ID和更新后的信息
//
// 返回值：
//
//	error - 操作错误，如ID无效、服务器ID为空、更新失败等
//
// 实现细节：
//   - 验证ID和服务器ID的有效性
//   - 将服务器ID和选项序列化为JSON字符串
//   - 更新schedules表中的记录
//   - 验证是否有记录被更新（ID是否存在）
func (s *service) UpdateSchedule(ctx context.Context, schedule types.Schedule) error {
	if schedule.ID <= 0 {
		return fmt.Errorf("%w: invalid schedule ID", ErrInvalidInput)
	}

	if len(schedule.ServerIDs) == 0 {
		return fmt.Errorf("%w: server IDs required", ErrInvalidInput)
	}

	serverIDs, err := json.Marshal(schedule.ServerIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal server IDs: %w", err)
	}

	options, err := json.Marshal(schedule.Options)
	if err != nil {
		return fmt.Errorf("failed to marshal options: %w", err)
	}

	data := map[string]interface{}{
		"server_ids": string(serverIDs),
		"interval":   schedule.Interval,
		"next_run":   schedule.NextRun,
		"enabled":    schedule.Enabled,
		"options":    string(options),
	}

	query := s.sqlBuilder.
		Update("schedules").
		SetMap(data).
		Where(sq.Eq{"id": schedule.ID})

	result, err := query.RunWith(s.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to update schedule: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteSchedule 删除指定ID的调度
// 参数：
//
//	ctx - 上下文，用于控制请求的生命周期
//	id - 要删除的调度ID
//
// 返回值：
//
//	error - 操作错误，如ID无效、删除失败等
//
// 实现细节：
//   - 验证ID的有效性
//   - 从schedules表中删除指定ID的记录
//   - 验证是否有记录被删除（ID是否存在）
func (s *service) DeleteSchedule(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: invalid schedule ID", ErrInvalidInput)
	}

	query := s.sqlBuilder.
		Delete("schedules").
		Where(sq.Eq{"id": id})

	result, err := query.RunWith(s.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return ErrNotFound
	}

	return nil
}
