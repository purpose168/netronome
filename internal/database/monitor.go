// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 此文件包含监控代理的数据库操作功能
// 包括代理的创建、查询、更新和删除操作

package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/netronome/internal/config"
	"github.com/autobrr/netronome/internal/types"
)

// CreateMonitorAgent 创建一个新的监控代理
// 参数：
//
//	ctx: 上下文，用于控制请求的生命周期和超时
//	agent: 监控代理结构体，包含代理的名称、URL、API密钥等信息
//
// 返回值：
//
//	*types.MonitorAgent: 创建成功的监控代理，包含自动生成的ID和时间戳
//	error: 如果创建过程中发生错误，返回错误信息
//
// 该函数会根据配置的数据库类型（PostgreSQL或SQLite）使用不同的插入方式
// PostgreSQL使用RETURNING子句获取插入的ID，SQLite使用LastInsertId方法
func (s *service) CreateMonitorAgent(ctx context.Context, agent *types.MonitorAgent) (*types.MonitorAgent, error) {
	now := time.Now()
	agent.CreatedAt = now
	agent.UpdatedAt = now

	query := s.sqlBuilder.
		Insert("monitor_agents").
		Columns("name", "url", "api_key", "enabled", "interface", "is_tailscale", "tailscale_hostname", "discovered_at", "created_at", "updated_at").
		Values(agent.Name, agent.URL, agent.APIKey, agent.Enabled, agent.Interface, agent.IsTailscale, agent.TailscaleHostname, agent.DiscoveredAt, agent.CreatedAt, agent.UpdatedAt)

	if s.config.Type == config.Postgres {
		query = query.Suffix("RETURNING id")
		err := query.RunWith(s.db).QueryRowContext(ctx).Scan(&agent.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to create monitor agent: %w", err)
		}
	} else {
		res, err := query.RunWith(s.db).ExecContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create monitor agent: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("failed to get last insert id: %w", err)
		}
		agent.ID = id
	}

	return agent, nil
}

// GetMonitorAgent 根据ID获取监控代理
// 参数：
//
//	ctx: 上下文，用于控制请求的生命周期和超时
//	agentID: 监控代理的唯一标识符
//
// 返回值：
//
//	*types.MonitorAgent: 获取到的监控代理
//	error: 如果获取过程中发生错误，返回错误信息；如果未找到代理，返回ErrNotFound
//
// 该函数通过ID在数据库中查找监控代理，并将结果映射到MonitorAgent结构体
func (s *service) GetMonitorAgent(ctx context.Context, agentID int64) (*types.MonitorAgent, error) {
	query := s.sqlBuilder.
		Select("id", "name", "url", "api_key", "enabled", "interface", "is_tailscale", "tailscale_hostname", "discovered_at", "created_at", "updated_at").
		From("monitor_agents").
		Where(sq.Eq{"id": agentID})

	var agent types.MonitorAgent
	err := query.RunWith(s.db).QueryRowContext(ctx).Scan(
		&agent.ID,
		&agent.Name,
		&agent.URL,
		&agent.APIKey,
		&agent.Enabled,
		&agent.Interface,
		&agent.IsTailscale,
		&agent.TailscaleHostname,
		&agent.DiscoveredAt,
		&agent.CreatedAt,
		&agent.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get monitor agent: %w", err)
	}

	return &agent, nil
}

// GetMonitorAgents 获取所有监控代理
// 参数：
//
//	ctx: 上下文，用于控制请求的生命周期和超时
//	enabledOnly: 如果为true，只返回启用状态的监控代理
//
// 返回值：
//
//	[]*types.MonitorAgent: 监控代理列表，按创建时间降序排列
//	error: 如果获取过程中发生错误，返回错误信息
//
// 该函数支持过滤只获取启用的代理，并按创建时间倒序排列结果
func (s *service) GetMonitorAgents(ctx context.Context, enabledOnly bool) ([]*types.MonitorAgent, error) {
	query := s.sqlBuilder.
		Select("id", "name", "url", "api_key", "enabled", "interface", "is_tailscale", "tailscale_hostname", "discovered_at", "created_at", "updated_at").
		From("monitor_agents").
		OrderBy("created_at DESC")

	if enabledOnly {
		query = query.Where(sq.Eq{"enabled": true})
	}

	rows, err := query.RunWith(s.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get monitor agents: %w", err)
	}
	defer rows.Close()

	agents := make([]*types.MonitorAgent, 0)
	for rows.Next() {
		var agent types.MonitorAgent
		err := rows.Scan(
			&agent.ID,
			&agent.Name,
			&agent.URL,
			&agent.APIKey,
			&agent.Enabled,
			&agent.Interface,
			&agent.IsTailscale,
			&agent.TailscaleHostname,
			&agent.DiscoveredAt,
			&agent.CreatedAt,
			&agent.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan monitor agent: %w", err)
		}
		agents = append(agents, &agent)
	}

	return agents, nil
}

// UpdateMonitorAgent 更新监控代理信息
// 参数：
//
//	ctx: 上下文，用于控制请求的生命周期和超时
//	agent: 包含更新信息的监控代理结构体，必须包含有效的ID
//
// 返回值：
//
//	error: 如果更新过程中发生错误，返回错误信息
//
// 该函数会更新监控代理的所有字段，并自动更新UpdatedAt时间戳
func (s *service) UpdateMonitorAgent(ctx context.Context, agent *types.MonitorAgent) error {
	agent.UpdatedAt = time.Now()

	query := s.sqlBuilder.
		Update("monitor_agents").
		Set("name", agent.Name).
		Set("url", agent.URL).
		Set("api_key", agent.APIKey).
		Set("enabled", agent.Enabled).
		Set("interface", agent.Interface).
		Set("is_tailscale", agent.IsTailscale).
		Set("tailscale_hostname", agent.TailscaleHostname).
		Set("discovered_at", agent.DiscoveredAt).
		Set("updated_at", agent.UpdatedAt).
		Where(sq.Eq{"id": agent.ID})

	_, err := query.RunWith(s.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to update monitor agent: %w", err)
	}

	return nil
}

// DeleteMonitorAgent 删除监控代理及其所有关联数据
// 参数：
//
//	ctx: 上下文，用于控制请求的生命周期和超时
//	agentID: 要删除的监控代理的唯一标识符
//
// 返回值：
//
//	error: 如果删除过程中发生错误，返回错误信息
//
// 该函数使用事务确保数据一致性：
// 1. 首先删除所有关联数据（遵循外键约束）
// 2. 最后删除代理本身
// 如果任何步骤失败，会回滚整个事务
func (s *service) DeleteMonitorAgent(ctx context.Context, agentID int64) error {
	// Start transaction to delete agent and all associated data
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete associated data first (foreign key constraints)
	tables := []string{
		"monitor_agent_interfaces",
		"monitor_agent_system_info",
		"monitor_peak_stats",
		"monitor_resource_stats",
		"monitor_historical_snapshots",
	}

	for _, table := range tables {
		deleteQuery := s.sqlBuilder.Delete(table).Where(sq.Eq{"agent_id": agentID})
		result, err := deleteQuery.RunWith(tx).ExecContext(ctx)
		if err != nil {
			log.Error().Err(err).Str("table", table).Int64("agent_id", agentID).Msg("Failed to delete agent data")
			return fmt.Errorf("failed to delete %s for agent %d: %w", table, agentID, err)
		}
		rowsDeleted, _ := result.RowsAffected()
		log.Debug().Str("table", table).Int64("agent_id", agentID).Int64("rows_deleted", rowsDeleted).Msg("Deleted agent data")
	}

	// Finally delete the agent itself
	query := s.sqlBuilder.Delete("monitor_agents").Where(sq.Eq{"id": agentID})
	_, err = query.RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete monitor agent: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit delete transaction: %w", err)
	}

	log.Info().Int64("agent_id", agentID).Msg("Successfully deleted monitor agent and all associated data")
	return nil
}
