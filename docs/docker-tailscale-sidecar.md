# Docker Tailscale 边车容器配置

本指南解释如何使用 Tailscale 边车容器运行 Netronome，以实现安全网络和代理发现功能。

## 概述

当在 Docker 中与 Tailscale 边车容器一起运行 Netronome 时，需要特殊配置来启用 Tailscale 发现功能。边车模式允许您：

- 将 Tailscale 和 Netronome 保持在独立容器中
- 在容器之间共享网络
- 启用自动代理发现
- 保持清晰的关注点分离

## 先决条件

- 已安装 Docker 和 Docker Compose
- 一个 Tailscale 账户和认证密钥
- 对 Docker 网络有基本了解

## 配置

### 使用 Docker Compose 锚点的工作示例

当运行多个 Tailscale 容器时，您可以使用 YAML 锚点实现更简洁的设置：

```yaml
x-tailscale-base: &tailscale-base  # 定义 Tailscale 基础配置锚点
  image: tailscale/tailscale:latest  # Tailscale 镜像
  cap_add:
    - NET_ADMIN  # 添加网络管理权限
  restart: unless-stopped  # 除非手动停止，否则自动重启
  networks:
    - tailscale_network  # 使用自定义网络

services:
  netronome:
    image: ghcr.io/autobrr/netronome:latest  # Netronome 最新镜像
    container_name: netronome  # 容器名称
    user: 1000:1000  # 运行用户和组 ID
    restart: unless-stopped  # 除非手动停止，否则自动重启
    env_file: .env  # 从 .env 文件加载环境变量
    volumes:
      - "./netronome:/data"  # 持久化数据目录
      - tailscale-socket:/var/run/tailscale  # 共享 Tailscale 套接字
    depends_on:
      - netronome-ts  # 依赖于 Tailscale 容器
    network_mode: service:netronome-ts  # 共享 Tailscale 容器的网络命名空间

  netronome-ts:
    <<: *tailscale-base  # 引用 Tailscale 基础配置
    container_name: netronome-ts  # Tailscale 容器名称
    hostname: netronome  # Tailscale 主机名
    environment:
      - TS_AUTHKEY=${TS_AUTHKEY}  # Tailscale 认证密钥
      - TS_STATE_DIR=${TS_STATE_DIR}  # Tailscale 状态目录
      - TS_EXTRA_ARGS=${TS_EXTRA_ARGS}  # Tailscale 额外参数
      - TZ=${TZ}  # 时区设置
      - TS_SERVE_CONFIG=/config/netronome.json  # Tailscale Serve 配置文件
      - TS_SOCKET=/var/run/tailscale/tailscaled.sock  # Tailscale 套接字位置
    volumes:
      - /dev/net/tun:/dev/net/tun  # 挂载 tun 设备
      - ${BASE_DOCKER_DATA_PATH}/config:/config  # 配置文件目录
      - tailscale-data-netronome:/var/lib/tailscale  # Tailscale 数据持久化
      - tailscale-socket:/var/run/tailscale  # 共享套接字目录

volumes:
  tailscale-data-netronome:  # Tailscale 数据卷
  tailscale-socket:  # 用于套接字共享的命名卷

networks:
  tailscale_network:  # 自定义网络
    ipam:
      config:
        - subnet: 172.19.0.0/16  # 子网配置
```

### 基本配置

如果您只需要使用 Tailscale 运行 Netronome，以下是更简单的配置：

```yaml
services:
  netronome:
    image: ghcr.io/autobrr/netronome:latest  # Netronome 最新镜像
    container_name: netronome  # 容器名称
    user: 1000:1000  # 运行用户和组 ID
    restart: unless-stopped  # 除非手动停止，否则自动重启
    env_file: .env  # 从 .env 文件加载环境变量
    volumes:
      - "./netronome:/data"  # 持久化数据目录
      - tailscale-socket:/var/run/tailscale  # 共享 Tailscale 套接字目录
    depends_on:
      - netronome-ts  # 依赖于 Tailscale 容器
    network_mode: service:netronome-ts  # 共享 Tailscale 容器的网络命名空间

  netronome-ts:
    image: tailscale/tailscale:latest  # Tailscale 最新镜像
    container_name: netronome-ts  # Tailscale 容器名称
    hostname: netronome  # Tailscale 主机名（这将是您的 Tailscale 节点名称）
    cap_add:
      - NET_ADMIN  # 添加网络管理权限
    environment:
      - TS_AUTHKEY=${TS_AUTHKEY}  # Tailscale 认证密钥
      - TS_STATE_DIR=/var/lib/tailscale  # Tailscale 状态目录
      - TS_EXTRA_ARGS=${TS_EXTRA_ARGS}  # Tailscale 额外参数
      - TZ=${TZ}  # 时区设置
      - TS_SERVE_CONFIG=/config/netronome.json  # Tailscale Serve 配置
      - TS_SOCKET=/var/run/tailscale/tailscaled.sock  # 强制设置套接字位置
    volumes:
      - /dev/net/tun:/dev/net/tun  # 挂载 tun 设备以支持 VPN
      - ./config:/config  # 配置文件目录
      - tailscale-data-netronome:/var/lib/tailscale  # Tailscale 数据持久化
      - tailscale-socket:/var/run/tailscale  # 共享套接字目录

volumes:
  tailscale-data-netronome:  # Tailscale 数据持久化卷
  tailscale-socket:  # 用于套接字共享的命名卷
```

### 环境变量 (.env 文件)

```bash
# Tailscale 配置
TS_AUTHKEY=tskey-auth-YOUR-KEY-HERE  # Tailscale 认证密钥
TS_STATE_DIR=/var/lib/tailscale  # Tailscale 状态目录
TS_EXTRA_ARGS=--advertise-routes=192.168.1.0/24  # 可选：广播本地网络路由
TZ=America/New_York  # 时区设置

# Netronome 配置（可选）
NETRONOME__TAILSCALE_ENABLED=true  # 启用 Tailscale 集成
NETRONOME__TAILSCALE_METHOD=host  # 使用主机的 tailscaled 实例
```

### 通过 Tailscale Serve 提供 HTTPS 服务

要通过 Tailscale 提供带有 HTTPS 证书的 Netronome 服务，请在配置目录中创建一个 `netronome.json` 文件：

```json
{
    "TCP": {
        "443": {
            "HTTPS": true  // 在 443 端口启用 HTTPS
        }
    },
    "Web": {
        "${TS_CERT_DOMAIN}:443": {
            "Handlers": {
                "/": {
                    "Proxy": "http://127.0.0.1:7575"  // 将请求代理到 Netronome
                }
            }
        }
    },
    "AllowFunnel": {
        "${TS_CERT_DOMAIN}:443": false  // 禁用 Funnel 功能（保持服务私有）
    }
}
```

此配置：
- 使用 Tailscale 自动证书在 443 端口启用 HTTPS
- 将所有请求代理到运行在 7575 端口的 Netronome
- 保持服务对您的 tailnet 私有（禁用 Funnel）

`${TS_CERT_DOMAIN}` 变量由 Tailscale 自动填充为您节点的完整域名。

## 关键配置要点

### 1. 套接字共享

最关键的部分是在容器之间共享 Tailscale 套接字：

```yaml
volumes:
  - tailscale-socket:/var/run/tailscale  # 两个容器都挂载此卷
```

并在 Tailscale 容器中强制设置套接字位置：

```yaml
environment:
  - TS_SOCKET=/var/run/tailscale/tailscaled.sock  # 强制设置套接字位置
```

### 2. 网络模式

使用 `network_mode: service:netronome-ts` 共享网络命名空间：

```yaml
netronome:
  network_mode: service:netronome-ts  # 与 Tailscale 容器共享网络
```

这允许 Netronome 访问 Tailscale 网络接口。

### 3. Netronome 配置

配置 Netronome 使用主机的 tailscaled（实际上是在边车容器中）：

```toml
# 在您的 config.toml 文件或通过环境变量配置
[tailscale]
enabled = true  # 启用 Tailscale
method = "host"  # 使用主机模式连接到边车的 tailscaled
auto_discover = true  # 启用自动代理发现
discovery_interval = "5m"  # 发现间隔
```

## 故障排除

### 套接字连接问题

如果您看到类似 "no running tailscaled found on host" 的错误：

1. **验证套接字路径**：检查 Tailscale 容器是否在 `/var/run/tailscale/tailscaled.sock` 创建了套接字
   ```bash
   docker exec netronome-ts ls -la /var/run/tailscale/  # 列出套接字目录内容
   ```

2. **检查卷挂载**：确保两个容器都挂载了套接字卷
   ```bash
   docker inspect netronome | grep -A5 Mounts  # 检查 Netronome 容器挂载
   docker inspect netronome-ts | grep -A5 Mounts  # 检查 Tailscale 容器挂载
   ```

3. **测试套接字连接**：从 Netronome 容器内部测试连接
   ```bash
   docker exec netronome curl --unix-socket /var/run/tailscale/tailscaled.sock http://local-tailscaled.sock/localapi/v0/status  # 测试套接字连接
   ```

### 发现功能不工作

如果代理没有被发现：

1. **检查 Tailscale 状态**：
   ```bash
   docker exec netronome-ts tailscale status  # 查看 Tailscale 状态
   ```

2. **验证代理是否在发现端口上**：确保代理运行在 8200 端口（或您配置的发现端口）

3. **检查日志**：查找与发现相关的消息
   ```bash
   docker logs netronome | grep -i tailscale  # 搜索包含 tailscale 的日志
   ```

## 替代方案

### 选项 1：TSNet 模式（独立 Tailscale 实例）

如果您希望 Netronome 拥有自己的 Tailscale 身份：

```yaml
netronome:
  image: ghcr.io/autobrr/netronome:latest  # Netronome 最新镜像
  environment:
    - NETRONOME__TAILSCALE_ENABLED=true  # 启用 Tailscale
    - NETRONOME__TAILSCALE_METHOD=tsnet  # 使用 TSNet 模式（内置 Tailscale）
    - NETRONOME__TAILSCALE_AUTH_KEY=tskey-auth-YOUR-KEY  # Tailscale 认证密钥
    - NETRONOME__TAILSCALE_HOSTNAME=netronome-monitor  # Tailscale 主机名
  volumes:
    - "./netronome:/data"  # 持久化数据目录
  ports:
    - 7575:7575  # 暴露 Netronome 端口
```

此方法不需要边车容器。

### 选项 2：主机网络模式

如果在 Linux 上运行，您可以使用主机的网络和 tailscaled：

```yaml
netronome:
  image: ghcr.io/autobrr/netronome:latest  # Netronome 最新镜像
  network_mode: host  # 使用主机网络
  environment:
    - NETRONOME__TAILSCALE_ENABLED=true  # 启用 Tailscale
    - NETRONOME__TAILSCALE_METHOD=host  # 使用主机的 tailscaled
  volumes:
    - "./netronome:/data"  # 持久化数据目录
    - /var/run/tailscale:/var/run/tailscale:ro  # 只读挂载主机的 Tailscale 套接字
```

## 最佳实践

1. **使用命名卷** 存储套接字目录，以确保正确的权限
2. **设置明确的套接字路径** 以避免自动检测问题
3. **监控日志** 在初始设置期间捕捉配置问题
4. **测试连接** 在启用自动发现之前验证连接
5. **使用环境变量** 存储敏感数据，如认证密钥

## 安全考虑

- 套接字共享授予 Netronome 对 Tailscale 守护进程的完全访问权限
- 尽可能考虑使用只读挂载
- 在代理上使用 API 密钥进行额外认证
- 定期轮换 Tailscale 认证密钥

## 其他资源

- [Tailscale Docker 文档](https://tailscale.com/kb/1282/docker)（英文）
- [Docker 网络文档](https://docs.docker.com/network/)（英文）
- [Netronome Tailscale 集成指南](../README.md#tailscale-integration)