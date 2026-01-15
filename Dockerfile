# 使用 Node.js LTS 版本作为前端构建基础镜像
FROM node:lts-alpine3.22 AS web-builder

# 全局安装 pnpm 包管理器（指定版本 9.9.0）
RUN npm install -g pnpm@9.9.0

# 设置前端构建工作目录
WORKDIR /app/web

# 复制前端的依赖配置文件
COPY web/package.json web/pnpm-lock.yaml ./
# 使用锁定的依赖版本进行安装，确保构建一致性
RUN pnpm install --frozen-lockfile

# 复制所有前端源代码
COPY web/ ./
# 构建前端应用，生成静态文件
RUN pnpm run build

# 使用 Go 1.24 作为后端构建基础镜像，支持多平台构建
FROM --platform=$BUILDPLATFORM golang:1.24-alpine3.22 AS app-builder

# 定义构建参数
# 版本信息（默认开发版）
ARG VERSION=dev     
# Git 提交哈希（默认开发版）   
ARG REVISION=dev   
# 构建时间    
ARG BUILDTIME       
# 目标操作系统   
ARG TARGETOS        
# 目标架构   
ARG TARGETARCH         
# 目标架构变体（如 armv7, armv6 等）
ARG TARGETVARIANT      

# 安装系统依赖包
RUN apk add --no-cache git build-base tzdata curl ca-certificates

# 设置环境变量
# 服务名称
ENV SERVICE=netronome  
# 禁用 CGO，生成静态链接的二进制文件
ENV CGO_ENABLED=0      

# 设置后端构建工作目录
WORKDIR /src

# 复制 Go 模块依赖文件
COPY go.mod go.sum ./
# 下载依赖包
RUN go mod download

# 复制所有后端源代码
COPY . ./
# 从前端构建阶段复制构建好的静态文件
COPY --from=web-builder /app/web/dist ./web/dist

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
    curl -fsSL --retry 3 --retry-delay 2 \
        -o /tmp/librespeed-cli.tar.gz \
        "https://github.com/librespeed/speedtest-cli/releases/download/v1.0.12/librespeed-cli_1.0.12_linux_${SPEEDTEST_ARCH}.tar.gz" && \
    echo "${SPEEDTEST_CHECKSUM}  /tmp/librespeed-cli.tar.gz" | sha256sum -c - && \
    tar -xzf /tmp/librespeed-cli.tar.gz -C /usr/local/bin/ && \
    chmod +x /usr/local/bin/librespeed-cli && \
    rm /tmp/librespeed-cli.tar.gz

# 构建 Go 应用程序
RUN export GOOS=$TARGETOS; \
    export GOARCH=$TARGETARCH; \
    [[ "$GOARCH" == "amd64" ]] && export GOAMD64=$TARGETVARIANT; \
    [[ "$GOARCH" == "arm" ]] && [[ "$TARGETVARIANT" == "v6" ]] && export GOARM=6; \
    [[ "$GOARCH" == "arm" ]] && [[ "$TARGETVARIANT" == "v7" ]] && export GOARM=7; \
    echo "正在构建: $GOARCH $GOOS $GOARM$GOAMD64"; \
    go build -ldflags "-s -w \
    -X 'main.version=${VERSION}' \
    -X 'main.commit=${REVISION}' \
    -X 'main.buildTime=${BUILDTIME}'" \
    -o /app/netronome ./cmd/netronome

# 构建最终运行镜像
FROM alpine:3.22

# 安装运行时依赖包，包括 tini 用于正确处理僵尸进程
RUN apk add --no-cache tini sqlite iperf3 traceroute mtr tzdata vnstat

# 设置环境变量
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

# 切换到非 root 用户
USER netronome

# 使用 tini 作为 PID 1 进程，处理信号和僵尸进程
ENTRYPOINT ["/sbin/tini", "--", "netronome"]
# 默认命令：启动服务
CMD ["serve"]