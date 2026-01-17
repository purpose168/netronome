// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later

// Netronome 认证模块
// 包名: auth
// 功能: 提供密码哈希、令牌签名和验证等认证相关功能
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later

// 主要功能包括：
// 1. 密码哈希和验证（基于 bcrypt 算法）
// 2. 令牌签名和验证（基于 HMAC-SHA256 算法）
// 3. JWT 令牌识别和处理
// 4. 内存令牌标记
//
// 注意事项：
// - 密码哈希使用 bcrypt 算法，提供强安全性
// - 令牌签名支持两种模式：内存模式和 HMAC 签名模式
// - 支持 JWT 令牌的特殊处理
// - 提供了明确的错误常量用于错误处理

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// 错误变量定义
var (
	// ErrPasswordTooShort 密码太短错误
	// 当密码长度少于 8 个字符时返回此错误
	ErrPasswordTooShort = errors.New("密码必须至少 8 个字符")

	// ErrPasswordTooLong 密码太长错误
	// 当密码长度超过 72 个字符时返回此错误
	// 限制长度是因为 bcrypt 算法最多只处理 72 个字符
	ErrPasswordTooLong = errors.New("密码必须少于 72 个字符")

	// ErrInvalidSession 无效会话错误
	// 当令牌无效、签名不匹配或令牌格式错误时返回此错误
	ErrInvalidSession = errors.New("无效会话")
)

//func ValidatePassword(password string) error {
//	if len(password) < 8 {
//		return ErrPasswordTooShort
//	}
//	if len(password) > 72 {
//		return ErrPasswordTooLong
//	}
//	return nil
//}

// HashPassword 对密码进行哈希处理
// 使用 bcrypt 算法对明文密码进行哈希，返回加密后的哈希字符串
// bcrypt 是一种安全的密码哈希算法，具有以下特点：
// - 自动生成随机盐值
// - 可以通过成本参数调整哈希计算的复杂度
// - 最多处理 72 个字符的密码
//
// 参数：
//
//	password: 明文密码字符串
//
// 返回值：
//
//	string: 加密后的哈希字符串（包含盐值信息）
//	error: 如果哈希过程中发生错误，则返回错误信息
func HashPassword(password string) (string, error) {
	// 使用 bcrypt 算法和默认成本参数生成密码哈希
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	// 将字节数组转换为字符串返回
	return string(bytes), nil
}

// CheckPassword 验证密码与哈希值是否匹配
// 使用 bcrypt 算法验证明文密码是否与给定的哈希值匹配
// 该函数是 HashPassword 函数的配对函数，用于密码验证场景
//
// 参数：
//
//	password: 明文密码字符串
//	hash: 加密后的哈希字符串（由 HashPassword 函数生成）
//
// 返回值：
//
//	bool: 如果密码与哈希值匹配返回 true，否则返回 false
//
// 实现原理：
// 1. 将哈希字符串转换为字节数组
// 2. 将明文密码转换为字节数组
// 3. 使用 bcrypt.CompareHashAndPassword 进行比较
// 4. 如果比较成功（err == nil），返回 true；否则返回 false
//
// 注意事项：
// - 该函数不区分密码验证失败的具体原因（如哈希格式错误、密码不匹配等）
// - 始终返回布尔值，便于直接用于条件判断
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// MemoryOnlyPrefix 内存令牌前缀
// 用于标记只应存在于内存中的令牌，不需要持久化存储
// 内存令牌不需要签名，只需要添加此前缀即可
const MemoryOnlyPrefix = "mem_"

// isJWT 判断令牌是否为 JWT 格式
// JWT (JSON Web Token) 是一种用于在各方之间安全传输信息的紧凑、自包含的方式
// JWT 由三个用点分隔的部分组成：
// 1. 头部（Header）：包含令牌类型和签名算法
// 2. 负载（Payload）：包含声明信息
// 3. 签名（Signature）：用于验证令牌的完整性
//
// 参数：
//
//	token: 要检查的令牌字符串
//
// 返回值：
//
//	bool: 如果令牌看起来像 JWT（有 3 个用点分隔的部分）返回 true，否则返回 false
//
// 实现原理：
// 通过检查令牌是否包含恰好 3 个用点分隔的部分来简单判断
// 注意：这只是基本的格式检查，不验证令牌的有效性或签名
func isJWT(token string) bool {
	parts := strings.Split(token, ".")
	return len(parts) == 3
}

// SignToken 对令牌进行签名
// 支持两种令牌处理模式：
// 1. 如果提供了密钥，使用 HMAC-SHA256 算法对令牌进行签名
// 2. 如果未提供密钥，将令牌标记为内存令牌（添加内存令牌前缀）
//
// 参数：
//
//	token: 要签名的令牌字符串
//	secret: 用于签名的密钥字符串
//
// 返回值：
//
//	string: 签名后的令牌字符串
//	- 如果提供了密钥：格式为 "token.signature"（token 是原始令牌，signature 是 HMAC-SHA256 签名的十六进制表示）
//	- 如果未提供密钥：格式为 "mem_token"（添加内存令牌前缀）
//
// 实现原理：
// 1. 检查是否提供了密钥
// 2. 如果没有密钥，直接添加内存令牌前缀返回
// 3. 如果有密钥，使用 HMAC-SHA256 算法生成签名：
//   - 创建 HMAC-SHA256 哈希对象，使用密钥初始化
//   - 写入令牌数据
//   - 计算并获取哈希值
//   - 将哈希值转换为十六进制字符串作为签名
//
// 4. 将原始令牌和签名用点连接起来返回
//
// 安全注意事项：
// - 密钥应妥善保管，不应硬编码在代码中
// - HMAC-SHA256 是一种安全的哈希算法，可有效防止令牌被篡改
func SignToken(token, secret string) string {
	// 如果没有提供密钥，将令牌标记为内存令牌
	if secret == "" {
		return MemoryOnlyPrefix + token
	}

	// 使用 HMAC-SHA256 算法生成签名
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(token))
	signature := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("%s.%s", token, signature)
}

// VerifyToken 验证签名令牌的有效性
// 该函数是 SignToken 函数的配对函数，用于验证签名令牌的完整性和有效性
// 支持三种令牌格式：
// 1. 内存令牌：以 mem_ 前缀开头的令牌，直接返回去除前缀后的内容
// 2. 普通签名令牌：格式为 "token.signature"，包含原始令牌和 HMAC-SHA256 签名
// 3. JWT 签名令牌：格式为 "header.payload.signature.our_signature"，包含 JWT 令牌和额外的 HMAC-SHA256 签名
//
// 参数：
//
//	signedToken: 要验证的签名令牌字符串
//	secret: 用于验证签名的密钥字符串（与 SignToken 函数使用的密钥相同）
//
// 返回值：
//
//	string: 如果令牌有效，返回原始令牌字符串；否则返回空字符串
//	error: 如果令牌无效、签名不匹配或令牌格式错误，返回相应的错误信息
//
// 实现逻辑：
// 1. 检查令牌是否为内存令牌（以 mem_ 前缀开头）
//   - 如果是，直接返回去除前缀后的内容
//
// 2. 检查是否提供了密钥
//   - 如果没有提供密钥，返回无效会话错误
//     3. 根据令牌的点分隔部分数量处理不同的令牌格式：
//     a. 2 个部分：普通签名令牌（token.signature）
//   - 将第一部分作为令牌，第二部分作为签名
//     b. 4 个部分：JWT 签名令牌（header.payload.signature.our_signature）
//   - 前三部分组成 JWT 令牌，第四部分作为我们添加的额外签名
//   - 验证前三部分是否构成有效的 JWT 格式
//     c. 其他数量的部分：返回令牌格式错误
//     4. 生成预期的 HMAC-SHA256 签名：
//   - 创建 HMAC-SHA256 哈希对象，使用密钥初始化
//   - 写入令牌数据
//   - 计算并获取哈希值
//   - 将哈希值转换为十六进制字符串作为预期签名
//     5. 比较实际签名与预期签名是否匹配：
//   - 使用 hmac.Equal 函数进行安全比较，防止时间攻击
//   - 如果不匹配，返回签名无效错误
//     6. 如果所有验证都通过，返回原始令牌
//
// 安全注意事项：
// - 使用 hmac.Equal 函数进行签名比较，防止时间攻击
// - 严格验证令牌格式，拒绝格式错误的令牌
// - 确保密钥的安全性，不应泄露给未授权人员
// - 对于 JWT 令牌，只验证我们添加的额外签名，不验证 JWT 本身的签名
func VerifyToken(signedToken, secret string) (string, error) {
	// 检查是否为内存令牌
	if strings.HasPrefix(signedToken, MemoryOnlyPrefix) {
		return strings.TrimPrefix(signedToken, MemoryOnlyPrefix), nil
	}

	// 检查是否提供了密钥
	if secret != "" {
		parts := strings.Split(signedToken, ".")

		var token, signature string

		// 处理不同的令牌格式
		if len(parts) == 2 {
			// 普通签名令牌：token.signature
			token, signature = parts[0], parts[1]
		} else if len(parts) == 4 {
			// JWT 签名令牌：header.payload.signature.our_signature
			jwtParts := strings.Join(parts[:3], ".")
			if isJWT(jwtParts) {
				token, signature = jwtParts, parts[3]
			} else {
				return "", fmt.Errorf("%w: 令牌格式错误", ErrInvalidSession)
			}
		} else {
			return "", fmt.Errorf("%w: 令牌格式错误", ErrInvalidSession)
		}

		// 生成预期的签名
		expectedSignature := hmac.New(sha256.New, []byte(secret))
		expectedSignature.Write([]byte(token))
		expectedHex := hex.EncodeToString(expectedSignature.Sum(nil))

		// 比较实际签名与预期签名
		if !hmac.Equal([]byte(signature), []byte(expectedHex)) {
			return "", fmt.Errorf("%w: 签名无效", ErrInvalidSession)
		}

		return token, nil
	}

	return "", ErrInvalidSession
}
