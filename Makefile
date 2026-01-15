# 构建变量定义
BINARY_NAME=netronome  # 生成的二进制文件名
BUILD_DIR=bin          # 构建输出目录
DOCKER_IMAGE=netronome # Docker 镜像名称

# 声明伪目标，防止与同名文件冲突
.PHONY: all build clean run docker-build docker-run watch dev dev-expose

# 默认目标：执行构建
all: build

# 构建目标：编译前端和后端
build: 
	@echo "正在构建前端和后端..."
	@mkdir -p $(BUILD_DIR)           # 创建构建输出目录
	@mkdir -p web/dist               # 创建前端输出目录
	@cd web && pnpm install && pnpm build  # 安装前端依赖并构建前端
	@touch web/dist/.gitkeep         # 创建占位文件，确保目录被 Git 跟踪
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/netronome  # 构建后端二进制文件

# 清理目标：删除构建产物和依赖
clean:
	@echo "正在清理..."
	@rm -rf $(BUILD_DIR)             # 删除构建输出目录
	@rm -rf web/dist                 # 删除前端构建产物
	@mkdir -p web/dist               # 重新创建前端输出目录（空）
	@touch web/dist/.gitkeep         # 创建占位文件
	@rm -rf web/node_modules         # 删除前端依赖

# 运行目标：先构建，然后运行应用
run: build
	@echo "正在运行应用..."
	@./$(BUILD_DIR)/$(BINARY_NAME) serve --config config.toml  # 使用 config.toml 配置文件启动服务

# Docker 构建目标：构建 Docker 镜像
docker-build:
	@echo "正在构建 Docker 镜像..."
	docker build -t $(DOCKER_IMAGE) .  # 从当前目录构建 Docker 镜像

# Docker 运行目标：先构建镜像，然后运行容器
docker-run: docker-build
	@echo "正在运行 Docker 容器..."
	docker run -p 7575:7575 $(DOCKER_IMAGE)  # 运行容器并映射 7575 端口

# 开发模式目标：带热重载功能的开发环境
dev:
	@echo "正在启动开发服务器..."
	# 使用 tmux 创建新会话，启动前端开发服务器
	@GIN_MODE=debug tmux new-session -d -s dev 'cd web && pnpm dev'
	@touch web/dist/.gitkeep > /dev/null 2>&1  # 创建占位文件，避免构建错误
	# 在 tmux 中水平分割窗口，启动后端监控
	@GIN_MODE=debug tmux split-window -h 'make watch'
	@tmux -2 attach-session -d  # 连接到 tmux 会话，进入开发环境

# 网络开发模式目标：带热重载且暴露在网络上的开发环境
dev-expose:
	@echo "正在启动开发服务器（暴露在网络上）..."
	@echo "前端将在 http://0.0.0.0:5173 可用"
	@echo "后端将在 http://0.0.0.0:7575 可用"
	@echo "您可以使用机器的 IP 地址从其他设备访问"
	# 启动前端开发服务器，监听所有网络接口
	@GIN_MODE=debug tmux new-session -d -s dev 'cd web && pnpm dev --host 0.0.0.0'
	@touch web/dist/.gitkeep > /dev/null 2>&1  # 创建占位文件
	# 在 tmux 中水平分割窗口，启动后端监控
	@GIN_MODE=debug tmux split-window -h 'make watch'
	@tmux -2 attach-session -d  # 连接到 tmux 会话

# 监控目标：使用 air 工具实现后端代码热重载
watch:
	# 检查 air 工具是否已安装
	@if command -v air > /dev/null; then \
		GIN_MODE=debug air -- serve --config config.toml; \
		echo "正在监控代码变化...";\
	else \
		# 提示用户安装 air 工具
		read -p "您的机器上未安装 Go 的 'air' 工具。是否要安装？[Y/n] " choice; \
		if [ "$$choice" != "n" ] && [ "$$choice" != "N" ]; then \
			go install github.com/cosmtrek/air@latest; \
			GIN_MODE=debug air -- serve --config config.toml; \
			echo "正在监控代码变化...";\
		else \
			echo "您选择不安装 air。退出..."; \
			exit 1; \
		fi; \
	fi