[![Go Reference](https://pkg.go.dev/badge/github.com/kercylan98/vivid-nexus.svg)](https://pkg.go.dev/github.com/kercylan98/vivid-nexus)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Go Report Card](https://goreportcard.com/badge/github.com/kercylan98/vivid-nexus)](https://goreportcard.com/report/github.com/kercylan98/vivid-nexus)
[![GitHub stars](https://img.shields.io/github/stars/kercylan98/vivid-nexus.svg?style=social&label=Stars)](https://github.com/kercylan98/vivid-nexus)

**Vivid-Nexus** 是基于 [vivid](https://github.com/kercylan98/vivid) Actor 模型的会话管理层：将每个连接抽象为 `Session`，由 Nexus 统一托管并驱动读写与生命周期，业务通过 `SessionActor` 实现连接建立、断开与收包回调，并可通过同一实例进行 `Close`、`Send`、`Broadcast` 等操作。

> **依赖**：需配合 [vivid](https://github.com/kercylan98/vivid) 使用；API 可能随版本调整，请以 [Releases](https://github.com/kercylan98/vivid-nexus/releases) 与文档为准。

## 目录

- [特性](#特性)
- [安装与依赖](#安装与依赖)
- [快速开始](#快速开始)
- [架构概览](#架构概览)
- [核心接口](#核心接口)
- [配置](#配置)
- [许可证与资源](#许可证与资源)

## 特性

- **会话即 Actor**：每个 `Session` 对应一个 sessionActor，生命周期与消息由 vivid 统一调度
- **业务侧回调**：实现 `SessionActor`（OnConnected / OnDisconnected / OnMessage）即可处理连接与收包
- **读路径背压**：读循环与业务处理通过 channel 同步，读一条、处理完、再读下一条，避免邮箱堆积
- **统一操作入口**：同一 Nexus 实例既可作为 Actor 接收 `Session` 托管，也可调用 `Close(sessionId)`、`Send(sessionId, message)`、`Broadcast(message)`，并发安全
- **可替换读策略**：通过 `SessionReaderProvider` 定制按协议/会话的读取逻辑，默认提供按字节流读取实现
- **同 ID 替换**：相同 `GetSessionId()` 的新连接会替换旧会话并关闭旧连接

## 安装与依赖

- **Go**：1.26+
- **安装**：

```bash
go get github.com/kercylan98/vivid-nexus
```

依赖 [github.com/kercylan98/vivid](https://github.com/kercylan98/vivid)，使用前请确保已引入 vivid 并完成其系统初始化。

## 架构概览

```mermaid
graph TB
    subgraph 接入层
        Conn[连接 TCP/WS/...]
    end
    subgraph Nexus
        Actor[actor 实现 vivid.Actor]
        Op[operator: Close/Send/Broadcast]
        Map[sessions map]
        Actor --> Map
        Actor --> Op
    end
    subgraph 每个 Session
        SA[sessionActor]
        Info[sessionInfo]
        RL[readLoop]
        SA --> Info
        SA --> RL
    end
    Conn -->|实现 Session| Session[Session]
    Actor -->|Tell Session| Session
    Session -->|托管| SA
    Map -->|sessionId -> sessionInfo| Info
    RL -->|TellSelf []byte| SA
    SA -->|OnMessage ctx, message| Biz[SessionActor 业务]
```

- **Nexus**：一个 actor + operator。作为 Actor 接收 `Session` 并维护 `sessions` 映射；作为 operator 对外提供 `Close`、`Send`、`Broadcast`，内部用 `sessionLock` 保证并发安全。
- **sessionActor**：每个托管连接一个，负责 Prelaunch（拉取 SessionActor、SessionReader）、OnLaunch（OnConnected、启动 readLoop）、OnKill（OnDisconnected、关闭 Session 与 messageC）。
- **readLoop**：独立 goroutine 中循环 `SessionReader.Read()`，将读到的 `[]byte` 投递到本 Actor 邮箱并等待处理完成再读下一条（背压）。

## 核心接口

| 接口 | 说明 |
|------|------|
| **Session** | 底层连接抽象：`io.ReadWriteCloser` + `GetSessionId() string`，由接入层实现 |
| **SessionContext** | 业务在回调中拿到的上下文：`vivid.ActorContext` + `GetSessionId` + `Close()` + `Send([]byte) error` |
| **SessionActor** | 业务实现的会话逻辑：`OnConnected` / `OnDisconnected` / `OnMessage`，所有回调在单线程中串行执行 |
| **SessionActorProvider** | 为每个新会话提供 `SessionActor` 实例；可用 `SessionActorProviderFN` 函数适配 |
| **SessionReader** | 会话数据读取器，`Read() (n, data, err)`；返回的 `data` 仅在本轮返回到下次 Read 前有效，跨周期使用须拷贝 |
| **SessionReaderProvider** | 为指定 Session 提供 `SessionReader`；可用 `SessionReaderProviderFN` 函数适配，未设置时使用默认按字节流读取 |

详细约定与 []byte 所有权见 [pkg.go.dev](https://pkg.go.dev/github.com/kercylan98/vivid-nexus) 或源码注释。

## 配置

通过 `New(provider, options...)` 传入可选配置：

- **NewOptions(opts...)**：构造 Options，默认带 `SessionReaderProvider` 为按字节流读取实现。
- **WithSessionReaderProvider(provider)**：自定义读取器提供方。
- **WithOptions(options)**：从已有 Options 克隆；若源 `SessionReaderProvider` 为 nil 会补默认实现。

示例：

```go
opts := nexus.NewOptions(
    nexus.WithSessionReaderProvider(myReaderProvider),
)
n, err := nexus.New(provider, opts)
```

## 许可证与资源

- **许可证**：[MIT](LICENSE)
- **依赖**：[vivid](https://github.com/kercylan98/vivid) · [API 文档](https://pkg.go.dev/github.com/kercylan98/vivid-nexus)
- **贡献**：欢迎 Issue 与 PR，请保持测试通过（`go test ./...`）与代码风格一致。
