# Build go
FROM golang:1.26.2-alpine AS builder
WORKDIR /app
COPY . .
ENV CGO_ENABLED=0
RUN GOEXPERIMENT=jsonv2 go mod download
RUN GOEXPERIMENT=jsonv2 go build -v -o v2node

# Release
FROM alpine:3.20
# 安装必要的工具包，创建非 root 用户
RUN apk --update --no-cache add tzdata ca-certificates \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && adduser -D -H -s /sbin/nologin v2node \
    && mkdir -p /etc/v2node/ \
    && chown v2node:v2node /etc/v2node/
COPY --from=builder /app/v2node /usr/local/bin

USER v2node
ENTRYPOINT [ "v2node", "server", "--config", "/etc/v2node/config.json"]
