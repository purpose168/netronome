// Copyright (c) 2024-2025, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// 包 database 提供数据库操作相关功能
// 本文件实现了用户管理的数据库操作，包括用户创建、查询、密码验证和更新
package database

import (
	"context" // 上下文管理，用于控制请求的生命周期
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel" // SQL构建器，用于安全地构建SQL查询
	"golang.org/x/crypto/bcrypt"         // bcrypt密码哈希库，用于密码加密和验证

	"github.com/autobrr/netronome/internal/config" // 配置管理
)

// 用户相关错误定义
var (
	ErrUserNotFound         = errors.New("用户不存在")  // 用户未找到错误
	ErrRegistrationDisabled = errors.New("注册已禁用")  // 注册功能禁用错误
	ErrUserAlreadyExists    = errors.New("用户名已存在") // 用户名已存在错误
)

// User 用户结构体
// 用于表示系统中的用户信息
// 注意：PasswordHash 字段使用 json:"-" 标签，不会被序列化到JSON响应中
// 这是安全措施，防止密码哈希泄露

type User struct {
	ID           int64  `json:"id"`       // 用户ID
	Username     string `json:"username"` // 用户名
	PasswordHash string `json:"-"`        // 密码哈希值（不序列化到JSON）
}

// UserService 用户服务接口
// 定义了用户管理的核心操作
// 所有实现该接口的类型都必须提供这些方法的具体实现

type UserService interface {
	// CreateUser 创建新用户
	// 参数：
	//   ctx: 上下文
	//   username: 用户名
	//   password: 密码
	// 返回：
	//   *User: 创建的用户信息
	//   error: 可能的错误
	CreateUser(ctx context.Context, username, password string) (*User, error)

	// GetUserByUsername 根据用户名获取用户
	// 参数：
	//   ctx: 上下文
	//   username: 要查找的用户名
	// 返回：
	//   *User: 用户信息
	//   error: 可能的错误
	GetUserByUsername(ctx context.Context, username string) (*User, error)

	// ValidatePassword 验证用户密码
	// 参数：
	//   user: 用户信息
	//   password: 要验证的密码
	// 返回：
	//   bool: 密码是否有效
	ValidatePassword(user *User, password string) bool

	// UpdatePassword 更新用户密码
	// 参数：
	//   ctx: 上下文
	//   username: 用户名
	//   newPassword: 新密码
	// 返回：
	//   error: 可能的错误
	UpdatePassword(ctx context.Context, username, newPassword string) error
}

// CreateUser 创建新用户
// 参数：
//   ctx: 上下文，用于控制请求生命周期和超时
//   username: 用户名
//   password: 密码（明文）
// 返回：
//   *User: 创建成功的用户信息
//   error: 可能的错误，包括：
//          - ErrInvalidInput: 用户名或密码为空
//          - ErrRegistrationDisabled: 注册已被禁用
//          - ErrUserAlreadyExists: 用户名已存在
//          - 其他数据库或密码哈希相关错误
// 注意：
//   该系统只允许创建一个用户，第一个用户创建成功后，注册功能将自动禁用
//   密码会使用bcrypt算法进行哈希处理，不会存储明文密码
//   所有数据库操作都在事务中执行，确保数据一致性

func (s *service) CreateUser(ctx context.Context, username, password string) (*User, error) {
	// 验证输入参数
	if username == "" || password == "" {
		return nil, fmt.Errorf("%w: 用户名和密码不能为空", ErrInvalidInput)
	}

	// 检查是否已有用户存在，如果有则禁用注册
	count, err := s.count(ctx, "users", sq.Eq{})
	if err != nil && !isTableNotExistsError(err) {
		return nil, fmt.Errorf("检查现有用户失败: %w", err)
	}

	// 如果已有用户，返回注册禁用错误
	if count > 0 {
		return nil, ErrRegistrationDisabled
	}

	// 检查用户名是否已存在
	exists, err := s.count(ctx, "users", sq.Eq{"username": username})
	if err != nil && !isTableNotExistsError(err) {
		return nil, fmt.Errorf("检查用户名存在性失败: %w", err)
	}

	if exists > 0 {
		return nil, ErrUserAlreadyExists
	}

	// 使用bcrypt算法生成密码哈希
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("生成密码哈希失败: %w", err)
	}

	// 开始数据库事务，确保操作原子性
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("开始事务失败: %w", err)
	}
	// 确保如果函数提前返回，事务会被回滚
	defer tx.Rollback()

	// 构建用户插入查询
	query := s.sqlBuilder.
		Insert("users").
		Columns("username", "password_hash").
		Values(username, string(hash))

	// 根据数据库类型添加不同的后缀
	if s.config.Type == config.Postgres {
		query = query.Suffix("RETURNING id") // PostgreSQL使用RETURNING获取ID
	}

	var id int64 // 存储新用户的ID
	// 执行插入操作并获取ID
	if s.config.Type == config.Postgres {
		// PostgreSQL使用QueryRow获取RETURNING的结果
		err = query.RunWith(tx).QueryRowContext(ctx).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("创建用户失败: %w", err)
		}
	} else {
		// SQLite使用LastInsertId获取ID
		result, err := query.RunWith(tx).ExecContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("创建用户失败: %w", err)
		}
		id, err = result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("获取最后插入ID失败: %w", err)
		}
	}

	// 禁用注册功能的SQL语句
	var disableRegQuery string
	if s.config.Type == config.Postgres {
		// PostgreSQL: 删除现有记录并插入新记录
		disableRegQuery = `
			DELETE FROM registration_status;
			INSERT INTO registration_status (is_registration_enabled) VALUES (false);`
	} else {
		// SQLite: 使用ON CONFLICT更新现有记录
		disableRegQuery = `
			INSERT INTO registration_status (is_registration_enabled) 
			VALUES (0) 
			ON CONFLICT (rowid) DO UPDATE SET is_registration_enabled = 0`
	}

	// 执行禁用注册的SQL
	_, err = tx.ExecContext(ctx, disableRegQuery)
	if err != nil && !isTableNotExistsError(err) {
		return nil, fmt.Errorf("禁用注册失败: %w", err)
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	// 返回创建的用户信息
	return &User{
		ID:           id,
		Username:     username,
		PasswordHash: string(hash),
	}, nil
}

// GetUserByUsername 根据用户名获取用户信息
// 参数：
//   ctx: 上下文，用于控制请求生命周期和超时
//   username: 要查找的用户名
// 返回：
//   *User: 用户信息，包含ID、用户名和密码哈希
//   error: 可能的错误，包括：
//          - ErrInvalidInput: 用户名为空
//          - ErrUserNotFound: 用户不存在
//          - 其他数据库相关错误
// 注意：
//   返回的User对象包含PasswordHash字段，用于密码验证
//   但该字段不会被序列化到JSON响应中（由于json:"-"标签）

func (s *service) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	// 验证输入参数
	if username == "" {
		return nil, fmt.Errorf("%w: 用户名为空", ErrInvalidInput)
	}

	// 构建查询，获取用户ID、用户名和密码哈希
	query := s.sqlBuilder.
		Select("id", "username", "password_hash").
		From("users").
		Where(sq.Eq{"username": username})

	// 执行查询
	rows, err := query.RunWith(s.db).QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	// 确保结果集在函数结束时关闭
	defer rows.Close()

	// 检查是否找到用户
	if !rows.Next() {
		return nil, ErrUserNotFound
	}

	// 扫描结果到User对象
	user := &User{}
	err = rows.Scan(&user.ID, &user.Username, &user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("扫描用户失败: %w", err)
	}

	// 检查遍历过程中是否发生错误
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("扫描用户后出错: %w", err)
	}

	return user, nil
}

// ValidatePassword 验证用户密码
// 参数：
//   user: 用户对象，包含密码哈希
//   password: 要验证的密码（明文）
// 返回：
//   bool: 密码是否有效
// 注意：
//   该方法使用bcrypt.CompareHashAndPassword函数比较密码哈希和明文密码
//   如果user为nil或密码为空，直接返回false
//   如果哈希比较成功，返回true；否则返回false
//   bcrypt算法会自动处理哈希的盐值和成本参数，不需要额外处理

func (s *service) ValidatePassword(user *User, password string) bool {
	// 快速失败检查：如果用户对象或密码为空，直接返回false
	if user == nil || password == "" {
		return false
	}

	// 使用bcrypt比较密码哈希和明文密码
	// 如果比较成功，err为nil，返回true
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// UpdatePassword 更新用户密码
// 参数：
//   ctx: 上下文，用于控制请求生命周期和超时
//   username: 用户名
//   newPassword: 新密码（明文）
// 返回：
//   error: 可能的错误，包括：
//          - ErrInvalidInput: 用户名或新密码为空
//          - ErrUserNotFound: 用户不存在
//          - 其他数据库或密码哈希相关错误
// 注意：
//   该方法会先验证用户是否存在，然后生成新的密码哈希并更新数据库
//   密码使用bcrypt算法进行哈希处理，确保安全性

func (s *service) UpdatePassword(ctx context.Context, username, newPassword string) error {
	// 验证输入参数
	if username == "" || newPassword == "" {
		return fmt.Errorf("%w: 用户名和密码不能为空", ErrInvalidInput)
	}

	// 检查用户是否存在
	_, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("获取用户失败: %w", err)
	}

	// 生成新的密码哈希
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("生成密码哈希失败: %w", err)
	}

	// 构建更新查询
	query := s.sqlBuilder.
		Update("users").
		Set("password_hash", string(hash)).
		Where(sq.Eq{"username": username})

	// 执行更新操作
	result, err := query.RunWith(s.db).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}

	// 检查更新是否成功（是否影响了一行）
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// isTableNotExistsError 检查错误是否是表不存在的错误
// 参数：
//   err: 要检查的错误
// 返回：
//   bool: 如果错误是表不存在的错误，则返回true
// 注意：
//   该函数仅检查特定的SQLite表不存在错误信息
//   用于在初始化数据库时忽略表不存在的错误
//   当前支持检查"users"和"registration_status"表

func isTableNotExistsError(err error) bool {
	return err != nil && (err.Error() == "SQL logic error: no such table: users (1)" ||
		err.Error() == "SQL logic error: no such table: registration_status (1)")
}
