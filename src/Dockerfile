# --- Stage 1: Build ---
# 使用官方的Go镜像作为构建环境
FROM golang:alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制go.mod和go.sum文件，并下载依赖
# 这一步可以被Docker缓存，加快后续构建速度
COPY go.mod go.sum ./
RUN go mod download

# 复制所有源代码
COPY . .

# 编译Go程序。
# CGO_ENABLED=0: 禁用CGO，构建一个纯Go的静态二进制文件。
# -o /sre-agent: 将编译好的程序输出到根目录下的sre-agent文件。
RUN CGO_ENABLED=0 GOOS=linux go build -o /sre-agent .

# --- Stage 2: Final Image ---
# 使用一个极小的基础镜像alpine
FROM alpine:latest

# 从构建阶段(builder)复制编译好的二进制文件到当前镜像
COPY --from=builder /sre-agent /sre-agent

# 设置容器的入口点。当容器启动时，它会执行/sre-agent这个程序。
ENTRYPOINT ["/sre-agent"]
