# v2node

A high-performance v2board backend based on modified xray-core.基于改进的xray-core的高性能v2board后端。

[![Build](https://img.shields.io/github/actions/workflow/status/Mcloud136/v2node/release.yml?label=Build&logo=github&logoColor=white&color=28a745&style=for-the-badge)](https://github.com/Mcloud136/v2node/actions/workflows/release.yml)
[![Version](https://img.shields.io/github/v/release/Mcloud136/v2node?label=Version&logo=semver&logoColor=white&color=0366d6&style=for-the-badge)](https://github.com/Mcloud136/v2node/releases/)
[![Released](https://img.shields.io/github/release-date/Mcloud136/v2node?label=Released&logo=calendar&logoColor=white&color=6f42c1&style=for-the-badge)](https://github.com/Mcloud136/v2node/releases/)
[![Commits](https://img.shields.io/github/commit-activity/m/Mcloud136/v2node?label=Commits&logo=git&logoColor=white&color=f1e05a&style=for-the-badge)](https://github.com/Mcloud136/v2node/)


## 项目简介

本项目基于[xiao佬的v2node](https://github.com/wyx2685/v2node)制作，提升性能并加入一些新的特性。

**注意**：本项目和[xiao佬的v2node](https://github.com/wyx2685/v2node)一样，需要搭配[修改版 V2board](https://github.com/wyx2685/v2board) 使用。

## 原版特性

- 支持多种协议：VLESS、VMESS、Trojan、Shadowsocks、Hysteria2、TUIC、AnyTLS
- 高性能并发设计，支持高负载场景
- 完善的流量限制和设备管理
- 自动证书管理和更新
- 配置热更新，无需重启服务
- 优雅的错误处理和日志管理

## 改进内容

### 性能优化

- 使用 `atomic.Pointer` 实现无锁读取，提升并发性能
- `sync.Map` 替代普通 map，支持高效并发读写
- 复合 key 设计简化数据结构，减少内存开销
- `determineSpeedLimit` 逻辑优化，代码减少 60%

### 类型安全

- 使用类型断言替代反射，提高代码安全性和执行效率
- 添加 `int64` 类型支持，增强兼容性

### 配置管理

- 新增配置验证机制，启动前校验参数有效性
- 端口可用性检查，避免启动失败
- 配置热更新失败时保持原配置运行，确保服务连续性
- 支持增量配置更新

### 资源管理

- 统一资源生命周期管理
- pprof 服务支持优雅关闭
- 日志文件句柄统一管理，避免资源泄漏

---

## 系统要求

### 支持的操作系统

| 系统 | 最低版本 | 说明 |
|------|----------|------|
| CentOS | 7 | CentOS 7 无法使用 hysteria1/2 协议 |
| Ubuntu | 16 | 推荐 18.04+ |
| Debian | 8 | 推荐 10+ |
| Alpine | 3.10+ | 轻量级发行版 |

### 硬件要求

| 配置 | CPU | 内存 | 带宽 |
|------|-----|------|------|
| 最小配置 | 1核 | 512MB | 100Mbps |
| 推荐配置 | 2核 | 1GB | 1Gbps |

### 架构支持

- x86_64 (amd64) - 推荐(其余版本请自行构建）
- ARM64 (aarch64)
- s390x

---

## 安装方法

### 方法一：一键安装

```bash
wget -N https://raw.githubusercontent.com/Mcloud136/v2node/master/script/install.sh && bash install.sh
```

**带参数安装**（跳过交互）：

```bash
wget -N https://raw.githubusercontent.com/Mcloud136/v2node/master/script/install.sh && \
bash install.sh --api-host https://your-panel.com/ --node-id 1 --api-key your-secret-key
```

### 方法二：手动安装

#### 1. 创建目录结构

```bash
mkdir -p /usr/local/v2node
mkdir -p /etc/v2node
```

#### 2. 下载并解压二进制文件

```bash
cd /usr/local/v2node
curl -sL "https://github.com/Mcloud136/v2node/releases/latest/download/v2node-linux-amd64.zip" | unzip -
chmod +x v2node
```

#### 3. 创建 systemd 服务

```bash
cat > /etc/systemd/system/v2node.service <<EOF
[Unit]
Description=v2node Service
After=network.target nss-lookup.target
Wants=network.target

[Service]
User=root
Group=root
Type=simple
LimitAS=infinity
LimitRSS=infinity
LimitCORE=infinity
LimitNOFILE=999999
WorkingDirectory=/usr/local/v2node/
ExecStart=/usr/local/v2node/v2node server
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable v2node
```

#### 4. 配置管理脚本

```bash
curl -o /usr/bin/v2node -Ls https://raw.githubusercontent.com/Mcloud136/v2node/main/script/v2node.sh
chmod +x /usr/bin/v2node
```

---

## config.json 配置详解

### 配置文件位置

```
/etc/v2node/config.json
```

### 完整配置示例

```json
{
    "Log": {
        "Level": "info",
        "Output": "/var/log/v2node.log",
        "Access": "/var/log/v2node_access.log"
    },
    "Nodes": [
        {
            "ApiHost": "https://your-panel.com/",
            "NodeID": 1,
            "ApiKey": "your-secret-api-key",
            "Timeout": 15
        }
    ],
    "PprofPort": 6060
}
```

### 配置参数说明

#### 1. Log 配置

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `Level` | string | 否 | `info` | 日志级别：`debug`、`info`、`warning`、`error` |
| `Output` | string | 否 | `""` | 日志输出文件路径，为空则输出到控制台 |
| `Access` | string | 否 | `"none"` | 访问日志路径，设置为 `"none"` 禁用访问日志 |

#### 2. Nodes 配置（支持多节点）

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `ApiHost` | string | **是** | - | V2board 面板 API 地址，必须以 `/` 结尾 |
| `NodeID` | int | **是** | - | 节点 ID，在面板中创建节点时获得 |
| `ApiKey` | string | **是** | - | 节点通讯密钥，在面板中设置 |
| `Timeout` | int | 否 | `15` | API 请求超时时间（秒） |
| `RetryCount` | int | 否 | `1` | 失败重试次数 |

#### 3. PprofPort 配置

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `PprofPort` | int | 否 | `0` | pprof 调试端口，设置为 `0` 禁用 |

### 多节点配置示例

```json
{
    "Log": {
        "Level": "warning",
        "Output": "",
        "Access": "none"
    },
    "Nodes": [
        {
            "ApiHost": "https://panel1.example.com/",
            "NodeID": 1,
            "ApiKey": "key1",
            "Timeout": 15
        },
        {
            "ApiHost": "https://panel2.example.com/",
            "NodeID": 2,
            "ApiKey": "key2",
            "Timeout": 20
        }
    ]
}
```

---

## 服务管理

### 使用管理脚本

```bash
v2node              # 显示管理菜单
v2node start        # 启动服务
v2node stop         # 停止服务
v2node restart      # 重启服务
v2node status       # 查看状态
v2node enable       # 设置开机自启
v2node disable      # 取消开机自启
v2node log          # 查看日志
v2node generate     # 生成配置文件
v2node update       # 更新到最新版本
v2node update 1.0.0 # 更新到指定版本
v2node version      # 查看版本
v2node uninstall    # 卸载
```

### 使用 systemd 命令

```bash
systemctl start v2node        # 启动
systemctl stop v2node         # 停止
systemctl restart v2node      # 重启
systemctl status v2node       # 查看状态
journalctl -u v2node -f       # 查看日志
systemctl enable v2node       # 设置开机自启
systemctl disable v2node      # 取消开机自启
```

---

## 命令行参数

### 启动命令

```bash
v2node server                   # 使用默认配置文件
v2node server -c /path/to/config.json  # 指定配置文件
v2node server --help            # 显示帮助
```

### 版本信息

```bash
v2node version
```

---

## 支持的协议

| 协议 | 状态 | 说明 |
|------|------|------|
| VLESS | ✅ 支持 | 推荐使用 |
| VMESS | ✅ 支持 | 经典协议 |
| Trojan | ✅ 支持 | 伪装能力强 |
| Shadowsocks | ✅ 支持 | 传统协议 |
| Hysteria2 | ✅ 支持 | 高性能UDP协议 |
| TUIC | ✅ 支持 | QUIC协议 |
| AnyTLS | ✅ 支持 | TLS伪装 |

---

## 常见问题

### Q1: 启动失败，提示 "Exec format error"

**原因**：二进制文件与系统架构不匹配

**解决方案**：
```bash
uname -m                          # 查看系统架构
# x86_64: 下载 v2node-linux-amd64.zip
# aarch64: 下载 v2node-linux-arm64-v8a.zip
```

### Q2: 无法连接到面板 API

**解决方案**：
1. 检查 `ApiHost` 是否正确，必须以 `http://或者https://` 开头
2. 检查服务器网络是否能访问面板地址
3. 检查防火墙是否放行出站流量

### Q3: 日志显示证书错误

**解决方案**：
```bash
# Debian/Ubuntu
apt-get update && apt-get install ca-certificates

# CentOS
yum install ca-certificates
update-ca-trust
```

### Q4: 高 CPU 占用

**解决方案**：
1. 降低日志级别为 `warning` 或 `error`
2. 检查是否有大量连接请求
3. 考虑升级服务器配置

### Q5: 如何配置 HTTPS

HTTPS 配置在 V2board 面板中完成，节点会自动获取证书配置。

---

## 日志管理

### 日志位置

- 默认日志：控制台输出（可通过 `Log.Output` 配置到文件）
- Systemd 日志：`journalctl -u v2node`

### 日志级别说明

| 级别 | 说明 |
|------|------|
| `debug` | 详细调试信息，适合开发环境 |
| `info` | 一般信息，适合生产环境 |
| `warning` | 警告信息，推荐生产环境使用 |
| `error` | 仅错误信息，最小日志量 |

---

## 安全建议

1. **防火墙配置**：只开放必要端口
2. **定期更新**：保持软件版本最新
3. **密钥管理**：妥善保管并定期更新 `ApiKey`，避免泄露
4. **日志监控**：定期检查日志，发现异常及时处理
5. **权限控制**：建议使用非 root 用户运行

---

## 更新日志

### v1.0.2
- 修复 README.md 文档格式问题（操作系统支持表格、代码块标记等）
- 优化代码格式，运行 `go fmt` 规范代码风格
- 运行 `go vet` 检查代码质量，确保无潜在问题

### v1.0.1
- 优化 Limiter 模块，使用 sync.Map 替代 map，消除性能瓶颈
- 用户管理 Context 复用，减少内存分配和 GC 压力
- 修复流量计数器超时参数传递问题
- 更新 Go 版本为 1.26.2，修复构建兼容性
- 添加 cloudflared 服务联动重启

### v1.0.0
- 优化并发性能和配置验证
- 支持配置热更新
- 完善错误处理机制

---

## 构建

```bash
GOEXPERIMENT=jsonv2 go build -v -o build_assets/v2node -trimpath -ldflags "-X 'github.com/Mcloud136/v2node/cmd.version=1.0.2' -s -w -buildid="
```

---

## 许可证

GPL V3 License

---

## 贡献

欢迎提交 Issue 和 Pull Request！

---

## 相关项目

- [V2board](https://github.com/wyx2685/v2board)
- [Xray-core](https://github.com/XTLS/Xray-core)

---

## 联系支持

- 项目地址：https://github.com/Mcloud136/v2node
- 面板项目：https://github.com/wyx2685/v2board
