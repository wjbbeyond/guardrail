# GuardRail — AI Agent 安全网关 开发计划

> 制定日期：2026-06-26 | 制定人：🔬 雷达 (research-agent)
> 状态：待老大确认
> 关联文档：[社区痛点调研与开源项目方案](2026-06-24-社区痛点调研与开源项目方案.md)

---

## 一、项目定位与目标

### 1.1 一句话定义

GuardRail 是一个**轻量级开源反向代理**，部署在 AI Agent 与 LLM API 之间，提供**安全防护、成本熔断、API 可靠性**三层核心能力。

### 1.2 解决的核心痛点

| 痛点 | 严重度 | GuardRail 的解法 |
|------|--------|------------------|
| Agent 安全爆炸半径（数据泄露、破坏性操作） | 9/10 | Prompt Injection 检测、Tool Call 权限管控、敏感数据脱敏 |
| LLM API 成本失控（$34K 账单事故） | 8/10 | 实时 Token 计量、预算熔断器、消费告警 |
| LLM API 不可靠（Provider 宕机、级联崩溃） | 8/10 | 多 Provider 自动故障转移、智能重试、模型版本锁定 |

### 1.3 目标用户

- **中小企业/独立开发者**：需要成本控制和基本安全防护
- **企业 AI 平台团队**：需要安全合规、审计日志
- **DevSecOps 工程师**：需要 Agent 级别的安全治理

---

## 二、竞品分析与差异化

| 项目 | 语言 | Stars(估) | 安全 | 成本控制 | 可靠性 | 定位 |
|------|------|-----------|------|----------|--------|------|
| **LiteLLM** | Python | 20K+ | ❌ 基础 | ✅ Token 计数 | ✅ 负载均衡/故障转移 | 统一 LLM 接口 |
| **Portkey Gateway** | TypeScript | 12K+ | ✅ Prompt Guard | ✅ 缓存/限制 | ✅ 负载均衡 | 通用 AI 网关 |
| **Agentgateway** | Rust | 5K+ | ✅ MCP 安全 | ❌ | ✅ 路由 | MCP/A2A 协议网关 |
| **APISIX AI Gateway** | Lua/C | 15K+ | ✅ 认证 | ✅ Rate Limit | ✅ 熔断 | 传统网关扩展 AI |
| **Circuit Breaker** | Python | ~200 | ❌ | ✅ 成本天花板 | ❌ | 专注成本熔断 |
| **GuardRail（我们）** | Go | 新项目 | ✅ **深度** | ✅ **熔断+预测** | ✅ **版本锁定+回归** | 安全+成本+可靠性 三位一体 |

### 差异化策略

1. **三位一体**：不只做路由或只做安全，而是安全+成本+可靠性一个套件搞定
2. **Agent 原生**：不是传统 API 网关加了 AI 插件，而是从 Agent 使用场景出发设计
3. **零配置启动**：`docker run` 一行命令就能用，不需要复杂的 YAML 配置
4. **安全深度**：Prompt Injection 检测、Tool Call RBAC、PII 脱敏——不止于 Rate Limit

---

## 三、技术架构

### 3.1 技术栈选择

| 层级 | 选型 | 理由 |
|------|------|------|
| **核心语言** | Go | 高并发、低内存、单二进制部署、适合网关场景 |
| **配置格式** | YAML + 环境变量 | 人类可读、K8s 友好 |
| **存储（元数据/日志）** | SQLite（默认）/ PostgreSQL（生产） | 零依赖启动、可水平扩展 |
| **缓存** | 内置 LRU + 可选 Redis | 语义缓存减少重复 LLM 调用 |
| **Web UI** | Svelte + TailwindCSS | 轻量、编译为静态文件、嵌入二进制 |
| **可观测性** | OpenTelemetry SDK | 行业标准，可对接 Jaeger/Grafana |
| **容器化** | Docker + Docker Compose | 开发和部署标准化 |
| **测试** | Go testing + testify + httptest | 标准工具链 |

### 3.2 系统架构图

```
                        ┌─────────────────┐
                        │   AI Agent 应用   │
                        │  (LangChain etc) │
                        └────────┬────────┘
                                 │ OpenAI-compatible API
                                 ▼
                    ┌────────────────────────┐
                    │     GuardRail Gateway  │
                    │    (反向代理, :8080)     │
                    ├────────────────────────┤
                    │  ┌──────────────────┐  │
                    │  │  Middleware Chain │  │
                    │  │  ┌────────────┐  │  │
                    │  │  │ Auth       │  │  │
                    │  │  ├────────────┤  │  │
                    │  │  │ Rate Limit │  │  │
                    │  │  ├────────────┤  │  │
                    │  │  │ Cost Guard │  │  │
                    │  │  ├────────────┤  │  │
                    │  │  │ Security   │  │  │
                    │  │  ├────────────┤  │  │
                    │  │  │ Transform  │  │  │
                    │  │  └────────────┘  │  │
                    ├────────────────────────┤
                    │  ┌──────────────────┐  │
                    │  │  Router / LB     │  │
                    │  │  (故障转移/降级) │  │
                    │  └──────────────────┘  │
                    ├────────────────────────┤
                    │  ┌──────────────────┐  │
                    │  │  Cache Layer     │  │
                    │  │  (语义缓存/精确) │  │
                    │  └──────────────────┘  │
                    ├────────────────────────┤
                    │  ┌──────────────────┐  │
                    │  │  Audit Logger   │  │
                    │  └──────────────────┘  │
                    ├────────────────────────┤
                    │  ┌──────────────────┐  │
                    │  │  Admin API / UI  │  │
                    │  │  (:9090)         │  │
                    │  └──────────────────┘  │
                    └────────────────────────┘
                                 │
                    ┌────────────┼────────────┐
                    ▼            ▼            ▼
              ┌──────────┐ ┌──────────┐ ┌──────────┐
              │  OpenAI  │ │ Anthropic │ │  Google  │
              │  API     │ │  Claude  │ │  Gemini  │
              └──────────┘ └──────────┘ └──────────┘
```

### 3.3 项目结构

```
guardrail/
├── cmd/
│   ├── guardrail/          # 主程序入口
│   │   └── main.go
│   └── guardrail-cli/      # CLI 管理工具
│       └── main.go
├── internal/
│   ├── gateway/             # HTTP 反向代理核心
│   │   ├── proxy.go        # 请求转发
│   │   ├── middleware.go    # Middleware 链
│   │   └── server.go       # HTTP server
│   ├── security/
│   │   ├── prompt_guard.go     # Prompt Injection 检测
│   │   ├── pii_scanner.go      # PII/密钥检测
│   │   ├── tool_permission.go  # Tool Call 权限管控
│   │   └── response_filter.go  # 响应内容过滤
│   ├── cost/
│   │   ├── counter.go      # Token 计量
│   │   ├── circuit.go      # 熔断器
│   │   ├── budget.go       # 预算管理
│   │   └── pricer.go       # 价格计算（含各 Provider 价目表）
│   ├── reliability/
│   │   ├── router.go       # 路由决策
│   │   ├── failover.go     # 故障转移
│   │   ├── retry.go        # 智能重试
│   │   └── version_lock.go # 模型版本锁定
│   ├── cache/
│   │   ├── cache.go        # 缓存接口
│   │   ├── exact.go        # 精确匹配缓存
│   │   └── semantic.go     # 语义缓存（基于 embedding 相似度）
│   ├── audit/
│   │   ├── logger.go       # 审计日志
│   │   ├── store.go        # 日志存储
│   │   └── compliance.go   # 合规报告生成
│   ├── config/
│   │   └── config.go       # 配置加载/校验
│   ├── admin/
│   │   ├── api.go          # 管理 API
│   │   └── dashboard.go    # 内嵌 Web UI
│   └── pkg/                # 内部公共工具
│       ├── otel/           # OpenTelemetry 集成
│       └── testutil/       # 测试工具
├── web/                    # Dashboard 前端（Svelte）
│   ├── src/
│   ├── package.json
│   └── vite.config.ts
├── configs/
│   ├── guardrail.yaml      # 默认配置
│   └── guardrail.dev.yaml  # 开发配置
├── deployments/
│   ├── Dockerfile
│   ├── docker-compose.yaml
│   └── k8s/                # Kubernetes manifests
├── scripts/
│   ├── build.sh
│   └── test-integration.sh
├── docs/
│   ├── architecture.md
│   ├── configuration.md
│   ├── security-model.md
│   └── api-reference.md
├── go.mod
├── go.sum
├── Makefile
├── LICENSE
├── README.md
├── CONTRIBUTING.md
├── CHANGELOG.md
└── .github/
    ├── workflows/
    │   ├── ci.yaml
    │   ├── release.yaml
    │   └── dependabot.yaml
    └── PULL_REQUEST_TEMPLATE.md
```

### 3.4 核心模块说明

#### 模块 A：Security Layer（安全层）

**Prompt Injection 检测**
- 基于规则 + 轻量模型的混合检测
- 规则引擎：正则匹配已知攻击模式（`ignore previous instructions`、角色注入、分隔符攻击等）
- 可选：调用小模型（如 GPT-4o-mini）做二次判定（延迟 <100ms）
- 动作：拦截/警告/放行（可配置策略）

**PII/密钥检测**
- 内置常见模式：邮箱、电话、身份证、API Key、信用卡号
- 支持自定义正则规则
- 动作：脱敏替换/拦截/记录

**Tool Call 权限管控**
- 基于角色的 Agent 权限（RBAC）
- 可定义每个 Agent/用户的 Tool Call 白名单/黑名单
- 危险操作标记（删除、写入、外部 API 调用等需要二次确认）

#### 模块 B：Cost Control Layer（成本控制层）

**实时 Token 计量**
- 请求/响应流式解析，实时计数 prompt_tokens / completion_tokens
- 支持所有主流 Provider 的 token 计数差异
- 每 API Key / 每 Agent / 每 User 维度累计

**熔断器**
- 配置每日/每月/每请求预算上限
- 触发阈值后：拒绝请求 / 降级到廉价模型 / 发告警
- 支持软限制（告警）+ 硬限制（熔断）两级

**价格计算**
- 内置各 Provider 各模型的实时价格表（定期更新）
- 支持自定义价格（私有部署模型场景）

#### 模块 C：Reliability Layer（可靠性层）

**多 Provider 故障转移**
- 配置主/备 Provider 列表
- 5xx / 429 / timeout 自动切换到备用 Provider
- 支持跨模型降级（如 GPT-4o → GPT-4o-mini）

**模型版本锁定**
- 明确指定模型版本（`gpt-4o-2024-08-06` 而非 `gpt-4o`）
- 检测 Provider 静默更新模型行为（响应格式变化告警）

**智能重试**
- 指数退避重试（可配置最大次数和间隔）
- 429 错误遵守 Retry-After header
- 幂等性检查避免重复处理

#### 模块 D：Cache Layer（缓存层）

**精确缓存**
- 基于 `model + system_prompt + user_prompt` 的精确匹配
- 可配置 TTL
- HIT 时直接返回，不调用 LLM

**语义缓存（MVP 后期）**
- 基于 embedding 相似度匹配
- 相似度阈值可配（默认 0.95）
- 需要额外 embedding API 调用（计入成本）

---

## 四、MVP 范围定义

### 4.1 MVP 包含（v0.1.0）

| 功能 | 优先级 | 说明 |
|------|--------|------|
| ✅ HTTP 反向代理 | P0 | 透明代理 OpenAI 兼容 API |
| ✅ 多 Provider 路由 | P0 | 支持 OpenAI / Anthropic / Google |
| ✅ API Key 管理 | P0 | 多 Key 池、自动轮换 |
| ✅ Token 计数 | P0 | 实时 prompt/completion token 计数 |
| ✅ 成本追踪 | P0 | 每 Key/每用户的累计花费 |
| ✅ 熔断器 | P0 | 预算上限触发拒绝 |
| ✅ Prompt Injection 检测（规则版） | P1 | 基于正则的轻量检测 |
| ✅ PII 检测 | P1 | 基础敏感信息模式匹配 |
| ✅ 审计日志 | P1 | 请求/响应记录到 SQLite |
| ✅ YAML 配置 | P0 | 配置文件驱动 |
| ✅ Docker 部署 | P0 | 单容器部署 |
| ✅ 健康检查 / Metrics | P0 | `/healthz` + Prometheus 格式 |

### 4.2 MVP 不包含（后续版本）

| 功能 | 计划版本 | 说明 |
|------|----------|------|
| ❌ Web Dashboard | v0.2.0 | 先用 API + curl 管理 |
| ❌ 语义缓存 | v0.2.0 | 需要额外的 embedding 基础设施 |
| ❌ Tool Call RBAC | v0.2.0 | 需要解析 tool_calls 结构 |
| ❌ 模型版本回归检测 | v0.3.0 | 需要历史基线数据积累 |
| ❌ 合规报告（GDPR/HIPAA） | v0.3.0 | 需要法律咨询 |
| ❌ 流式响应缓存 | v0.3.0 | 技术复杂度高 |
| ❌ K8s Operator | v0.4.0 | 社区有需求再开发 |
| ❌ 多语言 SDK | v0.4.0 | 先确保核心稳定 |

---

## 五、开发里程碑

### Phase 1：基础骨架（2 周）

**目标：能跑起来的最小代理**

| # | 任务 | 产出物 | 工时 |
|---|------|--------|------|
| 1.1 | 项目初始化：Go module、目录结构、CI | 可编译的空项目 | 1天 |
| 1.2 | 配置加载模块（YAML + 环境变量） | `internal/config/` | 1天 |
| 1.3 | HTTP 反向代理核心 | 透明代理 OpenAI chat completions | 3天 |
| 1.4 | 流式响应透传（SSE） | 支持 streaming: true | 2天 |
| 1.5 | 多 Provider 适配（OpenAI/Anthropic/Google） | 请求格式转换 | 3天 |
| 1.6 | API Key 管理（池轮换） | 多 Key 负载均衡 | 1天 |
| 1.7 | 基础测试 + Docker 构建 | Dockerfile + docker-compose | 1天 |

**Phase 1 产出物：**
- ✅ `docker run` 即可使用的 LLM 代理
- ✅ 支持切换 Provider
- ✅ 基础单元测试覆盖率 >70%

**预计工时：12 天**

---

### Phase 2：成本控制（1.5 周）

**目标：能防住 $34K 账单事故**

| # | 任务 | 产出物 | 工时 |
|---|------|--------|------|
| 2.1 | Token 计数模块 | 流式/非流式实时计数 | 2天 |
| 2.2 | 价格计算模块 | 各 Provider 价格表 | 1天 |
| 2.3 | 成本累计存储 | SQLite 持久化 | 1天 |
| 2.4 | 熔断器实现 | 预算触达拦截请求 | 2天 |
| 2.5 | 管理 API：成本查询 + 重置 | REST API | 1天 |
| 2.6 | Prometheus Metrics | token/cost/request metrics | 1天 |

**Phase 2 产出物：**
- ✅ 实时显示每个 Key 的 token 消耗和花费
- ✅ 预算超限自动拦截
- ✅ Grafana 可视化

**预计工时：8 天**

---

### Phase 3：安全防护（2 周）

**目标：基本的 Prompt Injection 和 PII 防护**

| # | 任务 | 产出物 | 工时 |
|---|------|--------|------|
| 3.1 | Prompt Injection 规则引擎 | 可扩展的规则集 | 3天 |
| 3.2 | PII 检测模块 | 内置模式 + 自定义规则 | 2天 |
| 3.3 | Middleware Chain 架构 | 可插拔的中间件链 | 2天 |
| 3.4 | 安全策略配置 | 允许/拒绝/脱敏策略 | 1天 |
| 3.5 | 安全事件日志 | 结构化安全审计日志 | 2天 |
| 3.6 | 安全测试集 | 已知攻击 payload 测试用例 | 2天 |

**Phase 3 产出物：**
- ✅ 能检测常见 Prompt Injection 攻击
- ✅ 能检测和脱敏 PII
- ✅ 完整的安全事件日志

**预计工时：12 天**

---

### Phase 4：可靠性 + 审计（1.5 周）

**目标：生产级可靠性**

| # | 任务 | 产出物 | 工时 |
|---|------|--------|------|
| 4.1 | 故障转移逻辑 | 主备 Provider 自动切换 | 3天 |
| 4.2 | 智能重试 | 指数退避 + 429 处理 | 2天 |
| 4.3 | 模型版本锁定 | 拒绝别名/强制版本号 | 1天 |
| 4.4 | 审计日志完善 | 结构化 JSON 日志 + 查询 API | 2天 |
| 4.5 | OpenTelemetry 集成 | Trace + Span 导出 | 2天 |

**Phase 4 产出物：**
- ✅ Provider 挂了自动切换
- ✅ 完整的分布式追踪
- ✅ 可接入企业 SIEM

**预计工时：10 天**

---

### Phase 5：开源发布准备（1 周）

**目标：专业的开源项目形象**

| # | 任务 | 产出物 | 工时 |
|---|------|--------|------|
| 5.1 | README 完善 | 架构图、快速开始、配置示例 | 1天 |
| 5.2 | CONTRIBUTING.md | 贡献流程、代码规范 | 0.5天 |
| 5.3 | 文档网站（可选） | GitHub Pages / Mintlify | 1天 |
| 5.4 | GitHub Actions CI/CD | 自动测试 + 发布 | 1天 |
| 5.5 | 示例配置 + Quick Start | 5 分钟上手体验 | 1天 |
| 5.6 | CHANGELOG + Release notes | v0.1.0 发布 | 0.5天 |
| 5.7 | 社区准备 | Issue/PR Template, Discord 频道 | 1天 |

**Phase 5 产出物：**
- ✅ 专业的 GitHub 仓库
- ✅ 完整的入门文档
- ✅ v0.1.0 正式发布

**预计工时：6 天**

---

### 总结：工时估算

| Phase | 内容 | 工时 | 累计 |
|-------|------|------|------|
| Phase 1 | 基础骨架 | 12天 | 12天 |
| Phase 2 | 成本控制 | 8天 | 20天 |
| Phase 3 | 安全防护 | 12天 | 32天 |
| Phase 4 | 可靠性+审计 | 10天 | 42天 |
| Phase 5 | 开源发布准备 | 6天 | 48天 |

**MVP 总工时：约 48 人天（单人开发 ≈ 10 周）**

> 如果有 2 人协作（1 后端 + 1 全栈/安全），可压缩到 6-7 周。

---

## 六、开源策略

### 6.1 License 选择

**推荐：Apache 2.0**

| License | 商业友好 | 专利保护 | 社区接受度 | 适合场景 |
|---------|----------|----------|------------|----------|
| MIT | ✅ 最高 | ❌ 无 | ✅ 高 | 纯工具库 |
| Apache 2.0 | ✅ 高 | ✅ 有 | ✅ 高 | 基础设施/网关 |
| GPL 3.0 | ❌ 低 | ✅ 有 | ⚠️ 中 | 防止闭源商业化 |
| BSL | ❌ 中 | ✅ 有 | ⚠️ 低 | 商业+开源混合 |

**理由：**
- Apache 2.0 有专利授权条款，对企业用户友好
- 允许商业使用和修改，降低采用门槛
- 与 LiteLLM、Portkey Gateway 等竞品一致
- 不限制未来商业化（如 GuardRail Cloud 托管版）

### 6.2 GitHub 仓库结构

```
guardrail/                        # GitHub org: guardrail-ai
├── README.md                     # 项目首页（最重要）
├── LICENSE                       # Apache 2.0
├── CONTRIBUTING.md               # 贡献指南
├── CODE_OF_CONDUCT.md            # 行为准则
├── CHANGELOG.md                  # 变更日志
├── SECURITY.md                   # 安全漏洞报告流程
├── Makefile                      # 常用命令
├── go.mod / go.sum
├── cmd/                          # 入口
├── internal/                     # 核心代码
├── web/                          # Dashboard 前端
├── configs/                      # 示例配置
│   └── examples/
│       ├── quick-start.yaml
│       ├── multi-provider.yaml
│       └── security-hardened.yaml
├── deployments/
│   ├── Dockerfile
│   ├── docker-compose.yaml
│   └── k8s/
├── docs/                         # 详细文档
├── scripts/                      # 构建/测试脚本
├── test/                         # 集成测试
│   └── fixtures/
│       ├── prompt_injection_payloads.json
│       └── pii_test_cases.json
└── .github/
    ├── workflows/
    ├── ISSUE_TEMPLATE/
    │   ├── bug_report.md
    │   ├── feature_request.md
    │   └── security_vulnerability.md
    └── PULL_REQUEST_TEMPLATE.md
```

### 6.3 README 框架

```markdown
# GuardRail 🔒

> AI Agent 安全网关 — 安全防护、成本熔断、API 可靠性，一个代理全搞定。

[![CI](badge)](#) [![License: Apache 2.0](badge)](#) [![Go Report Card](badge)](#)

## ✨ Features
## 🚀 Quick Start (5 分钟)
## 📦 Installation
## ⚙️ Configuration
## 🔐 Security
## 💰 Cost Control
## 📊 Observability
## 🏗 Architecture
## 🤝 Contributing
## 📄 License
## 🙏 Acknowledgments
```

### 6.4 贡献指南要点

- `CONTRIBUTING.md` 包含：
  - 开发环境搭建（Go 1.22+、Node 20+）
  - 代码风格（`gofmt`、ESLint）
  - PR 流程：Fork → Branch → Test → PR → Review → Merge
  - Commit message 规范（Conventional Commits）
  - 安全相关的 PR 需要额外 Review

- 好的首次贡献（good first issue）：
  - 新增 PII 检测模式
  - 新增 Provider 价格表更新
  - 文档翻译
  - 测试用例补充

### 6.5 社区运营计划

| 时间 | 行动 |
|------|------|
| 发布前 | 准备 3-5 篇技术博客素材（架构设计、安全模型、成本控制案例） |
| 发布日 | 同步发 Reddit r/LLMDevs、r/AI_Agents、Hacker News Show HN |
| 发布后第1周 | 在 X/Twitter 做技术 thread |
| 发布后第2周 | 提交到相关 Awesome 列表、Go Weekly |
| 持续 | 维护 Discord/Telegram 社区 |

---

## 七、技术风险与依赖

### 7.1 技术风险

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| **SSE 流式透传复杂度高** | 流式响应解析容易丢数据或阻塞 | 高 | Phase 1 重点攻关；充分测试；参考 LiteLLM 实现 |
| **Provider API 格式差异大** | OpenAI/Anthropic/Google 请求响应格式不一致 | 高 | 抽象统一的内部格式；为每个 Provider 写适配器；测试覆盖 |
| **Prompt Injection 检测误报率高** | 影响正常业务流量 | 中 | 默认模式="warn-only"（只记录不拦截）；用户可调整阈值 |
| **Token 计数不准确** | 成本计算偏差 | 中 | 使用 tiktoken（OpenAI）、cl100k_base 分词；允许手动覆盖 |
| **性能瓶颈** | 代理增加延迟 | 低 | Go 天生高并发；Middleware 链异步化；目标 P99 <50ms 额外延迟 |

### 7.2 外部依赖

| 依赖 | 用途 | 风险 | 替代方案 |
|------|------|------|----------|
| Go 标准库 + net/http | HTTP 代理 | 低 | — |
| tiktoken-go | Token 计数 | 中 | 按 character 估算（精度降 5-10%） |
| SQLite（modernc.org/sqlite） | 日志存储 | 低 | 纯 Go 实现，无 CGO 依赖 |
| Prometheus client | Metrics 导出 | 低 | 自定义格式 |
| OpenTelemetry SDK | 分布式追踪 | 低 | 可选模块 |
| embedding API（语义缓存） | 缓存相似度 | 中 | v0.1 不含；v0.2 选 OpenAI / 本地模型 |
| Docker / Docker Compose | 部署 | 低 | 也支持直接编译二进制 |

---

## 八、版本路线图

| 版本 | 时间 | 核心能力 | 标志性功能 |
|------|------|----------|------------|
| **v0.1.0** | Week 10 | 代理 + 成本控制 + 基础安全 | 能跑、能防账单事故 |
| **v0.2.0** | Week 16 | Dashboard + 语义缓存 + Tool RBAC | 有 UI、能缓存、能管权限 |
| **v0.3.0** | Week 22 | 合规报告 + 模型回归检测 + 多租户 | 企业可部署 |
| **v0.4.0** | Week 30 | K8s Operator + 多语言 SDK + 社区插件 | 生态扩展 |

---

## 九、关键决策记录（ADR）

### ADR-1：为什么选 Go 而不是 Python/Rust？

- **Python**：LiteLLM 已占据生态位，且 Python 在高并发代理场景性能不足
- **Rust**：Agentgateway 已占据 Rust 生态位，且 Rust 开发效率低、招人难
- **Go**：网关/代理场景的黄金语言（Envoy 是 C++ 但生态配套用 Go），编译为单二进制，部署极简，招人容易

### ADR-2：为什么是反向代理而不是 SDK？

- SDK 需要每个语言写一遍，且需要改 Agent 应用代码
- 反向代理**零侵入**——Agent 应用只需改一个 URL（base_url 指向 GuardRail）
- 代理可以做全局管控（安全策略、成本限制），SDK 只能做请求级

### ADR-3：为什么先不做 AI 模型的安全检测？

- MVP 阶段用规则引擎就够了（覆盖 80% 的已知攻击模式）
- AI 模型检测需要额外 API 调用，增加延迟和成本
- 积累真实攻击数据后，v0.2 可以训练/微调专用检测模型

---

## 十、下一步行动

| # | 行动 | 负责人 | 优先级 |
|---|------|--------|--------|
| 1 | 老大确认开发计划，调整优先级 | 老大 | urgent |
| 2 | 确定 GitHub 组织名（guardrail-ai）和仓库名 | 老大 | urgent |
| 3 | 确认开发者资源（自研？外包？招人？） | 老大 | urgent |
| 4 | 创建 GitHub 仓库，初始化项目结构 | coding-agent | high |
| 5 | Phase 1 开发启动 | coding-agent | high |

---

*计划制定完成。等待老大确认后进入开发阶段。*