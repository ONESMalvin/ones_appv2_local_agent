# ONES AppV2 Local Agent 使用文档

## 项目简介

ONES AppV2 Local Agent 是一个基于 WebSocket 的本地代理工具，用于将本地开发中的应用服务与ONES打通，不依赖外网环境直接与ONES点对点通信。

**注意**: 这是一个开发工具，请勿在生产环境中使用。

## 安装方法

```bash
# 克隆项目
go install github.com/ONESMalvin/ones_appv2_local_agent@latest
```


## 使用方法

### 基本语法

```bash
./ones_appv2_local_agent [选项]
```

### 必需参数

| 参数 | 短参数 | 说明 | 示例值 |
|------|--------|------|--------|
| `--server` | `-s` | 中继服务器地址 | `p8205-k3s-9.k3s-dev.myones.net` |
| `--app` | `-a` | 应用 ID | `app_F63GRnbJR6xINLyK` |
| `--token` | `-t` | 认证令牌 | `testmyrelaytoken` |
| `--port` | `-p` | 本地服务端口 | `8082` |

### 使用示例

#### 基本使用

```bash
./ones_appv2_local_agent -s p8205-k3s-9.k3s-dev.myones.net \
              -a app_F63GRnbJR6xINLyK \
              -t testmyrelaytoken \
              -p 8082
```

#### 使用长参数

```bash
./ones_appv2_local_agent --server p8205-k3s-9.k3s-dev.myones.net \
              --app app_F63GRnbJR6xINLyK \
              --token testmyrelaytoken \
              --port 8082
```

## 工作原理

1. **建立连接**: Local Agent 通过 WebSocket 连接到指定的中继服务器
2. **认证**: 使用提供的 Token 进行身份验证
3. **请求转发**: 接收来自中继服务器的 HTTP 请求，转发到本地服务
4. **响应返回**: 将本地服务的响应通过 WebSocket 返回给中继服务器
5. **自动重连**: 连接断开时自动重连，确保服务可用性

### 日志说明

- `[agent] dialing {url}`: 正在连接ONES中继服务器
- `[agent] dialed {url}`: 成功连接到ONES中继服务器
- `[agent] endpoint {url}`: ONES可以访问的应用地址，可以用这个地址作为应用的baseUrl。

## 故障排除

### 常见问题

#### 1. 连接失败

**症状**: 显示 "dial failed" 错误

**解决方案**:
- 检查网络连接
- 验证服务器地址是否正确
- 确认防火墙设置

#### 2. 认证失败

**症状**: 连接被拒绝或返回 401 错误

**解决方案**:
- 检查 Token 是否正确
- 确认 Token 是否过期
- 验证应用 ID 是否有效

#### 3. 本地服务无法访问

**症状**: 返回 502 Bad Gateway 错误

**解决方案**:
- 确认本地服务是否正在运行
- 检查端口号是否正确
- 验证本地服务是否监听在 127.0.0.1 上



