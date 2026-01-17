// Copyright (c) 2024-2025, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// 包 database 提供数据库操作相关功能
// 本文件实现了速度测试结果的数据库存储和查询功能
// 支持 PostgreSQL 和 SQLite 两种数据库类型
package database

import (
	"context" // 上下文管理，用于控制请求的生命周期
	"fmt"
	"time"

	"github.com/autobrr/netronome/internal/config" // 配置管理
	"github.com/autobrr/netronome/internal/types"  // 数据类型定义
)

// SaveSpeedTest 保存速度测试结果到数据库
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和超时
//	result: 速度测试结果数据结构
//
// 返回值:
//
//	*types.SpeedTestResult: 保存成功后的结果，包含自动生成的ID
//	error: 如果保存过程中发生错误，则返回错误信息
//
// 说明:
//   - 支持 PostgreSQL 和 SQLite 数据库
//   - 如果未提供创建时间，默认使用当前UTC时间
//   - 自动将时间转换为UTC格式存储
func (s *service) SaveSpeedTest(ctx context.Context, result types.SpeedTestResult) (*types.SpeedTestResult, error) {
	// 准备数据库插入数据映射
	data := map[string]interface{}{
		"server_name":    result.ServerName,    // 服务器名称
		"server_id":      result.ServerID,      // 服务器ID
		"server_host":    result.ServerHost,    // 服务器主机地址
		"test_type":      result.TestType,      // 测试类型 (iperf, speedtest, librespeed)
		"download_speed": result.DownloadSpeed, // 下载速度 (bps)
		"upload_speed":   result.UploadSpeed,   // 上传速度 (bps)
		"latency":        result.Latency,       // 延迟 (ms)
		"jitter":         result.Jitter,        // 抖动 (ms)
		"is_scheduled":   result.IsScheduled,   // 是否为定时测试
	}

	// 如果提供了创建时间则使用，否则默认为当前UTC时间
	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now().UTC()
	} else {
		result.CreatedAt = result.CreatedAt.UTC() // 确保存储为UTC时间
	}
	data["created_at"] = result.CreatedAt

	var id int64 // 用于存储插入后的记录ID

	// 根据数据库类型执行不同的插入操作
	switch s.config.Type {
	case config.Postgres: // PostgreSQL 使用 RETURNING 子句获取ID
		query := s.sqlBuilder.Insert("speed_tests").
			SetMap(data).
			Suffix("RETURNING id") // 使用RETURNING子句获取自动生成的ID

		sqlStr, args, err := query.ToSql()
		if err != nil {
			return nil, fmt.Errorf("构建查询失败: %w", err)
		}

		err = s.db.QueryRowContext(ctx, sqlStr, args...).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("保存速度测试结果失败: %w", err)
		}

	case config.SQLite: // SQLite 使用 LastInsertId() 获取ID
		res, err := s.insert(ctx, "speed_tests", data)
		if err != nil {
			return nil, fmt.Errorf("保存速度测试结果失败: %w", err)
		}

		id, err = res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("获取最后插入ID失败: %w", err)
		}
	}

	result.ID = id // 设置自动生成的ID
	return &result, nil
}

// GetSpeedTests 获取速度测试结果列表
// 参数:
//
//	ctx: 上下文，用于控制请求生命周期和超时
//	timeRange: 时间范围过滤条件 ("all", "24h", "1d", "3d", "week", "1w", "month", "1m")
//	page: 当前页码 (从1开始)
//	limit: 每页记录数
//
// 返回值:
//
//	*types.PaginatedSpeedTests: 分页结果，包含数据列表和分页信息
//	error: 如果查询过程中发生错误，则返回错误信息
//
// 说明:
//   - 支持时间范围过滤
//   - 按创建时间倒序排列
//   - 返回结果包含分页信息 (当前页、每页条数、总记录数)
//   - 自动将时间转换为UTC格式返回
func (s *service) GetSpeedTests(ctx context.Context, timeRange string, page, limit int) (*types.PaginatedSpeedTests, error) {
	baseQuery := s.sqlBuilder.Select().From("speed_tests") // 基础查询构建器

	// 如果指定了时间范围，则添加时间过滤条件
	if timeRange != "all" {
		var timeExpr string // 时间表达式
		// 根据数据库类型构建不同的时间表达式
		switch s.config.Type {
		case config.Postgres: // PostgreSQL 时间表达式
			switch timeRange {
			case "24h", "1d":
				timeExpr = "NOW() - INTERVAL '1 day'" // 最近24小时
			case "3d":
				timeExpr = "NOW() - INTERVAL '3 days'" // 最近3天
			case "week", "1w":
				timeExpr = "NOW() - INTERVAL '7 days'" // 最近1周
			case "month", "1m":
				timeExpr = "NOW() - INTERVAL '1 month'" // 最近1个月
			}
		case config.SQLite: // SQLite 时间表达式
			switch timeRange {
			case "24h", "1d":
				timeExpr = "datetime('now', '-1 day')" // 最近24小时
			case "3d":
				timeExpr = "datetime('now', '-3 days')" // 最近3天
			case "week", "1w":
				timeExpr = "datetime('now', '-7 days')" // 最近1周
			case "month", "1m":
				timeExpr = "datetime('now', '-1 month')" // 最近1个月
			}
		}
		if timeExpr != "" {
			baseQuery = baseQuery.Where("created_at >= " + timeExpr) // 添加时间过滤条件
		}
	}

	// 构建并执行总数查询，用于分页
	countQuery := baseQuery.Columns("COUNT(*)")
	var total int
	err := countQuery.RunWith(s.db).QueryRowContext(ctx).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("获取总记录数失败: %w", err)
	}

	// 构建分页查询
	dataQuery := baseQuery.Columns(
		"id",
		"server_name",
		"server_id",
		"server_host",
		"test_type",
		"download_speed",
		"upload_speed",
		"latency",
		"jitter",
		"is_scheduled",
		"created_at",
	).
		OrderBy("created_at DESC").        // 按创建时间倒序排列
		Limit(uint64(limit)).              // 限制每页记录数
		Offset(uint64((page - 1) * limit)) // 计算偏移量

	rows, err := dataQuery.RunWith(s.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询速度测试结果失败: %w", err)
	}
	defer rows.Close() // 确保结果集在函数结束时关闭

	results := make([]types.SpeedTestResult, 0) // 存储结果的切片
	for rows.Next() {
		var result types.SpeedTestResult
		err := rows.Scan(
			&result.ID,
			&result.ServerName,
			&result.ServerID,
			&result.ServerHost,
			&result.TestType,
			&result.DownloadSpeed,
			&result.UploadSpeed,
			&result.Latency,
			&result.Jitter,
			&result.IsScheduled,
			&result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描速度测试结果失败: %w", err)
		}

		result.CreatedAt = result.CreatedAt.UTC() // 确保返回UTC时间
		results = append(results, result)
	}

	// 检查遍历过程中是否发生错误
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历速度测试结果时出错: %w", err)
	}

	// 返回分页结果
	return &types.PaginatedSpeedTests{
		Data:  results, // 结果数据列表
		Total: total,   // 总记录数
		Page:  page,    // 当前页码
		Limit: limit,   // 每页记录数
	}, nil
}
