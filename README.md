[![Go Reference](https://pkg.go.dev/badge/github.com/kercylan98/vivid-nexus.svg)](https://pkg.go.dev/github.com/kercylan98/vivid-nexus)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Go Report Card](https://goreportcard.com/badge/github.com/kercylan98/vivid-nexus)](https://goreportcard.com/report/github.com/kercylan98/vivid-nexus)
[![GitHub stars](https://img.shields.io/github/stars/kercylan98/vivid-nexus.svg?style=social&label=Stars)](https://github.com/kercylan98/vivid-nexus)

**Vivid-Nexus** 是基于 [vivid](https://github.com/kercylan98/vivid) Actor 模型的会话管理层：将每个连接抽象为 `Session`，由 Nexus 统一托管并驱动读写与生命周期。业务实现 `SessionActor` 的 `OnConnected` / `OnDisconnected` / `OnMessage`，通过同一 Nexus 实例调用 `Actor()` 注册到 vivid、`TakeoverSession(session)` 接管连接，以及 `Close`、`Send`、`SendTo`、`Broadcast` 等操作。

> **依赖**：需配合 [vivid](https://github.com/kercylan98/vivid) 使用。API 可能随版本调整，请以 [Releases](https://github.com/kercylan98/vivid-nexus/releases) 与文档为准。

## 目录

- [特性](#特性)
- [安装与依赖](#安装与依赖)
- [快速开始](#快速开始)
- [文档与资源](#文档与资源)

## 特性

- **会话即 Actor**：每个 `Session` 对应一个内部 sessionActor，生命周期与消息由 vivid 统一调度。
- **业务侧回调**：实现 `SessionActor`（OnConnected / OnDisconnected / OnMessage）即可处理连接与收包，无需实现 vivid.Actor。
- **读路径背压**：读循环与业务处理通过 channel 同步，读一条、处理完、再读下一条，避免邮箱堆积。
- **统一操作入口**：Nexus 接口提供 `Actor()`、`TakeoverSession`、`Close`、`Send`、`SendTo`、`Broadcast`，并发安全。
- **可替换读策略**：通过 `SessionReaderProvider` 定制按协议/会话的读取逻辑，默认提供按字节流读取。
- **同 ID 替换**：相同 `GetSessionId()` 的新连接会替换旧会话并关闭旧连接。

## 安装与依赖

- **Go**：1.26+
- **安装**：`go get github.com/kercylan98/vivid-nexus`
- 依赖 [github.com/kercylan98/vivid](https://github.com/kercylan98/vivid)，使用前请确保已引入 vivid 并完成其系统初始化。

## 快速开始

```go
// 创建 Nexus（New 返回 Nexus 接口）
n, _ := nexus.New(nexus.SessionActorProviderFN(func() (nexus.SessionActor, error) {
    return &MySessionActor{}, nil
}))

// 将 Nexus 的 Actor 注册到 vivid
system.ActorOf(n.Actor())

// 接入层拿到连接后交给 Nexus 托管
n.TakeoverSession(mySession)
```

更多用法与接口说明见 [vivid 文档](https://github.com/kercylan98/vivid/tree/main/docs) 与 [pkg.go.dev](https://pkg.go.dev/github.com/kercylan98/vivid-nexus)。

## 文档与资源

- **详细文档**：[vivid/docs](https://github.com/kercylan98/vivid/tree/main/docs)
- **API 文档**：[pkg.go.dev/github.com/kercylan98/vivid-nexus](https://pkg.go.dev/github.com/kercylan98/vivid-nexus)
- **许可证**：[MIT](LICENSE)
- **贡献**：欢迎 Issue 与 PR，请保持测试通过（`go test ./...`）与代码风格一致。
