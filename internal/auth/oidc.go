// Copyright (c) 2024-2025, s0up 和 autobrr 贡献者.
// SPDX-License-Identifier: GPL-2.0-or-later

// Netronome OpenID Connect 认证模块
// 包名: auth
// 功能: 提供基于 OpenID Connect (OIDC) 协议的认证功能实现
// 作者: s0up 和 autobrr 贡献者
// 创建日期: 2024
// 许可证: GPL-2.0-or-later

// OpenID Connect (OIDC) 是一种基于 OAuth 2.0 协议的身份认证标准：
// 1. 允许应用程序验证用户身份并获取基本用户信息
// 2. 提供单一登录 (SSO) 功能
// 3. 使用 JSON Web Tokens (JWT) 作为身份令牌
// 4. 支持多种认证流程（授权码流、隐式流等）

// 该模块实现的主要功能：
// 1. OIDC 提供商配置和发现
// 2. PKCE (Proof Key for Code Exchange) 支持，增强授权码流的安全性
// 3. JWE (JSON Web Encryption) 令牌解密
// 4. ID 令牌验证
// 5. 用户声明获取
// 6. OAuth 2.0 授权码交换

// 依赖的主要库：
// - github.com/coreos/go-oidc/v3/oidc: OIDC 协议实现
// - github.com/go-jose/go-jose/v4: JWE 加密和解密
// - golang.org/x/oauth2: OAuth 2.0 协议实现

// 使用场景：
// 1. Web 应用程序的用户认证
// 2. API 服务的身份验证和授权
// 3. 与企业身份提供商集成
// 4. 支持单点登录 (SSO) 功能

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"

	"github.com/autobrr/netronome/internal/config"
)

// OIDCConfig 保存 OIDC 提供商的配置信息和客户端实例
// 包含与 OIDC 提供商交互所需的所有组件
// - provider: OIDC 提供商实例，用于获取提供商的配置和公钥
// - OAuth2Config: OAuth 2.0 客户端配置，包含客户端 ID、密钥、重定向 URL 等
// - verifier: ID 令牌验证器，用于验证 ID 令牌的有效性
//
// 该结构体是 OIDC 认证模块的核心，封装了与 OIDC 提供商交互的所有功能
// 包括令牌生成、验证、用户信息获取等
// 使用场景：
// - Web 应用程序的用户认证流程
// - API 服务的身份验证和授权
// - 与企业身份提供商集成
// - 支持单点登录 (SSO) 功能
type OIDCConfig struct {
	provider     *oidc.Provider        // OIDC 提供商实例
	OAuth2Config oauth2.Config         // OAuth 2.0 客户端配置
	verifier     *oidc.IDTokenVerifier // ID 令牌验证器
}

// Claims 表示从 OIDC ID 令牌中提取的用户声明信息
// 这些声明包含有关用户身份的基本信息
// - Subject: 用户的唯一标识符 (sub)
// - Name: 用户的完整名称 (name)
// - Username: 用户的首选用户名 (preferred_username)
//
// 声明是 OIDC 协议中的核心概念，用于传递用户信息
// 它们以 JSON 格式存储在 ID 令牌中，并由 OIDC 提供商签名
// 应用程序可以使用这些声明来识别用户并提供个性化服务
//
// 使用场景：
// - 显示用户的名称和用户名
// - 基于用户身份进行访问控制
// - 个性化应用程序体验
// - 与其他系统共享用户身份信息
type Claims struct {
	Subject  string `json:"sub"`                // 用户的唯一标识符
	Name     string `json:"name"`               // 用户的完整名称
	Username string `json:"preferred_username"` // 用户的首选用户名
}

// PKCEParams 保存 PKCE (Proof Key for Code Exchange) 流程的参数
// PKCE 是 OAuth 2.0 的扩展，用于增强公共客户端的安全性
// - CodeVerifier: 43-128 字符的随机字符串，用于证明客户端的身份
// - CodeChallenge: CodeVerifier 的 SHA256 哈希值，使用 base64url 编码
//
// PKCE 流程的工作原理：
// 1. 客户端生成 CodeVerifier 和 CodeChallenge
// 2. 客户端使用 CodeChallenge 请求授权码
// 3. 提供商验证 CodeChallenge 并返回授权码
// 4. 客户端使用授权码和原始 CodeVerifier 交换令牌
// 5. 提供商验证 CodeVerifier 和之前的 CodeChallenge 是否匹配
//
// 使用场景：
// - 移动应用程序的认证
// - 单页应用程序 (SPA) 的认证
// - 任何无法安全存储客户端密钥的公共客户端
type PKCEParams struct {
	CodeVerifier  string // PKCE 代码验证器，客户端保留的原始随机字符串
	CodeChallenge string // PKCE 代码挑战，发送给授权服务器的哈希值
}

// GeneratePKCEParams 生成 PKCE 代码验证器和挑战值
// 该函数实现了 RFC 7636 中定义的 PKCE 扩展
// 用于增强 OAuth 2.0 授权码流的安全性，特别是对于公共客户端
//
// 返回值：
// - *PKCEParams: 包含代码验证器和挑战值的结构体指针
// - error: 如果生成过程中发生错误，则返回相应的错误信息
//
// 生成过程：
// 1. 生成 96 字节的随机字节数组（对应 128 个 base64url 字符）
// 2. 使用 base64url 编码将随机字节数组转换为代码验证器
// 3. 对代码验证器进行 SHA256 哈希
// 4. 使用 base64url 编码将哈希值转换为代码挑战
//
// 使用场景：
// - 在发起授权请求前调用此函数生成 PKCE 参数
// - 将代码挑战包含在授权请求中
// - 保留代码验证器用于后续的令牌交换
func GeneratePKCEParams() (*PKCEParams, error) {
	// 生成 43-128 字符的随机字符串作为代码验证器
	// 96 字节 = 128 个 base64url 字符，符合 PKCE 规范的最大长度
	verifierBytes := make([]byte, 96)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("生成 PKCE 代码验证器失败: %w", err)
	}

	// 使用 base64url 编码（不包含填充）将随机字节转换为字符串
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	// 使用 SHA256 哈希生成代码挑战
	challengeBytes := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])

	return &PKCEParams{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
	}, nil
}

// isJWE 检查令牌是否为 JWE (JSON Web Encryption) 格式
// JWE 令牌由 5 个部分组成，用点号分隔：
// 1. 受保护的头信息 (Protected Header)
// 2. 加密的密钥 (Encrypted Key)
// 3. 初始向量 (Initialization Vector)
// 4. 密文 (Ciphertext)
// 5. 认证标签 (Authentication Tag)
//
// 参数：
// - token: 要检查的令牌字符串
//
// 返回值：
// - bool: 如果令牌是 JWE 格式，则返回 true；否则返回 false
//
// 使用场景：
// - 在验证令牌前检查其格式
// - 根据令牌格式选择不同的处理方式
// - 识别需要解密的加密令牌
//
// 注意：此函数仅通过格式检查判断，不验证令牌的有效性
func isJWE(token string) bool {
	parts := strings.Split(token, ".")
	return len(parts) == 5 // JWE 格式必须有 5 个部分
}

// decryptJWE 使用客户端密钥解密 JWE (JSON Web Encryption) 令牌
// 支持多种密钥算法和内容加密算法：
// - 密钥算法：DIRECT (直接加密)、A128KW、A192KW、A256KW (密钥包装)
// - 内容加密算法：A128GCM、A192GCM、A256GCM (AES-GCM 模式)
//
// 参数：
// - jweToken: 要解密的 JWE 令牌字符串
//
// 返回值：
// - string: 解密后的明文令牌（通常是 JWT 格式）
// - error: 如果解密失败，则返回相应的错误信息
//
// 解密过程：
// 1. 解析 JWE 令牌
// 2. 获取客户端密钥作为解密密钥
// 3. 调整密钥长度以适应 AES 加密算法（16、24 或 32 字节）
// 4. 使用调整后的密钥尝试解密
// 5. 如果解密失败，尝试使用客户端密钥的 SHA256 哈希作为密钥
// 6. 如果仍然失败，尝试使用哈希的前 16 字节作为密钥
//
// 使用场景：
// - 处理加密的 ID 令牌或访问令牌
// - 从安全的令牌存储中检索敏感信息
// - 与要求加密令牌的 OIDC 提供商交互
//
// 注意：JWE 解密需要客户端密钥，因此客户端密钥必须安全存储
func (c *OIDCConfig) decryptJWE(jweToken string) (string, error) {
	// 解析 JWE 令牌
	jwe, err := jose.ParseEncrypted(jweToken,
		[]jose.KeyAlgorithm{jose.DIRECT, jose.A128KW, jose.A192KW, jose.A256KW},
		[]jose.ContentEncryption{jose.A128GCM, jose.A192GCM, jose.A256GCM})
	if err != nil {
		return "", fmt.Errorf("解析 JWE 令牌失败: %w", err)
	}

	// 获取客户端密钥作为解密密钥
	clientSecret := c.OAuth2Config.ClientSecret
	if clientSecret == "" {
		return "", fmt.Errorf("JWE 解密需要客户端密钥")
	}

	// 调整客户端密钥长度以适应 AES 加密算法
	// AES 支持的密钥长度：16 字节 (AES-128)、24 字节 (AES-192)、32 字节 (AES-256)
	key := []byte(clientSecret)
	if len(key) < 16 {
		// 如果密钥太短，用零填充到 16 字节
		padded := make([]byte, 16)
		copy(padded, key)
		key = padded
	} else if len(key) > 32 {
		// 如果密钥太长，截断到 32 字节
		key = key[:32]
	} else if len(key) > 16 && len(key) < 24 {
		// 如果密钥长度在 16-24 字节之间，填充到 24 字节
		padded := make([]byte, 24)
		copy(padded, key)
		key = padded
	} else if len(key) > 24 && len(key) < 32 {
		// 如果密钥长度在 24-32 字节之间，填充到 32 字节
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	}

	// 尝试使用客户端密钥解密
	decrypted, err := jwe.Decrypt(key)
	if err != nil {
		// 如果解密失败，尝试使用客户端密钥的 SHA256 哈希作为密钥（常见模式）
		hasher := sha256.New()
		hasher.Write([]byte(clientSecret))
		hashedKey := hasher.Sum(nil)

		// 尝试使用完整的哈希值（32 字节）
		decrypted, err = jwe.Decrypt(hashedKey)
		if err != nil {
			// 尝试使用哈希的前 16 字节
			decrypted, err = jwe.Decrypt(hashedKey[:16])
			if err != nil {
				return "", fmt.Errorf("使用客户端密钥或哈希解密 JWE 令牌失败: %w", err)
			}
		}
	}

	log.Debug().Msg("JWE 令牌解密成功")
	return string(decrypted), nil
}

// NewOIDC 创建并初始化新的 OIDC 配置实例
// 根据提供的配置连接到 OIDC 提供商并设置认证所需的所有组件
//
// 参数：
// - ctx: 上下文对象，用于控制函数的生命周期和取消操作
// - cfg: OIDC 配置，包含提供商 URL、客户端 ID、客户端密钥等信息
//
// 返回值：
// - *OIDCConfig: 初始化完成的 OIDC 配置实例
// - error: 如果初始化失败，则返回相应的错误信息
//
// 初始化过程：
// 1. 检查是否配置了 OIDC 提供商（Issuer）
// 2. 如果没有配置，使用内置认证并返回 nil
// 3. 执行 OIDC 提供商端点发现（手动方式）
// 4. 初始化 OIDC 提供商实例
// 5. 创建 OAuth 2.0 客户端配置
// 6. 创建 ID 令牌验证器
// 7. 返回完整的 OIDC 配置实例
//
// 使用场景：
// - 在应用程序启动时初始化 OIDC 认证
// - 切换内置认证和 OIDC 认证
// - 与不同的 OIDC 提供商集成
//
// 注意：如果 OIDC 配置不完整或提供商不可用，将返回 nil 和相应的错误
func NewOIDC(ctx context.Context, cfg config.OIDCConfig) (*OIDCConfig, error) {
	// 检查是否配置了 OIDC 提供商
	if cfg.Issuer == "" {
		log.Debug().Msg("使用内置认证")
		return nil, nil
	}

	log.Debug().Str("issuer", cfg.Issuer).Msg("初始化 OIDC 提供商")

	// 执行 OIDC 提供商端点发现（手动方式）
	endpoints, _, err := getProviderEndpoints(ctx, http.DefaultClient, cfg.Issuer)
	if err != nil {
		log.Error().Err(err).Str("issuer", cfg.Issuer).Msg("发现 OIDC 提供商端点失败")
		return nil, fmt.Errorf("发现 OIDC 提供商端点失败: %w", err)
	}

	// 初始化 OIDC 提供商实例
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		log.Error().Err(err).Str("issuer", cfg.Issuer).Msg("初始化 OIDC 提供商失败")
		return nil, fmt.Errorf("初始化 OIDC 提供商失败: %w", err)
	}

	// 创建 OAuth 2.0 客户端配置
	config := oauth2.Config{
		ClientID:     cfg.ClientID,                          // 客户端 ID，由 OIDC 提供商分配
		ClientSecret: cfg.ClientSecret,                      // 客户端密钥，由 OIDC 提供商分配
		RedirectURL:  cfg.RedirectURL,                       // 认证成功后的重定向 URL
		Endpoint:     endpoints,                             // OIDC 提供商的授权和令牌端点
		Scopes:       []string{oidc.ScopeOpenID, "profile"}, // 请求的权限范围
	}

	log.Trace().
		Str("clientID", config.ClientID).
		Str("redirectURL", config.RedirectURL).
		Str("authURL", endpoints.AuthURL).
		Str("tokenURL", endpoints.TokenURL).
		Strs("scopes", config.Scopes).
		Msg("OIDC 配置创建完成")

	// 创建并返回完整的 OIDC 配置实例
	return &OIDCConfig{
		provider:     provider,                                                   // OIDC 提供商实例
		OAuth2Config: config,                                                     // OAuth 2.0 客户端配置
		verifier:     provider.Verifier(&oidc.Config{ClientID: config.ClientID}), // ID 令牌验证器
	}, nil
}

// getProviderEndpoints 从 OIDC 提供商获取授权和令牌端点 URL
// 实现了 OIDC 发现协议，通过请求提供商的发现文档获取端点信息
//
// 参数：
// - ctx: 上下文对象，用于控制请求的生命周期和取消操作
// - client: HTTP 客户端，用于发送请求
// - issuer: OIDC 提供商的发行者 URL
//
// 返回值：
// - oauth2.Endpoint: 包含授权和令牌端点的 OAuth 2.0 端点结构体
// - string: 用户信息端点 URL
// - error: 如果获取端点失败，则返回相应的错误信息
//
// 发现过程：
// 1. 清理发行者 URL，移除末尾的斜杠
// 2. 构建 OIDC 发现文档的 URL（.well-known/openid-configuration）
// 3. 发送 HTTP GET 请求获取发现文档
// 4. 解析发现文档 JSON，提取授权、令牌和用户信息端点
// 5. 返回提取的端点信息
//
// 使用场景：
// - 初始化 OIDC 提供商时获取端点信息
// - 手动实现 OIDC 发现过程
// - 与不支持自动发现的 OIDC 提供商交互
//
// 注意：OIDC 发现文档包含提供商的所有配置信息，包括支持的算法、端点等
func getProviderEndpoints(ctx context.Context, client *http.Client, issuer string) (oauth2.Endpoint, string, error) {
	// 清理发行者 URL，移除末尾的斜杠
	issuer = strings.TrimRight(issuer, "/")

	// 构建 OIDC 发现文档的 URL
	wellKnown := issuer + "/.well-known/openid-configuration"
	// 如果发行者 URL 已经包含发现文档路径，则直接使用
	if strings.Contains(issuer, "/.well-known/openid-configuration") {
		wellKnown = issuer
	}

	log.Trace().Str("well_known_url", wellKnown).Msg("获取 OIDC 发现文档")

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "GET", wellKnown, nil)
	if err != nil {
		return oauth2.Endpoint{}, "", fmt.Errorf("创建发现请求失败: %w", err)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return oauth2.Endpoint{}, "", fmt.Errorf("获取发现文档失败: %w", err)
	}
	defer resp.Body.Close() // 确保响应体被关闭

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return oauth2.Endpoint{}, "", fmt.Errorf("读取发现文档失败: %w", err)
	}

	// 解析发现文档 JSON
	var discovery struct {
		Issuer      string `json:"issuer"`                 // OIDC 提供商的发行者标识符
		AuthURL     string `json:"authorization_endpoint"` // 授权端点 URL
		TokenURL    string `json:"token_endpoint"`         // 令牌端点 URL
		UserinfoURL string `json:"userinfo_endpoint"`      // 用户信息端点 URL
		JWKSURL     string `json:"jwks_uri"`               // JSON Web Key Set 端点 URL
	}

	if err := json.Unmarshal(body, &discovery); err != nil {
		return oauth2.Endpoint{}, "", fmt.Errorf("解析发现文档失败: %w", err)
	}

	log.Debug().
		Str("issuer", discovery.Issuer).
		Str("auth_url", discovery.AuthURL).
		Str("token_url", discovery.TokenURL).
		Msg("OIDC 发现成功")

	// 返回提取的端点信息
	return oauth2.Endpoint{
		AuthURL:  discovery.AuthURL,  // 授权端点 URL
		TokenURL: discovery.TokenURL, // 令牌端点 URL
	}, discovery.UserinfoURL, nil
}

// AuthURL 生成 OIDC 授权 URL
// 使用默认的固定状态值 "state" 生成授权 URL
//
// 返回值：
// - string: 完整的 OIDC 授权 URL
//
// 授权 URL 用于引导用户到 OIDC 提供商的登录页面
// 用户登录成功后，会被重定向回应用程序的重定向 URL
//
// 使用场景：
// - 简单的认证流程，不需要复杂的状态管理
// - 测试和调试 OIDC 认证流程
//
// 注意：生产环境中应该使用随机生成的状态值来防止 CSRF 攻击
// 推荐使用 AuthURLWithPKCE 函数，它提供了更安全的 PKCE 支持
func (c *OIDCConfig) AuthURL() string {
	// 使用固定状态值 "state" 生成授权 URL
	return c.OAuth2Config.AuthCodeURL("state")
}

// AuthURLWithPKCE 生成带有 PKCE 参数的 OIDC 授权 URL
// 增强了授权码流的安全性，特别适合公共客户端
//
// 参数：
// - state: 随机生成的状态值，用于防止 CSRF 攻击
// - pkce: PKCE 参数，包含代码挑战和验证器
//
// 返回值：
// - string: 带有 PKCE 参数的完整 OIDC 授权 URL
//
// 授权 URL 包含以下关键参数：
// - client_id: 客户端 ID
// - redirect_uri: 重定向 URL
// - response_type: 响应类型（code 表示授权码流）
// - scope: 请求的权限范围
// - state: 随机生成的状态值
// - code_challenge: PKCE 代码挑战
// - code_challenge_method: PKCE 代码挑战方法（S256 表示 SHA256 哈希）
//
// 使用场景：
// - 移动应用程序的认证
// - 单页应用程序 (SPA) 的认证
// - 任何无法安全存储客户端密钥的公共客户端
// - 需要增强安全性的生产环境
//
// 注意：
// - 状态值应该是随机生成的，并且与用户会话关联
// - 代码验证器应该安全保存，用于后续的令牌交换
func (c *OIDCConfig) AuthURLWithPKCE(state string, pkce *PKCEParams) string {
	// 生成带有 PKCE 参数的授权 URL
	return c.OAuth2Config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", pkce.CodeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"))
}

// ExchangeCodeWithPKCE 使用 PKCE 交换授权码获取令牌
// 实现了 OAuth 2.0 授权码流的令牌交换步骤
//
// 参数：
// - ctx: 上下文对象，用于控制请求的生命周期和取消操作
// - code: 授权服务器返回的授权码
// - codeVerifier: PKCE 代码验证器，与之前发送的代码挑战对应
//
// 返回值：
// - *oauth2.Token: 包含访问令牌、ID 令牌和刷新令牌的令牌对象
// - error: 如果交换失败，则返回相应的错误信息
//
// 交换过程：
// 1. 向令牌端点发送 POST 请求
// 2. 包含授权码、重定向 URL、客户端 ID 等参数
// 3. 包含 PKCE 代码验证器，用于验证客户端的身份
// 4. 接收包含访问令牌、ID 令牌和刷新令牌的响应
//
// 使用场景：
// - 移动应用程序的认证流程
// - 单页应用程序 (SPA) 的认证流程
// - 任何使用 PKCE 增强安全性的认证流程
//
// 注意：
// - 授权码只能使用一次，使用后会失效
// - 代码验证器必须与之前发送的代码挑战对应
// - 令牌应该安全存储，防止泄露
func (c *OIDCConfig) ExchangeCodeWithPKCE(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error) {
	// 使用授权码和 PKCE 代码验证器交换令牌
	return c.OAuth2Config.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier))
}

// VerifyToken 验证 OIDC ID 令牌的有效性
// 支持解密 JWE 格式的令牌并验证 JWT 格式的令牌
//
// 参数：
// - ctx: 上下文对象，用于控制验证过程的生命周期和取消操作
// - token: 要验证的 ID 令牌（可以是 JWE 或 JWT 格式）
//
// 返回值：
// - error: 如果令牌无效，则返回相应的错误信息；否则返回 nil
//
// 验证过程：
// 1. 检查令牌是否为 JWE 格式，如果是则解密
// 2. 使用 ID 令牌验证器验证 JWT 令牌的签名、发行者、受众等
// 3. 解析令牌的声明
// 4. 验证主题 (sub) 声明是否存在
// 5. 验证令牌是否过期
//
// 使用场景：
// - API 请求的身份验证
// - 用户会话的验证
// - 授权访问受保护资源
//
// 注意：
// - 令牌验证是安全的关键步骤，应该在所有需要认证的请求中执行
// - 验证过程包括检查令牌的签名、发行者、受众、过期时间等
// - 如果令牌是 JWE 格式，会自动解密为 JWT 格式后再验证
func (c *OIDCConfig) VerifyToken(ctx context.Context, token string) error {
	// 检查令牌是否为 JWE 格式，如果是则解密
	var verifyToken string = token
	if isJWE(token) {
		log.Debug().Msg("检测到 JWE 令牌，尝试解密")
		decrypted, err := c.decryptJWE(token)
		if err != nil {
			log.Error().Err(err).Msg("JWE 令牌解密失败")
			return fmt.Errorf("JWE 令牌解密失败: %w", err)
		}
		verifyToken = decrypted
		log.Debug().Msg("JWE 令牌解密成功")
	}

	// 使用 ID 令牌验证器验证令牌
	idToken, err := c.verifier.Verify(ctx, verifyToken)
	if err != nil {
		return fmt.Errorf("无效令牌: %w", err)
	}

	// 解析令牌的声明
	var claims struct {
		Subject string `json:"sub"` // 主题（用户的唯一标识符）
		Expiry  int64  `json:"exp"` // 过期时间（Unix 时间戳）
	}
	if err := idToken.Claims(&claims); err != nil {
		return fmt.Errorf("解析声明失败: %w", err)
	}

	// 验证主题声明是否存在
	if claims.Subject == "" {
		return fmt.Errorf("令牌缺少主题声明")
	}

	// 验证令牌是否过期
	now := time.Now()
	if claims.Expiry == 0 {
		// 如果令牌没有过期时间，默认设置为 24 小时后过期
		claims.Expiry = now.Add(24 * time.Hour).Unix()
	}

	if now.After(time.Unix(claims.Expiry, 0)) {
		return fmt.Errorf("令牌已过期")
	}

	return nil
}

// GetClaims 从 OIDC ID 令牌中提取用户声明信息
// 支持解密 JWE 格式的令牌并提取 JWT 格式令牌的声明
//
// 参数：
// - ctx: 上下文对象，用于控制提取过程的生命周期和取消操作
// - token: 要提取声明的 ID 令牌（可以是 JWE 或 JWT 格式）
//
// 返回值：
// - *Claims: 包含用户声明信息的结构体指针
// - error: 如果提取失败，则返回相应的错误信息
//
// 提取过程：
// 1. 检查令牌是否为 JWE 格式，如果是则解密
// 2. 使用 ID 令牌验证器验证 JWT 令牌的有效性
// 3. 解析令牌的声明并映射到 Claims 结构体
// 4. 返回提取的声明信息
//
// 使用场景：
// - 获取用户的基本信息（姓名、用户名等）
// - 基于用户声明进行授权决策
// - 个性化应用程序体验
// - 存储用户信息到本地数据库
//
// 注意：
// - 只有有效的令牌才能提取声明
// - 声明的内容取决于 OIDC 提供商配置的作用域和返回的信息
// - 常见的声明包括 sub (主题)、name (姓名)、preferred_username (用户名) 等
func (c *OIDCConfig) GetClaims(ctx context.Context, token string) (*Claims, error) {
	// 检查令牌是否为 JWE 格式，如果是则解密
	var verifyToken string = token
	if isJWE(token) {
		log.Debug().Msg("在 GetClaims 中检测到 JWE 令牌，尝试解密")
		decrypted, err := c.decryptJWE(token)
		if err != nil {
			log.Error().Err(err).Msg("在 GetClaims 中 JWE 令牌解密失败")
			return nil, fmt.Errorf("JWE 令牌解密失败: %w", err)
		}
		verifyToken = decrypted
		log.Debug().Msg("在 GetClaims 中 JWE 令牌解密成功")
	}

	// 使用 ID 令牌验证器验证令牌并提取声明
	idToken, err := c.verifier.Verify(ctx, verifyToken)
	if err != nil {
		return nil, fmt.Errorf("无效令牌: %w", err)
	}

	// 解析令牌的声明到 Claims 结构体
	var claims Claims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("解析声明失败: %w", err)
	}

	return &claims, nil
}
