// 版权所有 (c) 2024-2025, s0up 和 autobrr 贡献者。
// SPDX-License-Identifier: GPL-2.0-or-later

// 数据库迁移包
// 该包负责管理数据库迁移文件，支持 SQLite 和 PostgreSQL 两种数据库类型
// 主要功能包括：
// 1. 加载和管理 SQL 迁移文件
// 2. 根据数据库类型选择合适的迁移文件
// 3. 按版本号排序迁移文件
// 4. 读取迁移文件内容
package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

// SchemaMigrations 嵌入的数据库迁移文件系统
// 包含 SQLite 和 PostgreSQL 的所有 SQL 迁移文件
//
//go:embed postgres/*.sql sqlite/*.sql
var SchemaMigrations embed.FS

// DatabaseType 数据库类型枚举
// 定义了系统支持的数据库类型
// 目前支持 SQLite 和 PostgreSQL 两种数据库
// 迁移系统会根据这个类型选择对应的 SQL 迁移文件
//
// 迁移文件组织规则：
// - SQLite 迁移文件：sqlite/ 目录下，扩展名为 .sql
// - PostgreSQL 迁移文件：postgres/ 目录下，扩展名为 _postgres.sql
//
// 迁移文件命名规则：
// - 文件名以版本号开头，如 001_initial_schema.sql 或 002_add_users_table_postgres.sql
// - 版本号用于确定迁移顺序
// - 文件名中的下划线分隔版本号和描述
//
// 使用示例：
// - GetMigrationFiles(SQLite) // 获取所有 SQLite 迁移文件
// - GetMigrationFiles(Postgres) // 获取所有 PostgreSQL 迁移文件
type DatabaseType string

// 支持的数据库类型常量
const (
	// SQLite SQLite 数据库类型
	// 适用于轻量级部署和单用户环境
	SQLite DatabaseType = "sqlite"

	// Postgres PostgreSQL 数据库类型
	// 适用于生产环境和多用户环境
	Postgres DatabaseType = "postgres"
)

// GetMigrationFiles 根据给定的数据库类型返回相应的迁移文件列表
// 该函数是迁移系统的核心函数之一，负责：
// 1. 根据数据库类型确定迁移文件的基础路径和文件后缀
// 2. 读取迁移文件目录下的所有文件
// 3. 过滤出符合条件的迁移文件
// 4. 按版本号对迁移文件进行排序
// 5. 返回排序后的迁移文件列表
//
// 参数：
// - dbType: 数据库类型，只能是 SQLite 或 Postgres
//
// 返回值：
// - []string: 排序后的迁移文件路径列表
// - error: 如果数据库类型不支持或读取目录失败，返回错误
//
// 实现逻辑：
// 1. 根据数据库类型设置基础路径和文件后缀
// 2. 检查是否在测试环境中（避免测试时输出过多日志）
// 3. 读取迁移文件目录
// 4. 过滤出符合后缀要求的文件
// 5. 检查文件是否可读（仅记录错误，不中断流程）
// 6. 按版本号对文件进行排序
// 7. 返回排序后的文件列表
//
// 使用示例：
// files, err := GetMigrationFiles(SQLite)
//
//	if err != nil {
//	    log.Fatal().Err(err).Msg("获取迁移文件失败")
//	}
//
//	for _, file := range files {
//	    content, err := ReadMigration(file)
//	    // 执行迁移
//	}
//
// 注意事项：
// - 迁移文件必须按照版本号顺序命名
// - 函数会忽略不可读的文件，但会记录错误日志
// - 测试环境下会减少日志输出
// - 函数返回的文件列表已经按版本号排序，从低到高
func GetMigrationFiles(dbType DatabaseType) ([]string, error) {
	var basePath string // 迁移文件的基础路径
	var suffix string   // 迁移文件的后缀

	// 根据数据库类型确定基础路径和后缀
	switch dbType {
	case Postgres:
		basePath = "postgres"
		suffix = "_postgres.sql"
	case SQLite:
		basePath = "sqlite"
		suffix = ".sql"
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}

	// 仅在非测试环境下记录调试信息
	isTest := strings.Contains(os.Args[0], ".test") || strings.HasSuffix(os.Args[0], "/test")
	if !isTest {
		log.Debug().
			Str("basePath", basePath).
			Str("suffix", suffix).
			Msg("正在查找迁移文件")
	}

	// 读取迁移文件目录
	entries, err := SchemaMigrations.ReadDir(basePath)
	if err != nil {
		log.Error().Err(err).Str("basePath", basePath).Msg("读取迁移目录失败")
		return nil, fmt.Errorf("读取迁移目录失败: %w", err)
	}

	if !isTest {
		log.Debug().Int("entryCount", len(entries)).Msg("在迁移目录中找到条目")
	}

	var files []string // 迁移文件列表

	// 遍历所有目录条目
	for _, entry := range entries {
		// 只处理文件，不处理目录，且文件后缀必须符合要求
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
			filePath := fmt.Sprintf("%s/%s", basePath, entry.Name())

			// 检查文件是否可读
			_, err := fs.ReadFile(SchemaMigrations, filePath)
			if err != nil {
				log.Error().Err(err).Str("file", filePath).Msg("读取迁移文件失败")
			}

			// 将文件添加到列表中
			files = append(files, filePath)
		}
	}

	// 按版本号排序迁移文件
	sortMigrationFiles(files)

	return files, nil
}

// sortMigrationFiles 按版本号对迁移文件进行排序
// 该函数使用冒泡排序算法对迁移文件列表进行排序
// 排序基于文件版本号，确保迁移按照正确的顺序执行
//
// 参数：
// - files []string: 迁移文件路径列表
//
// 副作用：
// - 直接修改传入的切片，不返回新的切片
//
// 排序逻辑：
// 1. 比较相邻两个文件的版本号
// 2. 如果前一个文件的版本号大于后一个，则交换它们的位置
// 3. 重复这个过程，直到所有文件都按版本号升序排列
//
// 注意事项：
// - 排序依赖于 getMigrationVersion 函数提取的版本号
// - 冒泡排序适合小数据集（通常迁移文件数量不会太多）
// - 函数假设文件名遵循命名规则，版本号能被正确提取
func sortMigrationFiles(files []string) {
	n := len(files)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if getMigrationVersion(files[j]) > getMigrationVersion(files[j+1]) {
				files[j], files[j+1] = files[j+1], files[j]
			}
		}
	}
}

// getMigrationVersion 从迁移文件名中提取版本号
// 该函数从迁移文件名中解析出版本号，用于迁移文件的排序和执行顺序
//
// 参数：
// - fileName string: 迁移文件路径，格式如 "sqlite/001_initial_schema.sql" 或 "postgres/002_add_users_postgres.sql"
//
// 返回值：
// - int: 提取的版本号，如果提取失败则返回 0
//
// 版本号提取逻辑：
// 1. 用下划线分割文件名（如 "sqlite/001_initial_schema.sql" -> ["sqlite/001", "initial", "schema.sql"]）
// 2. 获取分割后的第一部分（如 "sqlite/001"）
// 3. 去除前缀中的 "0"（如 "sqlite/001" -> "sqlite/1"）
// 4. 调用 parseInt 函数将版本号部分转换为整数
// 5. 如果转换成功则返回版本号，否则返回 0
//
// 注意事项：
// - 文件名格式必须符合命名规则（版本号+下划线+描述+扩展名）
// - 如果文件名格式不正确，函数会返回 0
// - 返回的版本号用于排序，低版本号的迁移会先执行
// - 函数会忽略文件路径中的目录部分
func getMigrationVersion(fileName string) int {
	parts := strings.Split(fileName, "_")
	if len(parts) > 0 {
		version := strings.TrimPrefix(parts[0], "0")
		if v, err := parseInt(version); err == nil {
			return v
		}
	}
	return 0
}

// parseInt 将字符串转换为整数
// 该函数实现了一个简单的字符串到整数的转换算法
// 用于将版本号字符串转换为整数形式，便于比较和排序
//
// 参数：
// - s string: 要转换的字符串，必须只包含数字字符
//
// 返回值：
// - int: 转换后的整数值
// - error: 如果字符串包含非数字字符，则返回错误
//
// 转换算法：
// 1. 遍历字符串中的每个字符
// 2. 检查字符是否在 '0' 到 '9' 之间
// 3. 如果不是数字字符，返回错误
// 4. 如果是数字字符，将当前结果乘以 10，然后加上字符对应的数字值
// 5. 遍历完成后返回结果
//
// 注意事项：
// - 字符串必须只包含数字字符，不能包含其他字符
// - 函数不处理负数
// - 函数不检查整数溢出
// - 主要用于解析版本号，版本号通常是正整数
func parseInt(s string) (int, error) {
	var result int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("无效的整数: %s", s)
		}
		result = result*10 + int(ch-'0')
	}
	return result, nil
}

// ReadMigration 读取迁移文件内容
// 该函数从嵌入式文件系统中读取指定的迁移文件内容
// 用于获取迁移SQL语句，以便执行数据库迁移
//
// 参数：
// - fileName string: 迁移文件路径，格式如 "sqlite/001_initial_schema.sql"
//
// 返回值：
// - []byte: 迁移文件的内容
// - error: 如果读取文件失败，则返回错误
//
// 功能逻辑：
// 1. 使用 fs.ReadFile 从嵌入式文件系统中读取文件
// 2. 如果读取失败，记录错误日志并返回错误
// 3. 如果读取成功，记录调试日志（包含文件路径和内容长度）
// 4. 返回文件内容和 nil 错误
//
// 注意事项：
// - 文件名必须是完整的路径，包括目录部分
// - 文件必须存在于嵌入式文件系统中
// - 读取的内容是原始的SQL语句，需要由调用方处理执行
func ReadMigration(fileName string) ([]byte, error) {
	content, err := fs.ReadFile(SchemaMigrations, fileName)
	if err != nil {
		log.Error().Err(err).Str("file", fileName).Msg("读取迁移文件失败")
		return nil, err
	}

	log.Debug().
		Str("file", fileName).
		Int("contentLength", len(content)).
		Msg("成功读取迁移文件内容")

	return content, nil
}
