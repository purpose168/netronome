// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 此文件包含丢包监控系统的数据库操作接口实现
// 提供了对packet_loss_monitors和packet_loss_results表的CRUD操作
// 支持PostgreSQL和SQLite数据库

package database

import (
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/netronome/internal/config"
	"github.com/autobrr/netronome/internal/types"
)

// GetPacketLossMonitor 根据ID获取丢包监控
// 参数：
//
//	monitorID - 丢包监控的ID
//
// 返回值：
//
//	*types.PacketLossMonitor - 丢包监控对象指针
//	error - 操作错误，如未找到返回ErrNotFound
func (s *service) GetPacketLossMonitor(monitorID int64) (*types.PacketLossMonitor, error) {
	query := s.sqlBuilder.
		Select("id", "host", "name", "interval", "packet_count", "enabled", "threshold", "last_run", "next_run", "last_state", "last_state_change", "created_at", "updated_at").
		From("packet_loss_monitors").
		Where(sq.Eq{"id": monitorID})

	monitor := &types.PacketLossMonitor{}
	err := query.RunWith(s.db).QueryRow().Scan(
		&monitor.ID,
		&monitor.Host,
		&monitor.Name,
		&monitor.Interval,
		&monitor.PacketCount,
		&monitor.Enabled,
		&monitor.Threshold,
		&monitor.LastRun,
		&monitor.NextRun,
		&monitor.LastState,
		&monitor.LastStateChange,
		&monitor.CreatedAt,
		&monitor.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get packet loss monitor: %w", err)
	}

	return monitor, nil
}

// GetEnabledPacketLossMonitors 获取所有启用的丢包监控
// 返回值：
//
//	[]*types.PacketLossMonitor - 启用的丢包监控列表
//	error - 操作错误
func (s *service) GetEnabledPacketLossMonitors() ([]*types.PacketLossMonitor, error) {
	query := s.sqlBuilder.
		Select("id", "host", "name", "interval", "packet_count", "enabled", "threshold", "last_run", "next_run", "last_state", "last_state_change", "created_at", "updated_at").
		From("packet_loss_monitors").
		Where(sq.Eq{"enabled": true}).
		OrderBy("created_at ASC")

	rows, err := query.RunWith(s.db).Query()
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled packet loss monitors: %w", err)
	}
	defer rows.Close()

	var monitors []*types.PacketLossMonitor
	for rows.Next() {
		monitor := &types.PacketLossMonitor{}
		err := rows.Scan(
			&monitor.ID,
			&monitor.Host,
			&monitor.Name,
			&monitor.Interval,
			&monitor.PacketCount,
			&monitor.Enabled,
			&monitor.Threshold,
			&monitor.LastRun,
			&monitor.NextRun,
			&monitor.LastState,
			&monitor.LastStateChange,
			&monitor.CreatedAt,
			&monitor.UpdatedAt,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan packet loss monitor")
			continue
		}
		monitors = append(monitors, monitor)
	}

	return monitors, nil
}

// SavePacketLossResult 保存丢包测试结果
// 参数：
//
//	result - 丢包测试结果对象指针，保存后会自动设置ID
//
// 返回值：
//
//	error - 操作错误
//
// 说明：根据数据库类型使用不同的方式获取插入ID
//   - PostgreSQL: 使用RETURNING子句
//   - SQLite: 使用LastInsertId方法
func (s *service) SavePacketLossResult(result *types.PacketLossResult) error {
	var id int64

	switch s.config.Type {
	case config.Postgres:
		query := s.sqlBuilder.
			Insert("packet_loss_results").
			Columns("monitor_id", "packet_loss", "min_rtt", "max_rtt", "avg_rtt", "std_dev_rtt", "packets_sent", "packets_recv", "used_mtr", "hop_count", "mtr_data", "privileged_mode", "created_at").
			Values(result.MonitorID, result.PacketLoss, result.MinRTT, result.MaxRTT, result.AvgRTT, result.StdDevRTT, result.PacketsSent, result.PacketsRecv, result.UsedMTR, result.HopCount, result.MTRData, result.PrivilegedMode, result.CreatedAt).
			Suffix("RETURNING id")

		sqlStr, args, err := query.ToSql()
		if err != nil {
			return fmt.Errorf("failed to build query: %w", err)
		}

		err = s.db.QueryRow(sqlStr, args...).Scan(&id)
		if err != nil {
			return fmt.Errorf("failed to save packet loss result: %w", err)
		}

	case config.SQLite:
		query := s.sqlBuilder.
			Insert("packet_loss_results").
			Columns("monitor_id", "packet_loss", "min_rtt", "max_rtt", "avg_rtt", "std_dev_rtt", "packets_sent", "packets_recv", "used_mtr", "hop_count", "mtr_data", "privileged_mode", "created_at").
			Values(result.MonitorID, result.PacketLoss, result.MinRTT, result.MaxRTT, result.AvgRTT, result.StdDevRTT, result.PacketsSent, result.PacketsRecv, result.UsedMTR, result.HopCount, result.MTRData, result.PrivilegedMode, result.CreatedAt)

		res, err := query.RunWith(s.db).Exec()
		if err != nil {
			return fmt.Errorf("failed to save packet loss result: %w", err)
		}

		id, err = res.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get last insert ID: %w", err)
		}

	default:
		return fmt.Errorf("unsupported database type: %s", s.config.Type)
	}

	result.ID = id
	return nil
}

// GetLatestPacketLossResult 获取指定监控的最新丢包测试结果
// 参数：
//
//	monitorID - 丢包监控的ID
//
// 返回值：
//
//	*types.PacketLossResult - 最新的丢包测试结果
//	error - 操作错误，如未找到返回ErrNotFound
func (s *service) GetLatestPacketLossResult(monitorID int64) (*types.PacketLossResult, error) {
	query := s.sqlBuilder.
		Select("id", "monitor_id", "packet_loss", "min_rtt", "max_rtt", "avg_rtt", "std_dev_rtt", "packets_sent", "packets_recv", "used_mtr", "hop_count", "mtr_data", "privileged_mode", "created_at").
		From("packet_loss_results").
		Where(sq.Eq{"monitor_id": monitorID}).
		OrderBy("created_at DESC").
		Limit(1)

	result := &types.PacketLossResult{}
	err := query.RunWith(s.db).QueryRow().Scan(
		&result.ID,
		&result.MonitorID,
		&result.PacketLoss,
		&result.MinRTT,
		&result.MaxRTT,
		&result.AvgRTT,
		&result.StdDevRTT,
		&result.PacketsSent,
		&result.PacketsRecv,
		&result.UsedMTR,
		&result.HopCount,
		&result.MTRData,
		&result.PrivilegedMode,
		&result.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest packet loss result: %w", err)
	}

	return result, nil
}

// CreatePacketLossMonitor 创建新的丢包监控
// 参数：
//
//	monitor - 丢包监控对象指针，创建时会自动设置创建时间和更新时间
//
// 返回值：
//
//	*types.PacketLossMonitor - 创建后的丢包监控对象，包含自动生成的ID
//	error - 操作错误
//
// 说明：根据数据库类型使用不同的方式获取插入ID
//   - PostgreSQL: 使用RETURNING子句
//   - SQLite: 使用LastInsertId方法
func (s *service) CreatePacketLossMonitor(monitor *types.PacketLossMonitor) (*types.PacketLossMonitor, error) {
	monitor.CreatedAt = time.Now()
	monitor.UpdatedAt = time.Now()

	query := s.sqlBuilder.
		Insert("packet_loss_monitors").
		Columns("host", "name", "interval", "packet_count", "enabled", "threshold", "created_at", "updated_at").
		Values(monitor.Host, monitor.Name, monitor.Interval, monitor.PacketCount, monitor.Enabled, monitor.Threshold, monitor.CreatedAt, monitor.UpdatedAt)

	if s.config.Type == config.Postgres {
		query = query.Suffix("RETURNING id")
		err := query.RunWith(s.db).QueryRow().Scan(&monitor.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to create packet loss monitor: %w", err)
		}
	} else {
		res, err := query.RunWith(s.db).Exec()
		if err != nil {
			return nil, fmt.Errorf("failed to create packet loss monitor: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("failed to get last insert ID: %w", err)
		}
		monitor.ID = id
	}

	return monitor, nil
}

// UpdatePacketLossMonitor 更新现有的丢包监控
// 参数：
//
//	monitor - 丢包监控对象指针，更新时会自动设置更新时间
//
// 返回值：
//
//	error - 操作错误，如未找到返回ErrNotFound
func (s *service) UpdatePacketLossMonitor(monitor *types.PacketLossMonitor) error {
	monitor.UpdatedAt = time.Now()

	data := map[string]interface{}{
		"host":         monitor.Host,
		"name":         monitor.Name,
		"interval":     monitor.Interval,
		"packet_count": monitor.PacketCount,
		"enabled":      monitor.Enabled,
		"threshold":    monitor.Threshold,
		"last_run":     monitor.LastRun,
		"next_run":     monitor.NextRun,
		"updated_at":   monitor.UpdatedAt,
	}

	query := s.sqlBuilder.
		Update("packet_loss_monitors").
		SetMap(data).
		Where(sq.Eq{"id": monitor.ID})

	res, err := query.RunWith(s.db).Exec()
	if err != nil {
		return fmt.Errorf("failed to update packet loss monitor: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeletePacketLossMonitor 删除丢包监控及其所有测试结果
// 参数：
//
//	monitorID - 丢包监控的ID
//
// 返回值：
//
//	error - 操作错误，如未找到返回ErrNotFound
//
// 说明：使用事务确保原子性操作，先删除结果，再删除监控
//
//	这样可以避免外键约束错误
func (s *service) DeletePacketLossMonitor(monitorID int64) error {
	// Start a transaction to delete both monitor and results atomically
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete results first (due to foreign key constraint)
	deleteResults := s.sqlBuilder.
		Delete("packet_loss_results").
		Where(sq.Eq{"monitor_id": monitorID})

	_, err = deleteResults.RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("failed to delete packet loss results: %w", err)
	}

	// Delete monitor
	deleteMonitor := s.sqlBuilder.
		Delete("packet_loss_monitors").
		Where(sq.Eq{"id": monitorID})

	res, err := deleteMonitor.RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("failed to delete packet loss monitor: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return tx.Commit()
}

// GetPacketLossMonitors 获取所有丢包监控
// 返回值：
//
//	[]*types.PacketLossMonitor - 所有丢包监控列表，按创建时间降序排列
//	error - 操作错误
func (s *service) GetPacketLossMonitors() ([]*types.PacketLossMonitor, error) {
	query := s.sqlBuilder.
		Select("id", "host", "name", "interval", "packet_count", "enabled", "threshold", "last_run", "next_run", "last_state", "last_state_change", "created_at", "updated_at").
		From("packet_loss_monitors").
		OrderBy("created_at DESC")

	rows, err := query.RunWith(s.db).Query()
	if err != nil {
		return nil, fmt.Errorf("failed to get packet loss monitors: %w", err)
	}
	defer rows.Close()

	var monitors []*types.PacketLossMonitor
	for rows.Next() {
		monitor := &types.PacketLossMonitor{}
		err := rows.Scan(
			&monitor.ID,
			&monitor.Host,
			&monitor.Name,
			&monitor.Interval,
			&monitor.PacketCount,
			&monitor.Enabled,
			&monitor.Threshold,
			&monitor.LastRun,
			&monitor.NextRun,
			&monitor.LastState,
			&monitor.LastStateChange,
			&monitor.CreatedAt,
			&monitor.UpdatedAt,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan packet loss monitor")
			continue
		}
		monitors = append(monitors, monitor)
	}

	return monitors, nil
}

// GetPacketLossResults 获取指定监控的丢包测试结果
// 参数：
//
//	monitorID - 丢包监控的ID
//	limit - 结果数量限制，0表示不限制
//
// 返回值：
//
//	[]*types.PacketLossResult - 丢包测试结果列表，按创建时间降序排列
//	error - 操作错误
func (s *service) GetPacketLossResults(monitorID int64, limit int) ([]*types.PacketLossResult, error) {
	query := s.sqlBuilder.
		Select("id", "monitor_id", "packet_loss", "min_rtt", "max_rtt", "avg_rtt", "std_dev_rtt", "packets_sent", "packets_recv", "used_mtr", "hop_count", "mtr_data", "privileged_mode", "created_at").
		From("packet_loss_results").
		Where(sq.Eq{"monitor_id": monitorID}).
		OrderBy("created_at DESC")

	if limit > 0 {
		query = query.Limit(uint64(limit))
	}

	rows, err := query.RunWith(s.db).Query()
	if err != nil {
		return nil, fmt.Errorf("failed to get packet loss results: %w", err)
	}
	defer rows.Close()

	var results []*types.PacketLossResult
	for rows.Next() {
		result := &types.PacketLossResult{}
		err := rows.Scan(
			&result.ID,
			&result.MonitorID,
			&result.PacketLoss,
			&result.MinRTT,
			&result.MaxRTT,
			&result.AvgRTT,
			&result.StdDevRTT,
			&result.PacketsSent,
			&result.PacketsRecv,
			&result.UsedMTR,
			&result.HopCount,
			&result.MTRData,
			&result.PrivilegedMode,
			&result.CreatedAt,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan packet loss result")
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// UpdatePacketLossMonitorState 更新丢包监控的状态和时间戳
// 参数：
//
//	monitorID - 丢包监控的ID
//	state - 新的状态（如"healthy", "degraded", "critical"）
//
// 返回值：
//
//	error - 操作错误，如未找到返回ErrNotFound
func (s *service) UpdatePacketLossMonitorState(monitorID int64, state string) error {
	query := s.sqlBuilder.
		Update("packet_loss_monitors").
		Set("last_state", state).
		Set("last_state_change", time.Now()).
		Where(sq.Eq{"id": monitorID})

	result, err := query.RunWith(s.db).Exec()
	if err != nil {
		return fmt.Errorf("failed to update packet loss monitor state: %w", err)
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
