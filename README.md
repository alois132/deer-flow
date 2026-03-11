# DeerFlow

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-blue?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/Framework-Eino-FF6B6B?style=flat-square" alt="Framework">
</p>

<p align="center">
  <b>DeerFlow 2.0</b> - 开源 AI Super Agent，具备持久化记忆与沙箱执行能力
</p>

<p align="center">
  <a href="#特性">特性</a> •
  <a href="#快速开始">快速开始</a> •
  <a href="#架构">架构</a> •
  <a href="#配置">配置</a> •
  <a href="#工具">工具</a> •
  <a href="#记忆系统">记忆系统</a>
</p>

---

## 简介

DeerFlow 是一个基于 Go 语言开发的开源 AI Super Agent，采用 [CloudWeGo Eino](https://github.com/cloudwego/eino) 框架构建。它集成了多提供商 LLM 支持、持久化记忆系统和安全的沙箱执行环境，能够执行复杂的任务并持续学习用户偏好。

## 特性

- **多提供商 LLM 支持** - 支持 OpenAI 兼容 API 和 Ark (ByteDance) 模型
- **持久化记忆系统** - 基于 Redis 的智能记忆，自动提取和分类用户偏好、知识和行为模式
- **安全沙箱执行** - Docker 容器化执行环境，支持自动启动和空闲超时管理
- **丰富的工具集** - 内置文件操作、命令执行等工具
- **流式响应** - 实时流式输出，提供流畅的交互体验
- **线程隔离** - 每个对话线程拥有独立的沙箱环境

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│                         DeerFlow                            │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │    Leader    │  │    Memory    │  │   Sandbox    │      │
│  │    Agent     │  │   Service    │  │   Manager    │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
└─────────┼─────────────────┼─────────────────┼──────────────┘
          │                 │                 │
    ┌─────▼─────┐    ┌─────▼─────┐    ┌─────▼─────┐
    │   LLM     │    │  Redis    │    │  Docker   │
    │ Providers │    │           │    │  Sandbox  │
    └───────────┘    └───────────┘    └───────────┘
```

## 快速开始

### 环境要求

- Go 1.25+
- Docker
- MySQL 8.0+
- Redis 7.0+

### 安装

1. 克隆仓库

```bash
git clone https://github.com/yourusername/deer-flow.git
cd deer-flow
```

2. 启动依赖服务

```bash
docker-compose up -d
```

3. 配置环境

```bash
cp config.yaml.example config.yaml
# 编辑 config.yaml，配置你的 LLM API 密钥
```

4. 运行应用

```bash
go run cmd/main.go
```

## 配置

配置文件 `config.yaml` 示例：

```yaml
server:
  port: 8080
  mode: dev

database:
  master:
    driver: mysql
    host: localhost
    port: 3306
    username: root
    password: password
    database: deerflow

redis:
  host: localhost
  port: 6379

agent:
  default_llm:
    provider: openai
    api_key: sk-your-api-key
    model: kimi-k2.5
    base_url: https://api.moonshot.cn/v1

sandbox:
  auto_start: true
  container_prefix: deer-flow-
  idle_timeout: 3600
```

## 工具

DeerFlow 内置以下工具：

| 工具 | 描述 |
|------|------|
| `bash` | 在沙箱中执行 bash 命令 |
| `ls` | 列出目录内容 |
| `read_file` | 读取文件内容，支持行范围 |
| `write_file` | 写入文件内容 |
| `str_replace` | 替换文件中的字符串 |

## 记忆系统

DeerFlow 的记忆系统能够：

- **用户上下文** - 存储用户偏好、习惯和历史交互
- **工作上下文** - 记录当前任务和项目相关信息
- **个人上下文** - 维护用户的个人信息和背景
- **事实提取** - 自动从对话中提取并分类事实：
  - 偏好 (Preferences)
  - 知识 (Knowledge)
  - 行为 (Behavior)
  - 目标 (Goals)

记忆使用 Redis 持久化，并在每次对话后通过 LLM 分析自动更新。

## 沙箱环境

- **自动启动** - 按需自动创建和启动沙箱容器
- **空闲超时** - 默认 10 分钟空闲后自动清理
- **线程隔离** - 每个线程拥有独立的沙箱实例
- **卷挂载** - 支持技能、上传文件、工作区和输出的卷挂载

## 开发

### 项目结构

```
deer-flow/
├── cmd/main.go              # 应用入口
├── config.yaml.example      # 配置示例
├── docker-compose.yaml      # 依赖服务
├── internal/
│   ├── agent/               # Agent 核心逻辑
│   │   ├── leader.go        # Leader Agent
│   │   ├── tools.go         # 工具定义
│   │   └── memory/          # 记忆服务
│   └── global/              # 全局配置和实例
├── pkg/
│   ├── database/            # 数据库配置
│   ├── llm/                 # LLM 提供商集成
│   ├── log/                 # 日志系统
│   ├── redis/               # Redis 配置
│   └── sandbox/             # 沙箱管理
└── utils/                   # 工具函数
```

### 依赖的主要库

- [Eino](https://github.com/cloudwego/eino) - Go 语言 LLM 应用框架
- [sandbox-sdk-go](https://github.com/agent-infra/sandbox-sdk-go) - 沙箱环境 SDK
- [GORM](https://gorm.io/) - ORM 框架
- [Zap](https://github.com/uber-go/zap) - 结构化日志
- [Viper](https://github.com/spf13/viper) - 配置管理

## 许可证

MIT License

## 致谢

- [CloudWeGo Eino](https://github.com/cloudwego/eino) - 强大的 Go 语言 LLM 应用框架
- [Agent Infra](https://github.com/agent-infra) - 沙箱基础设施
