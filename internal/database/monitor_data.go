// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later

// database 包提供了与数据库交互的核心功能，包括监控数据的存储、检索和清理
// 本文件专注于监控数据的管理，包括系统信息、网络接口、峰值统计、资源使用和历史快照
package database

import (
	"context" // 上下文管理，用于控制goroutine和请求生命周期
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel" // SQL查询构建器，简化动态SQL生成
	"github.com/rs/zerolog/log"          // 结构化日志库

	"github.com/autobrr/netronome/internal/types" // 定义了所有数据结构
)

// UpsertMonitorSystemInfo 为代理插入或更新系统信息
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和取消操作
//	agentID: 代理的唯一标识符
//	info: 包含系统信息的结构体指针
//
// 返回值:
//
//	成功时返回nil，失败时返回错误信息
func (s *service) UpsertMonitorSystemInfo(ctx context.Context, agentID int64, info *types.MonitorSystemInfo) error {
	// 首先检查记录是否存在
	// 使用GetMonitorSystemInfo查询，通过ErrNotFound错误判断记录是否存在
	_, err := s.GetMonitorSystemInfo(ctx, agentID)
	if err != nil && err != ErrNotFound {
		return err // 如果是其他错误，直接返回
	}

	if err == ErrNotFound {
		// 插入新记录
		// 使用squirrel SQL构建器创建INSERT语句，确保类型安全
		query := s.sqlBuilder.
			Insert("monitor_agent_system_info").
			Columns("agent_id", "hostname", "kernel", "vnstat_version", "agent_version", "cpu_model", "cpu_cores", "cpu_threads", "total_memory", "created_at", "updated_at").
			Values(agentID, info.Hostname, info.Kernel, info.VnstatVersion, info.AgentVersion, info.CPUModel, info.CPUCores, info.CPUThreads, info.TotalMemory, time.Now(), time.Now())

		_, err := query.RunWith(s.db).ExecContext(ctx) // 执行查询并传入上下文
		return err
	}

	// 更新现有记录 - 仅更新非空字段
	// 这种策略可以避免覆盖已有的有效数据
	update := s.sqlBuilder.Update("monitor_agent_system_info").
		Set("updated_at", time.Now()).    // 每次更新都会更新时间戳
		Where(sq.Eq{"agent_id": agentID}) // 根据agent_id筛选要更新的记录

	hasUpdates := false // 标记是否有实际需要更新的字段

	// 仅当字段不为空时才更新
	// 对于字符串字段，检查是否为空字符串
	// 对于数值字段，检查是否大于0
	if info.Hostname != "" {
		update = update.Set("hostname", info.Hostname)
		hasUpdates = true
	}
	if info.Kernel != "" {
		update = update.Set("kernel", info.Kernel)
		hasUpdates = true
	}
	if info.VnstatVersion != "" {
		update = update.Set("vnstat_version", info.VnstatVersion)
		hasUpdates = true
	}
	if info.AgentVersion != nil && *info.AgentVersion != "" {
		update = update.Set("agent_version", info.AgentVersion)
		hasUpdates = true
	}
	if info.CPUModel != "" {
		update = update.Set("cpu_model", info.CPUModel)
		hasUpdates = true
	}
	if info.CPUCores > 0 {
		update = update.Set("cpu_cores", info.CPUCores)
		hasUpdates = true
	}
	if info.CPUThreads > 0 {
		update = update.Set("cpu_threads", info.CPUThreads)
		hasUpdates = true
	}
	if info.TotalMemory > 0 {
		update = update.Set("total_memory", info.TotalMemory)
		hasUpdates = true
	}

	if !hasUpdates {
		return nil // 没有需要更新的内容，直接返回成功
	}

	_, err = update.RunWith(s.db).ExecContext(ctx)
	return err
}

// GetMonitorSystemInfo 检索代理的系统信息
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和取消操作
//	agentID: 代理的唯一标识符
//
// 返回值:
//
//	成功时返回包含系统信息的结构体指针，失败时返回错误信息
//	如果记录不存在，返回ErrNotFound错误
func (s *service) GetMonitorSystemInfo(ctx context.Context, agentID int64) (*types.MonitorSystemInfo, error) {
	// 使用squirrel SQL构建器创建SELECT语句
	// 指定要查询的所有字段，确保与结构体字段顺序一致
	query := s.sqlBuilder.
		Select("id", "agent_id", "hostname", "kernel", "vnstat_version", "agent_version", "cpu_model", "cpu_cores", "cpu_threads", "total_memory", "created_at", "updated_at").
		From("monitor_agent_system_info").
		Where(sq.Eq{"agent_id": agentID}) // 根据agent_id筛选记录

	var info types.MonitorSystemInfo
	// 使用QueryRowContext执行查询，只返回一行结果
	// Scan方法将查询结果映射到结构体字段
	err := query.RunWith(s.db).QueryRowContext(ctx).Scan(
		&info.ID, &info.AgentID, &info.Hostname, &info.Kernel, &info.VnstatVersion, &info.AgentVersion,
		&info.CPUModel, &info.CPUCores, &info.CPUThreads, &info.TotalMemory,
		&info.CreatedAt, &info.UpdatedAt,
	)
	// 将sql.ErrNoRows转换为自定义的ErrNotFound错误，提高代码可读性
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return &info, err
}

// UpsertMonitorInterfaces 为代理插入或更新网络接口
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和取消操作
//	agentID: 代理的唯一标识符
//	interfaces: 包含网络接口信息的切片
//
// 返回值:
//
//	成功时返回nil，失败时返回错误信息
//
// 注意:
//
//	此函数使用事务确保数据一致性，要么全部成功，要么全部失败
func (s *service) UpsertMonitorInterfaces(ctx context.Context, agentID int64, interfaces []types.MonitorInterface) error {
	// 开始事务
	// BeginTx创建一个新事务，使用传入的上下文控制事务生命周期
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // 使用defer确保事务在函数返回时回滚，除非显式提交

	// 删除此代理的现有接口
	// 采用"先删后插"的策略来实现UPSERT操作
	deleteQuery := s.sqlBuilder.Delete("monitor_agent_interfaces").Where(sq.Eq{"agent_id": agentID})
	if _, err := deleteQuery.RunWith(tx).ExecContext(ctx); err != nil {
		return err // 如果删除失败，事务会通过defer自动回滚
	}

	// 插入新接口
	// 遍历所有接口，为每个接口创建INSERT语句
	for _, iface := range interfaces {
		insertQuery := s.sqlBuilder.
			Insert("monitor_agent_interfaces").
			Columns("agent_id", "name", "alias", "ip_address", "link_speed", "updated_at").
			Values(agentID, iface.Name, iface.Alias, iface.IPAddress, iface.LinkSpeed, time.Now())

		if _, err := insertQuery.RunWith(tx).ExecContext(ctx); err != nil {
			return err // 如果任何一个插入失败，事务会自动回滚
		}
	}

	// 提交事务
	// 如果所有操作都成功，提交事务以保存更改
	return tx.Commit()
}

// GetMonitorInterfaces 检索代理的网络接口
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和取消操作
//	agentID: 代理的唯一标识符
//
// 返回值:
//
//	成功时返回包含网络接口信息的切片，失败时返回错误信息
//	如果没有记录，返回空切片而不是错误
func (s *service) GetMonitorInterfaces(ctx context.Context, agentID int64) ([]types.MonitorInterface, error) {
	// 使用squirrel SQL构建器创建SELECT语句
	// OrderBy确保返回的接口按名称排序，提高可读性
	query := s.sqlBuilder.
		Select("id", "agent_id", "name", "alias", "ip_address", "link_speed", "created_at", "updated_at").
		From("monitor_agent_interfaces").
		Where(sq.Eq{"agent_id": agentID}).
		OrderBy("name")

	// 执行查询，返回多行结果
	rows, err := query.RunWith(s.db).QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close() // 使用defer确保结果集在函数返回时关闭

	var interfaces []types.MonitorInterface
	// 遍历所有结果行
	for rows.Next() {
		var iface types.MonitorInterface
		// 将行数据扫描到结构体中
		if err := rows.Scan(
			&iface.ID, &iface.AgentID, &iface.Name, &iface.Alias,
			&iface.IPAddress, &iface.LinkSpeed, &iface.CreatedAt, &iface.UpdatedAt,
		); err != nil {
			return nil, err
		}
		// 将扫描后的结构体添加到切片中
		interfaces = append(interfaces, iface)
	}

	// 检查遍历过程中是否发生错误
	return interfaces, rows.Err()
}

// UpsertMonitorPeakStats 为代理插入或更新峰值统计数据
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和取消操作
//	agentID: 代理的唯一标识符
//	stats: 包含峰值统计数据的结构体指针
//
// 返回值:
//
//	成功时返回nil，失败时返回错误信息
//
// 注意:
//
//	此函数仅在新峰值高于现有峰值时才会更新记录
func (s *service) UpsertMonitorPeakStats(ctx context.Context, agentID int64, stats *types.MonitorPeakStats) error {
	// 首先，获取现有峰值进行比较
	// 只有比较后才能确定是否需要更新
	existingStats, err := s.GetMonitorPeakStats(ctx, agentID)
	if err != nil && err != ErrNotFound {
		return err // 如果是其他错误，直接返回
	}

	// 如果没有现有统计数据，插入新数据
	if err == ErrNotFound {
		query := s.sqlBuilder.
			Insert("monitor_peak_stats").
			Columns("agent_id", "peak_rx_bytes", "peak_tx_bytes", "peak_rx_timestamp", "peak_tx_timestamp").
			Values(agentID, stats.PeakRxBytes, stats.PeakTxBytes, stats.PeakRxTimestamp, stats.PeakTxTimestamp)

		_, err := query.RunWith(s.db).ExecContext(ctx)
		return err
	}

	// 仅当新峰值更高时更新
	// 这种策略用于记录历史最高值
	needsUpdate := false // 标记是否需要更新

	// 比较接收方向的峰值
	if stats.PeakRxBytes > existingStats.PeakRxBytes {
		existingStats.PeakRxBytes = stats.PeakRxBytes
		existingStats.PeakRxTimestamp = stats.PeakRxTimestamp
		needsUpdate = true
	}
	// 比较发送方向的峰值
	if stats.PeakTxBytes > existingStats.PeakTxBytes {
		existingStats.PeakTxBytes = stats.PeakTxBytes
		existingStats.PeakTxTimestamp = stats.PeakTxTimestamp
		needsUpdate = true
	}

	// 如果需要更新，执行UPDATE语句
	if needsUpdate {
		query := s.sqlBuilder.
			Update("monitor_peak_stats").
			Set("peak_rx_bytes", existingStats.PeakRxBytes).
			Set("peak_tx_bytes", existingStats.PeakTxBytes).
			Set("peak_rx_timestamp", existingStats.PeakRxTimestamp).
			Set("peak_tx_timestamp", existingStats.PeakTxTimestamp).
			Where(sq.Eq{"agent_id": agentID})

		_, err = query.RunWith(s.db).ExecContext(ctx)
	}

	return err
}

// GetMonitorPeakStats 检索代理的峰值统计数据
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和取消操作
//	agentID: 代理的唯一标识符
//
// 返回值:
//
//	成功时返回包含峰值统计数据的结构体指针，失败时返回错误信息
//	如果记录不存在，返回ErrNotFound错误
func (s *service) GetMonitorPeakStats(ctx context.Context, agentID int64) (*types.MonitorPeakStats, error) {
	// 使用squirrel SQL构建器创建SELECT语句
	// OrderBy DESC确保返回最新的峰值数据
	// Limit 1确保只返回一行数据
	query := s.sqlBuilder.
		Select("id", "agent_id", "peak_rx_bytes", "peak_tx_bytes", "peak_rx_timestamp", "peak_tx_timestamp", "created_at").
		From("monitor_peak_stats").
		Where(sq.Eq{"agent_id": agentID}).
		OrderBy("created_at DESC").
		Limit(1)

	var stats types.MonitorPeakStats
	// 使用QueryRowContext执行查询，只返回一行结果
	// Scan方法将查询结果映射到结构体字段
	err := query.RunWith(s.db).QueryRowContext(ctx).Scan(
		&stats.ID, &stats.AgentID, &stats.PeakRxBytes, &stats.PeakTxBytes,
		&stats.PeakRxTimestamp, &stats.PeakTxTimestamp, &stats.CreatedAt,
	)
	// 将sql.ErrNoRows转换为自定义的ErrNotFound错误，提高代码可读性
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return &stats, err
}

// SaveMonitorResourceStats 保存代理的资源使用统计数据
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和取消操作
//	agentID: 代理的唯一标识符
//	stats: 包含资源使用统计数据的结构体指针
//
// 返回值:
//
//	成功时返回nil，失败时返回错误信息
//
// 注意:
//
//	此函数只做插入操作，不检查重复，用于记录资源使用的历史数据
func (s *service) SaveMonitorResourceStats(ctx context.Context, agentID int64, stats *types.MonitorResourceStats) error {
	// 使用squirrel SQL构建器创建INSERT语句
	// 资源使用数据采用只插入策略，便于后续查询历史趋势
	query := s.sqlBuilder.
		Insert("monitor_resource_stats").
		Columns("agent_id", "cpu_usage_percent", "memory_used_percent", "swap_used_percent", "disk_usage_json", "temperature_json", "uptime_seconds").
		Values(agentID, stats.CPUUsagePercent, stats.MemoryUsedPercent, stats.SwapUsedPercent, stats.DiskUsageJSON, stats.TemperatureJSON, stats.UptimeSeconds)

	_, err := query.RunWith(s.db).ExecContext(ctx)
	return err
}

// GetMonitorResourceStats 检索代理的资源统计数据
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和取消操作
//	agentID: 代理的唯一标识符
//	hours: 要检索的小时数，用于过滤时间范围
//
// 返回值:
//
//	成功时返回包含资源使用统计数据的切片，失败时返回错误信息
//	如果没有记录，返回空切片而不是错误
func (s *service) GetMonitorResourceStats(ctx context.Context, agentID int64, hours int) ([]types.MonitorResourceStats, error) {
	// 计算时间范围的起始时间
	// 使用当前时间减去指定的小时数
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	// 使用squirrel SQL构建器创建SELECT语句
	// 使用sq.And组合多个条件
	// 使用sq.GtOrEq过滤created_at大于等于指定时间的数据
	query := s.sqlBuilder.
		Select("id", "agent_id", "cpu_usage_percent", "memory_used_percent", "swap_used_percent", "disk_usage_json", "temperature_json", "uptime_seconds", "created_at").
		From("monitor_resource_stats").
		Where(sq.And{
			sq.Eq{"agent_id": agentID},     // 过滤特定代理
			sq.GtOrEq{"created_at": since}, // 过滤时间范围
		}).
		OrderBy("created_at DESC") // 按时间降序排列，最新数据在前

	// 执行查询，返回多行结果
	rows, err := query.RunWith(s.db).QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close() // 使用defer确保结果集在函数返回时关闭

	var stats []types.MonitorResourceStats
	// 遍历所有结果行
	for rows.Next() {
		var stat types.MonitorResourceStats
		// 将行数据扫描到结构体中
		if err := rows.Scan(
			&stat.ID, &stat.AgentID, &stat.CPUUsagePercent, &stat.MemoryUsedPercent,
			&stat.SwapUsedPercent, &stat.DiskUsageJSON, &stat.TemperatureJSON,
			&stat.UptimeSeconds, &stat.CreatedAt,
		); err != nil {
			return nil, err
		}
		// 将扫描后的结构体添加到切片中
		stats = append(stats, stat)
	}

	// 检查遍历过程中是否发生错误
	return stats, rows.Err()
}

// SaveMonitorHistoricalSnapshot 保存带宽监控数据快照
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和取消操作
//	agentID: 代理的唯一标识符
//	snapshot: 包含历史快照数据的结构体指针
//
// 返回值:
//
//	成功时返回nil，失败时返回错误信息
//
// 注意:
//
//	快照数据以JSON格式存储，便于灵活处理不同时期的带宽数据
func (s *service) SaveMonitorHistoricalSnapshot(ctx context.Context, agentID int64, snapshot *types.MonitorHistoricalSnapshot) error {
	// 使用squirrel SQL构建器创建INSERT语句
	// 历史快照数据以JSON格式存储在data_json字段中，提高存储和查询的灵活性
	query := s.sqlBuilder.
		Insert("monitor_historical_snapshots").
		Columns("agent_id", "interface_name", "period_type", "data_json").
		Values(agentID, snapshot.InterfaceName, snapshot.PeriodType, snapshot.DataJSON)

	_, err := query.RunWith(s.db).ExecContext(ctx)
	return err
}

// GetMonitorLatestSnapshot 检索代理的最新快照
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和取消操作
//	agentID: 代理的唯一标识符
//	periodType: 快照类型，用于过滤特定时期的数据
//
// 返回值:
//
//	成功时返回包含历史快照数据的结构体指针，失败时返回错误信息
//	如果记录不存在，返回ErrNotFound错误
func (s *service) GetMonitorLatestSnapshot(ctx context.Context, agentID int64, periodType string) (*types.MonitorHistoricalSnapshot, error) {
	// 使用squirrel SQL构建器创建SELECT语句
	// 使用sq.And组合多个条件
	// OrderBy DESC确保返回最新的快照数据
	// Limit 1确保只返回一行数据
	query := s.sqlBuilder.
		Select("id", "agent_id", "interface_name", "period_type", "data_json", "created_at").
		From("monitor_historical_snapshots").
		Where(sq.And{
			sq.Eq{"agent_id": agentID},       // 过滤特定代理
			sq.Eq{"period_type": periodType}, // 过滤特定时期类型
		}).
		OrderBy("created_at DESC"). // 按时间降序排列
		Limit(1)                    // 只返回最新的一条记录

	var snapshot types.MonitorHistoricalSnapshot
	// 使用QueryRowContext执行查询，只返回一行结果
	// Scan方法将查询结果映射到结构体字段
	err := query.RunWith(s.db).QueryRowContext(ctx).Scan(
		&snapshot.ID, &snapshot.AgentID, &snapshot.InterfaceName,
		&snapshot.PeriodType, &snapshot.DataJSON, &snapshot.CreatedAt,
	)
	// 将sql.ErrNoRows转换为自定义的ErrNotFound错误，提高代码可读性
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return &snapshot, err
}

// CleanupMonitorData 根据保留策略删除旧数据
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和取消操作
//
// 返回值:
//
//	成功时返回nil，失败时返回错误信息
//
// 功能:
//   - 清理超过2小时的资源统计数据
//   - 清理旧的历史快照，每个代理每种类型只保留最新的
//   - 使用事务确保数据一致性
func (s *service) CleanupMonitorData(ctx context.Context) error {
	log.Info().Msg("开始监控数据清理")

	// 开始事务
	// 使用事务确保所有清理操作要么全部成功，要么全部失败
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // 使用defer确保事务在函数返回时回滚，除非显式提交

	// 清理超过2小时的资源统计数据
	// 资源统计数据每30秒收集一次，2小时约240个数据点，足以满足监控需求
	resourceCutoff := time.Now().Add(-2 * time.Hour)
	log.Debug().Time("cutoff", resourceCutoff).Msg("清理资源统计数据")

	// 使用squirrel SQL构建器创建DELETE语句
	// 使用sq.Lt过滤created_at小于指定时间的数据
	deleteQuery := s.sqlBuilder.Delete("monitor_resource_stats").Where(sq.Lt{"created_at": resourceCutoff})
	result, err := deleteQuery.RunWith(tx).ExecContext(ctx)
	if err != nil {
		log.Error().Err(err).Msg("清理资源统计数据失败")
		return err // 如果删除失败，事务会通过defer自动回滚
	}

	rowsDeleted, _ := result.RowsAffected()
	log.Info().Int64("rows_deleted", rowsDeleted).Msg("已清理资源统计数据")

	// 清理旧的历史快照 - 每个代理每种类型只保留最新的
	// 这需要使用子查询，并且不同数据库有不同的实现方式
	if s.config.Type == "sqlite" {
		// SQLite版本：使用MAX(id)获取每个代理每种类型的最新记录
		// 由于SQLite没有DISTINCT ON语法，所以使用MAX(id)方法
		query := `
		DELETE FROM monitor_historical_snapshots
		WHERE id NOT IN (
			SELECT MAX(id)
			FROM monitor_historical_snapshots
			GROUP BY agent_id, period_type
		)`
		result, err := tx.ExecContext(ctx, query)
		if err != nil {
			log.Error().Err(err).Msg("清理历史快照失败")
			return err
		}
		rowsDeleted, _ := result.RowsAffected()
		log.Info().Int64("snapshots_deleted", rowsDeleted).Msg("已清理历史快照")
	} else {
		// PostgreSQL版本：使用DISTINCT ON获取每个代理每种类型的最新记录
		// DISTINCT ON是PostgreSQL的特有语法，更高效地获取每个组的第一条记录
		query := `
		DELETE FROM monitor_historical_snapshots
		WHERE id NOT IN (
			SELECT DISTINCT ON (agent_id, period_type) id
			FROM monitor_historical_snapshots
			ORDER BY agent_id, period_type, created_at DESC
		)`
		result, err := tx.ExecContext(ctx, query)
		if err != nil {
			log.Error().Err(err).Msg("清理历史快照失败")
			return err
		}
		rowsDeleted, _ := result.RowsAffected()
		log.Info().Int64("snapshots_deleted", rowsDeleted).Msg("已清理历史快照")
	}

	// 提交事务
	// 如果所有清理操作都成功，提交事务以保存更改
	if err := tx.Commit(); err != nil {
		log.Error().Err(err).Msg("提交清理事务失败")
		return err
	}

	log.Info().Msg("监控数据清理成功完成")
	return nil
}
