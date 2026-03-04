[![Go Reference](https://pkg.go.dev/badge/github.com/kercylan98/vivid-nexus.svg)](https://pkg.go.dev/github.com/kercylan98/vivid-nexus)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Go Report Card](https://goreportcard.com/badge/github.com/kercylan98/vivid-nexus)](https://goreportcard.com/report/github.com/kercylan98/vivid-nexus)
[![GitHub stars](https://img.shields.io/github/stars/kercylan98/vivid-nexus.svg?style=social&label=Stars)](https://github.com/kercylan98/vivid-nexus)

# Vivid-Nexus

基于 [vivid](https://github.com/kercylan98/vivid) 的会话管理层，将网络连接抽象为会话并由统一入口管理生命周期与消息收发。

需配合 [vivid](https://github.com/kercylan98/vivid) 使用，API 以 [Releases](https://github.com/kercylan98/vivid-nexus/releases) 与文档为准。

## 安装

```bash
go get github.com/kercylan98/vivid-nexus
```

Go 1.26+，并已引入 [vivid](https://github.com/kercylan98/vivid)。

## 快速开始

```go
n, _ := nexus.New(nexus.SessionActorProviderFN(func() (nexus.SessionActor, error) {
    return &MySessionActor{}, nil
}))

system.ActorOf(n.Actor())
n.TakeoverSession(mySession)
```

更多用法见 [vivid 文档](https://github.com/kercylan98/vivid/tree/main/docs) 与 [pkg.go.dev](https://pkg.go.dev/github.com/kercylan98/vivid-nexus)。

## 文档与许可

- API：[pkg.go.dev/github.com/kercylan98/vivid-nexus](https://pkg.go.dev/github.com/kercylan98/vivid-nexus)
- 许可证：[MIT](LICENSE)
- 欢迎 Issue 与 PR。
