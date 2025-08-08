# 多阶段构建 - 构建阶段
FROM golang:1.24-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的系统依赖
RUN apk add --no-cache git gcc musl-dev sqlite-dev

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main .

# 运行阶段
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates sqlite

# 创建应用用户
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/main .

# 复制模板文件
COPY --chown=appuser:appgroup templates ./templates

# 复制配置文件（可选，如果不通过环境变量提供）
COPY --chown=appuser:appgroup .env.example .env

# 创建数据目录
RUN mkdir -p /app/data && chown appuser:appgroup /app/data

# 切换到非root用户
USER appuser

# 启动应用
CMD ["./main"]