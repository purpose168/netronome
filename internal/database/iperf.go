// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later

package database

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	"github.com/autobrr/netronome/internal/config"
	"github.com/autobrr/netronome/internal/types"
)

// SaveIperfServer 保存 Iperf 服务器信息到数据库
// ctx: 上下文对象，用于控制函数执行和超时
// name: 服务器名称
// host: 服务器主机地址
// port: 服务器端口号
// 返回保存的服务器信息和可能发生的错误
func (s *service) SaveIperfServer(ctx context.Context, name, host string, port int) (*types.SavedIperfServer, error) {
	// 验证输入参数
	if name == "" || host == "" || port <= 0 {
		return nil, ErrInvalidInput // 参数无效错误
	}

	// 构建要插入的数据映射
	data := map[string]interface{}{
		"name":       name,
		"host":       host,
		"port":       port,
		"created_at": sq.Expr("CURRENT_TIMESTAMP"), // 使用数据库当前时间
		"updated_at": sq.Expr("CURRENT_TIMESTAMP"), // 使用数据库当前时间
	}

	var id int64 // 保存插入的记录ID

	// 根据数据库类型使用不同的插入方式
	switch s.config.Type {
	case config.Postgres:
		// 构建 PostgreSQL 插入查询，使用 RETURNING 获取插入的 ID
		query := s.sqlBuilder.Insert("saved_iperf_servers").
			SetMap(data).
			Suffix("RETURNING id")

		// 转换为 SQL 语句和参数
		sqlStr, args, err := query.ToSql()
		if err != nil {
			return nil, fmt.Errorf("构建查询失败: %w", err)
		}

		// 执行查询并获取插入的 ID
		err = s.db.QueryRowContext(ctx, sqlStr, args...).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("保存 Iperf 服务器失败: %w", err)
		}

	case config.SQLite:
		// 使用通用插入方法插入数据
		res, err := s.insert(ctx, "saved_iperf_servers", data)
		if err != nil {
			return nil, fmt.Errorf("保存 Iperf 服务器失败: %w", err)
		}

		// 获取插入记录的 ID
		id, err = res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("获取最后插入 ID 失败: %w", err)
		}
	}

	// 查询刚保存的服务器完整信息
	query := s.sqlBuilder.
		Select(
			"id",
			"name",
			"host",
			"port",
			"created_at",
			"updated_at",
		).
		From("saved_iperf_servers").
		Where(sq.Eq{"id": id})

	var server types.SavedIperfServer
	// 执行查询并将结果扫描到服务器对象
	err := query.RunWith(s.db).QueryRowContext(ctx).Scan(
		&server.ID,
		&server.Name,
		&server.Host,
		&server.Port,
		&server.CreatedAt,
		&server.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("获取保存的 Iperf 服务器失败: %w", err)
	}

	return &server, nil
}

// GetIperfServers 获取所有保存的 Iperf 服务器信息
// ctx: 上下文对象，用于控制函数执行和超时
// 返回 Iperf 服务器列表和可能发生的错误
func (s *service) GetIperfServers(ctx context.Context) ([]types.SavedIperfServer, error) {
	// 构建查询语句，按创建时间降序排序
	query := s.sqlBuilder.
		Select(
			"id",
			"name",
			"host",
			"port",
			"created_at",
			"updated_at",
		).
		From("saved_iperf_servers").
		OrderBy("created_at DESC")

	// 执行查询
	rows, err := query.RunWith(s.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取 Iperf 服务器列表失败: %w", err)
	}
	defer rows.Close() // 确保函数退出时关闭结果集

	// 初始化服务器列表
	servers := make([]types.SavedIperfServer, 0)
	// 遍历结果集
	for rows.Next() {
		var server types.SavedIperfServer
		// 将结果扫描到服务器对象
		err := rows.Scan(
			&server.ID,
			&server.Name,
			&server.Host,
			&server.Port,
			&server.CreatedAt,
			&server.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描 Iperf 服务器信息失败: %w", err)
		}
		servers = append(servers, server) // 添加到列表
	}

	// 检查遍历过程中是否发生错误
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 Iperf 服务器列表时出错: %w", err)
	}

	return servers, nil
}

// DeleteIperfServer 删除指定 ID 的 Iperf 服务器
// ctx: 上下文对象，用于控制函数执行和超时
// id: 要删除的服务器 ID
// 返回可能发生的错误
func (s *service) DeleteIperfServer(ctx context.Context, id int) error {
	// 验证 ID 是否有效
	if id <= 0 {
		return ErrInvalidInput // ID 无效错误
	}

	// 构建删除查询
	query := s.sqlBuilder.
		Delete("saved_iperf_servers").
		Where(sq.Eq{"id": id})

	// 执行删除操作
	result, err := query.RunWith(s.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("删除 Iperf 服务器失败: %w", err)
	}

	// 获取受影响的行数
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取受影响行数失败: %w", err)
	}

	// 如果没有行被影响，说明找不到指定 ID 的服务器
	if affected == 0 {
		return ErrNotFound // 未找到错误
	}

	return nil
}
