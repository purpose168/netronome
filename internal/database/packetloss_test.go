// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later
//
// 此文件包含丢包监控系统的单元测试
// 主要测试数据库特定行为和错误处理
// 包括PostgreSQL的RETURNING子句和SQLite的LastInsertId等数据库特性

package database

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	sq "github.com/Masterminds/squirrel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/netronome/internal/config"
	"github.com/autobrr/netronome/internal/types"
)

// stringPtr 返回一个指向字符串的指针
func stringPtr(s string) *string {
	return &s
}

// 数据库特定行为和错误处理的单元测试
// 有关全面的集成测试，请参阅 packetloss_integration_test.go

// mockPostgresResult 模拟不支持LastInsertId的PostgreSQL驱动程序结果
type mockPostgresResult struct {
	rowsAffected int64
}

// LastInsertId 模拟PostgreSQL驱动不支持获取最后插入ID的行为
func (m mockPostgresResult) LastInsertId() (int64, error) {
	return 0, errors.New("LastInsertId is not supported by this driver")
}

// RowsAffected 返回受影响的行数
func (m mockPostgresResult) RowsAffected() (int64, error) {
	return m.rowsAffected, nil
}

// TestSavePacketLossResult_PostgreSQLReturning 验证PostgreSQL的RETURNING子句行为
// 此测试确保PostgreSQL数据库在保存丢包结果时使用RETURNING子句获取插入的ID
func TestSavePacketLossResult_PostgreSQLReturning(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	s := &service{
		db:         db,
		config:     config.DatabaseConfig{Type: config.Postgres},
		sqlBuilder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}

	result := &types.PacketLossResult{
		MonitorID:  1,
		PacketLoss: 5.5,
		CreatedAt:  time.Now(),
	}

	// 验证PostgreSQL使用RETURNING子句
	mock.ExpectQuery(`INSERT INTO packet_loss_results .+ RETURNING id`).
		WithArgs(
			result.MonitorID,
			result.PacketLoss,
			sqlmock.AnyArg(), // MinRTT
			sqlmock.AnyArg(), // MaxRTT
			sqlmock.AnyArg(), // AvgRTT
			sqlmock.AnyArg(), // StdDevRTT
			sqlmock.AnyArg(), // PacketsSent
			sqlmock.AnyArg(), // PacketsRecv
			sqlmock.AnyArg(), // UsedMTR
			sqlmock.AnyArg(), // HopCount
			sqlmock.AnyArg(), // MTRData
			sqlmock.AnyArg(), // PrivilegedMode
			result.CreatedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))

	err = s.SavePacketLossResult(result)
	assert.NoError(t, err)
	assert.Equal(t, int64(42), result.ID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSavePacketLossResult_SQLiteLastInsertId 验证SQLite的LastInsertId行为
// 此测试确保SQLite数据库在保存丢包结果时使用LastInsertId获取插入的ID
func TestSavePacketLossResult_SQLiteLastInsertId(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	s := &service{
		db:         db,
		config:     config.DatabaseConfig{Type: config.SQLite},
		sqlBuilder: sq.StatementBuilder,
	}

	result := &types.PacketLossResult{
		MonitorID:  1,
		PacketLoss: 5.5,
		CreatedAt:  time.Now(),
	}

	// 验证SQLite使用LastInsertId
	mock.ExpectExec(`INSERT INTO packet_loss_results`).
		WithArgs(
			sqlmock.AnyArg(), // MonitorID
			sqlmock.AnyArg(), // PacketLoss
			sqlmock.AnyArg(), // MinRTT
			sqlmock.AnyArg(), // MaxRTT
			sqlmock.AnyArg(), // AvgRTT
			sqlmock.AnyArg(), // StdDevRTT
			sqlmock.AnyArg(), // PacketsSent
			sqlmock.AnyArg(), // PacketsRecv
			sqlmock.AnyArg(), // UsedMTR
			sqlmock.AnyArg(), // HopCount
			sqlmock.AnyArg(), // MTRData
			sqlmock.AnyArg(), // PrivilegedMode
			sqlmock.AnyArg(), // CreatedAt
		).
		WillReturnResult(sqlmock.NewResult(42, 1))

	err = s.SavePacketLossResult(result)
	assert.NoError(t, err)
	assert.Equal(t, int64(42), result.ID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestPostgreSQLDriverLastInsertIdError 演示PostgreSQL驱动程序的行为
// 此测试验证PostgreSQL驱动程序不支持LastInsertId方法的预期行为
func TestPostgreSQLDriverLastInsertIdError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// 模拟PostgreSQL驱动行为 - Exec返回不支持LastInsertId的结果
	mock.ExpectExec(`INSERT INTO test_table`).
		WillReturnResult(mockPostgresResult{rowsAffected: 1})

	// 执行一个通常需要LastInsertId的查询
	res, err := db.Exec(`INSERT INTO test_table (col) VALUES (?)`, "value")
	require.NoError(t, err)

	// 验证PostgreSQL的LastInsertId按预期失败
	_, err = res.LastInsertId()
	assert.Error(t, err)
	assert.Equal(t, "LastInsertId is not supported by this driver", err.Error())
}

// TestSavePacketLossResult_UnsupportedDatabase 验证不支持的数据库类型的错误处理
// 此测试确保系统对不支持的数据库类型返回适当的错误
func TestSavePacketLossResult_UnsupportedDatabase(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	s := &service{
		db:         db,
		config:     config.DatabaseConfig{Type: "unsupported"},
		sqlBuilder: sq.StatementBuilder,
	}

	result := &types.PacketLossResult{
		MonitorID: 1,
		CreatedAt: time.Now(),
	}

	err = s.SavePacketLossResult(result)
	assert.Error(t, err)
	assert.Equal(t, "unsupported database type: unsupported", err.Error())
}

// TestSavePacketLossResult_QueryError 验证查询失败时的错误处理
// 此测试确保系统在数据库查询失败时正确处理错误
func TestSavePacketLossResult_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	s := &service{
		db:         db,
		config:     config.DatabaseConfig{Type: config.Postgres},
		sqlBuilder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}

	result := &types.PacketLossResult{
		MonitorID: 1,
		CreatedAt: time.Now(),
	}

	// 模拟查询错误
	mock.ExpectQuery(`INSERT INTO packet_loss_results`).
		WillReturnError(errors.New("database connection lost"))

	err = s.SavePacketLossResult(result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database connection lost")
}
