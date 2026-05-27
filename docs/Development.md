# 开发指南

## 环境要求

- Go 1.26.3+
- Git

## 获取源码

```bash
git clone https://github.com/Mcloud136/v2node.git
cd v2node
```

## 构建项目

```bash
GOEXPERIMENT=jsonv2 go build -v -o build_assets/v2node
```

## 运行测试

```bash
go test ./...
```

## 代码规范

### Go 编码规范

- 遵循 [Effective Go](https://go.dev/doc/effective_go)
- 使用 `go fmt` 格式化代码
- 使用 `go vet` 检查代码

### 提交规范

```
<type>: <description>

<optional body>

<optional footer>
```

**类型**：
- feat: 新功能
- fix: Bug 修复
- docs: 文档更新
- style: 代码格式
- refactor: 重构
- test: 测试

## 贡献流程

1. Fork 仓库
2. 创建特性分支
3. 提交更改
4. 发起 Pull Request

## 架构概述

```
v2node/
├── cmd/          # 命令行接口
├── conf/         # 配置管理
├── core/         # 核心逻辑
├── api/          # API 客户端
├── limiter/      # 流量限制
├── node/         # 节点管理
└── task/         # 定时任务
```