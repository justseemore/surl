# 多阶段构建 - 构建阶段
FROM golang:1.24-bullseye AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的系统依赖
RUN apt-get update && apt-get install -y \
    gcc \
    libc6-dev \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

# 设置环境变量
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN go build -a -installsuffix cgo -ldflags='-w -s' -o main .

# 运行阶段
FROM debian:bullseye-slim

# 安装运行时依赖
RUN apt-get update && apt-get install -y \
    ca-certificates \
    sqlite3 \
    && rm -rf /var/lib/apt/lists/*


# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/main .

# 复制模板文件
COPY  templates ./templates

# 复制配置文件
COPY .env.example .env

# 创建数据目录
RUN mkdir -p /app/data && chown appuser:appgroup /app/data

# 暴露端口
EXPOSE 8080

# 启动应用
CMD ["./main"]