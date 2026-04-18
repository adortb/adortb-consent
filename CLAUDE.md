# CLAUDE.md — adortb-consent

> 本文件供 Claude Code 快速了解项目上下文，开始任务前必须通读。

---

## 项目定位

`adortb-consent` 是 GDPR TCF 2.2 / CCPA 合规性同意管理服务。
它横切整个广告投放链路，ADX Core 在每次投放前必须调用本服务完成合规检查。

---

## 技术栈

| 项目 | 版本 |
|------|------|
| Go | 1.25.3 |
| PostgreSQL 驱动 | lib/pq v1.12.0 |
| Prometheus | client_golang v1.20.0 |
| HTTP 端口 | 8089（`PORT`） |
| Metrics 端口 | 9101（`METRICS_PORT`） |

---

## 目录约定

```
cmd/consent/main.go          # 程序入口，只做组装（DI），不含业务逻辑
internal/api/handler.go      # HTTP 路由，薄层，调用 policy/store
internal/tcf/                # TCF 2.2 编解码，纯函数，无 IO
internal/usp/                # CCPA US Privacy String 解析，纯函数
internal/gvl/client.go       # GVL 客户端，有 IO，需 mock 测试
internal/policy/matcher.go   # 合规判断核心逻辑，依赖 tcf/usp/gvl
internal/store/              # 存储层接口 + PG 实现 + 内存实现
internal/metrics/metrics.go  # 全局 Prometheus 指标注册
migrations/                  # 仅增量 .up.sql，禁止修改已合并的迁移文件
```

**原则**：`tcf/` 和 `usp/` 为纯计算包，不得引入任何 IO 依赖。

---

## 关键业务规则（不可随意修改）

### 合规检查优先级（policy/matcher.go）

```
1. CCPA opt-out          → allowed=false, canPersonalize=false
2. GDPR Purpose 1 缺失   → allowed=false（Purpose 1 = 存储/访问，必需项）
3. 所需 Purpose 未授权   → allowed=false
4. 厂商未在 GVL 授权     → allowed=false
5. Purpose 3 AND 4 均有  → canPersonalize=true
```

此优先级由 IAB TCF 2.2 规范和 CCPA 法规决定，修改前必须同步法务确认。

### TCF 2.2 位操作

- 编解码均使用 **BigEndian** 位序
- 厂商编码支持两种格式：**BitField** 和 **Range**，解码时根据 `IsRangeEncoding` 标志自动选择

### GVL 刷新

- 启动时**异步**拉取，主服务不等待 GVL 就绪即可对外服务
- 刷新间隔 24 小时；刷新失败时保留上一次缓存，不影响服务可用性
- 外部地址：`https://vendor-list.consensu.org`

---

## 存储策略

| 场景 | 实现 |
|------|------|
| 生产 / 集成测试 | `store/pg_repo.go`（需 `DATABASE_URL`） |
| 单元测试 / 本地开发 | `store/memory_store.go`（无外部依赖） |

两者实现同一 `Store` 接口，切换仅靠环境变量，代码无需修改。

---

## 编码规范

- 函数不超过 50 行，文件不超过 800 行
- 错误必须向上传递，严禁 `_ = err` 吞掉错误
- 禁止 `panic`；所有可预见的边界情况返回 `error`
- 不可变数据优先：tcf/usp 解码结果为只读结构体，禁止在外部修改字段
- IP 优先从 `X-Forwarded-For` 提取，回退到 `RemoteAddr`

---

## 测试要求

- 覆盖率目标 **80%+**
- `tcf/` 和 `usp/` 必须有完整的位操作边界用例（空字符串、全 0、全 1、Range 编码）
- `policy/matcher.go` 须覆盖上述五条优先级路径的正反两个方向
- `store/` 接口测试通过内存实现运行，无需真实数据库
- GVL client 使用 `httptest.Server` mock，不访问真实网络

运行测试：

```bash
go test ./... -race -count=1
```

---

## 常见开发任务

### 添加新的 HTTP 端点

1. 在 `internal/api/handler.go` 注册路由
2. 在对应 `internal/` 子包实现业务逻辑
3. 在 `internal/metrics/metrics.go` 添加监控指标
4. 补充单元测试

### 修改合规检查逻辑

1. 阅读 `internal/policy/matcher.go` 和本文件"关键业务规则"章节
2. 更新 `matcher.go` 并同步更新测试
3. 在 PR 描述中说明变更原因，并注明是否经过法务评审

### 数据库变更

1. 在 `migrations/` 创建新的 `00N_xxx.up.sql`（递增编号）
2. 禁止修改已合并的迁移文件
3. 同步更新 `store/pg_repo.go` 中的查询

---

## 本地调试

```bash
# 无数据库启动（内存模式）
go run ./cmd/consent

# 带 PostgreSQL
DATABASE_URL="postgres://user:pass@localhost:5432/adortb_consent?sslmode=disable" \
go run ./cmd/consent

# 健康检查
curl http://localhost:8089/health

# 解码 TCF Consent String
curl -X POST http://localhost:8089/v1/consent/decode \
  -H 'Content-Type: application/json' \
  -d '{"consent_string":"COvFyGBOvFyGBAbAAAENAPCAAOAAAAAAAAAAAEEUAACEAAAAA"}'
```

---

## 与其他服务的协作

| 方向 | 服务 | 接口 |
|------|------|------|
| 上游 | Web SDK / 移动 SDK | `POST /v1/consent` |
| 下游 | ADX Core | `POST /v1/consent/check` |
| 下游 | DSP 个性化决策 | 读取 `canPersonalize` 字段 |
| 外部 | IAB GVL | HTTP GET（24h 缓存） |

详细架构见 [docs/architecture.md](docs/architecture.md)。
