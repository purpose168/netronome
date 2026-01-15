# CI/CD Docker 构建文件 - 用于自动化构建环境
# 此文件专为持续集成/持续部署流程设计，支持多平台构建

# 使用 Go 1.24 作为后端构建基础镜像，支持多平台构建
FROM --platform=$BUILDPLATFORM golang:1.24-alpine3.22 AS app-builder

ARG VERSION=dev
ARG REVISION=dev
ARG BUILDTIME
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG GITHUB_TOKEN

RUN apk add --no-cache git tzdata curl ca-certificates

ENV SERVICE=netronome
ENV CGO_ENABLED=0

# 设置后端构建工作目录
WORKDIR /src

# 缓存 Go 模块依赖（提高构建速度）
COPY go.mod go.sum ./
RUN go mod download

# 复制所有后端源代码
COPY . ./

# 下载并安装适合目标平台的 librespeed-cli 工具
RUN case "${TARGETOS}-${TARGETARCH}" in \
    "linux-amd64") \
        SPEEDTEST_ARCH="amd64"; \
        SPEEDTEST_CHECKSUM="78bf5cd10fc00006224efb82f5767b95ba414a7ca4dc315a7ed2b774ed80813c" ;; \
    "linux-arm64") \
        SPEEDTEST_ARCH="arm64"; \
        SPEEDTEST_CHECKSUM="e8d6fb054aee8bbfcad8c9f70fc89e983800f64221cfbb9b62ba1842bfcad6d4" ;; \
    "linux-386") \
        SPEEDTEST_ARCH="386"; \
        SPEEDTEST_CHECKSUM="392823ad7e984cb9aebce1922a5609cabdad3290c74a37f6dbf1384576adaa51" ;; \
    "linux-arm") \
        case "${TARGETVARIANT}" in \
            "v7") \
                SPEEDTEST_ARCH="armv7"; \
                SPEEDTEST_CHECKSUM="610aa869eb8db44599960fded5ce4e8833bbf332e3204ea998c2427bb47a271e" ;; \
            "v6") \
                SPEEDTEST_ARCH="armv6"; \
                SPEEDTEST_CHECKSUM="8772c020901e34ab28983cc1cc1b8b1c3244bb1db4453eaeafed6b00833f6291" ;; \
            "v5") \
                SPEEDTEST_ARCH="armv5"; \
                SPEEDTEST_CHECKSUM="d374b3bd9df8ab069c31b822e4f9e98409d26b1a77635dc6033173701854a338" ;; \
            *) \
                SPEEDTEST_ARCH="armv7"; \
                SPEEDTEST_CHECKSUM="610aa869eb8db44599960fded5ce4e8833bbf332e3204ea998c2427bb47a271e" ;; \
        esac ;; \
    *) echo "不支持的平台: ${TARGETOS}-${TARGETARCH}" && exit 1 ;; \
    esac && \
    # 根据是否提供 GitHub 令牌选择不同的下载方式
    if [ -n "${GITHUB_TOKEN}" ]; then \
        curl -fsSL --retry 3 --retry-delay 2 \
            -H "Authorization: Bearer ${GITHUB_TOKEN}" \
            -o /tmp/librespeed-cli.tar.gz \
            "https://github.com/librespeed/speedtest-cli/releases/download/v1.0.12/librespeed-cli_1.0.12_linux_${SPEEDTEST_ARCH}.tar.gz"; \
    else \
        curl -fsSL --retry 3 --retry-delay 2 \
            -o /tmp/librespeed-cli.tar.gz \
            "https://github.com/librespeed/speedtest-cli/releases/download/v1.0.12/librespeed-cli_1.0.12_linux_${SPEEDTEST_ARCH}.tar.gz"; \
    fi && \
    # 验证文件完整性
    echo "${SPEEDTEST_CHECKSUM}  /tmp/librespeed-cli.tar.gz" | sha256sum -c - && \
    # 解压并安装
    tar -xzf /tmp/librespeed-cli.tar.gz -C /usr/local/bin/ && \
    chmod +x /usr/local/bin/librespeed-cli && \
    # 清理临时文件
    rm /tmp/librespeed-cli.tar.gz

# 构建 Go 应用程序（使用平台特定设置）
RUN --network=none --mount=target=. \
    export GOOS=$TARGETOS; \
    export GOARCH=$TARGETARCH; \
    [[ "$GOARCH" == "amd64" ]] && export GOAMD64=$TARGETVARIANT; \
    [[ "$GOARCH" == "arm" ]] && [[ "$TARGETVARIANT" == "v6" ]] && export GOARM=6; \
    [[ "$GOARCH" == "arm" ]] && [[ "$TARGETVARIANT" == "v7" ]] && export GOARM=7; \
    echo "正在构建: $GOARCH $GOOS $GOARM$GOAMD64"; \
    # 构建应用程序并设置链接参数
    go build -ldflags "-s -w \
    -X 'main.version=${VERSION}' \
    -X 'main.commit=${REVISION}' \
    -X 'main.buildTime=${BUILDTIME}'" \
    -o /app/netronome ./cmd/netronome

# 构建最终运行镜像
FROM alpine:3.22

LABEL org.opencontainers.image.source="https://github.com/autobrr/netronome"
LABEL org.opencontainers.image.licenses="GPL-2.0-or-later"
LABEL org.opencontainers.image.base.name="alpine:3.22"

# Install dependencies including tini for proper process reaping
RUN apk add --no-cache tini sqlite iperf3 traceroute mtr tzdata vnstat

# 设置环境变量（数据持久化目录）
ENV HOME="/data" \
    XDG_CONFIG_HOME="/data" \
    XDG_DATA_HOME="/data"

# 设置工作目录
WORKDIR /data

# 从应用构建阶段复制二进制文件
COPY --from=app-builder /app/netronome /usr/local/bin/netronome
COPY --from=app-builder /usr/local/bin/librespeed-cli /usr/local/bin/librespeed-cli

# 暴露应用端口
EXPOSE 7575

# 创建用户和组，设置权限
RUN addgroup -S netronome && \
    adduser -S netronome -G netronome && \
    mkdir -p /data && \
    chown -R netronome:netronome /data && \
    chmod 755 /data

# 切换到非 root 用户（提高安全性）
USER netronome

# 使用 tini 作为 PID 1 进程，处理信号和僵尸进程
ENTRYPOINT ["/sbin/tini", "--", "netronome"]
# 默认命令：启动服务
CMD ["serve"]