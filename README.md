# adortb-consent

GDPR TCF 2.2 / CCPA 合规性同意管理服务。负责收集、存储用户同意记录，并为下游投放链路提供实时合规检查。

---

## 系统架构

Consent 服务作为合规检查层横切整个投放链路，所有涉及用户数据的环节均需通过本服务校验。

```
              ┌─────────────────────────────────────────┐
              │     Web SDK / iOS / Android / CTV       │
              └────────────────┬────────────────────────┘
                               ↓
                ┌─────────────────────────────────┐
                │  ★ Consent (GDPR TCF2.2 / CCPA) │ ← 合规检查层，横切所有投放链路
                └──────────────┬──────────────────┘
                               ↓
                       ┌───────────────┐
                       │   ADX Core    │◀──外部 DSP─┐
                       └───────┬───────┘            │
                   ┌───────────┼───────────┐        │
                   ↓           ↓           ↓        │
              ┌────────┐ ┌────────┐ ┌────────┐     │
              │  DSP   │ │  MMP   │ │  SSAI  │─────┘
              └───┬────┘ └───┬────┘ └────────┘
                  ↓          ↓
              ┌───────────────────────────┐
              │  Event Pipeline (Kafka)   │
              └───────┬───────────────────┘
        ┌─────────────┼─────────────┐
        ↓             ↓             ↓
  ┌─────────┐   ┌──────────┐   ┌──────────┐
  │ Billing │   │   DMP    │   │   CDP    │
  └─────────┘   └──────────┘   └──────────┘
```

**上游**：Web SDK / 移动 SDK 上报用户同意信号
**下游**：ADX Core 投放前调用 `/v1/consent/check` 检查合规性；DSP 据此决定个性化策略
**外部**：IAB Global Vendor List（https://vendor-list.consensu.org）每 24 小时刷新一次

---

## 快速开始

### 环境要求

- Go 1.25.3+
- PostgreSQL 14+（可选，未配置则使用内存存储）

### 本地运行

```bash
# 仅内存存储（开发 / 测试）
go run ./cmd/consent

# 使用 PostgreSQL
export DATABASE_URL="postgres://user:pass@localhost:5432/adortb_consent?sslmode=disable"
go run ./cmd/consent
```

### 数据库初始化

```bash
psql $DATABASE_URL -f migrations/001_consent.up.sql
```

### 环境变量

| 变量名           | 默认值   | 说明                                      |
|-----------------|---------|------------------------------------------|
| `DATABASE_URL`  | —       | PostgreSQL 连接串；未设置则使用内存存储      |
| `PORT`          | `8089`  | HTTP 服务监听端口                           |
| `METRICS_PORT`  | `9101`  | Prometheus 指标端口                         |

---

## HTTP API

| 方法   | 路径                    | 说明                                              |
|--------|------------------------|--------------------------------------------------|
| POST   | `/v1/consent`          | 保存用户同意记录                                   |
| GET    | `/v1/consent/{user_id}`| 获取指定用户的最新同意记录                          |
| POST   | `/v1/consent/decode`   | 解码 TCF 2.2 Consent String，返回结构化数据         |
| POST   | `/v1/consent/check`    | GDPR / CCPA 合规检查，返回 `allowed/reason/canPersonalize` |
| GET    | `/v1/vendors`          | 获取 GVL 厂商列表                                  |
| GET    | `/v1/gvl`             | 查询当前 GVL 版本号                                |
| GET    | `/health`             | 健康检查                                           |

### 合规检查请求示例

```json
POST /v1/consent/check
{
  "user_id": "u-12345",
  "vendor_id": 42,
  "purposes": [1, 3, 4]
}
```

响应：

```json
{
  "allowed": true,
  "reason": "consent granted",
  "canPersonalize": true
}
```

---

## 目录结构

```
adortb-consent/
├── cmd/consent/main.go              # 程序入口，初始化路由、存储、GVL 客户端
├── internal/
│   ├── api/handler.go               # HTTP 路由与请求处理
│   ├── tcf/decoder.go               # TCF 2.2 Consent String 位级解码
│   ├── tcf/encoder.go               # TCF 2.2 编码
│   ├── tcf/purposes.go              # IAB TCF Purpose 定义（Purpose 1-10+）
│   ├── usp/parser.go                # CCPA US Privacy String 解析（"1YNN" 格式）
│   ├── gvl/client.go                # Global Vendor List 客户端（24h 刷新）
│   ├── policy/matcher.go            # GDPR / CCPA 合规性匹配逻辑
│   ├── store/pg_repo.go             # PostgreSQL 存储实现
│   ├── store/memory_store.go        # 内存存储实现（测试 / 开发）
│   └── metrics/metrics.go           # Prometheus 指标定义
└── migrations/
    └── 001_consent.up.sql           # 建表及索引 DDL
```

---

## 核心技术特性

### TCF 2.2 实现

- 位级 BigEndian 编解码，100% 符合 IAB TCF 2.2 规范
- 支持 **BitField** 和 **Range** 两种厂商编码方式
- Purpose 覆盖 IAB 标准 Purpose 1-10+

### CCPA US Privacy String

- 解析四字符 `"1YNN"` 格式
- 自动检测 opt-out 状态

### GVL 缓存策略

- 服务启动后异步拉取，不阻塞主流程
- 本地缓存 24 小时，定时后台刷新

### 合规检查优先级

```
1. CCPA opt-out          → 直接拒绝
2. GDPR Purpose 1 缺失   → 拒绝（必需同意项）
3. Purpose 列表检查      → 所需 Purpose 未授权则拒绝
4. 厂商检查              → GVL 中厂商未授权则拒绝
5. 个性化判断            → Purpose 3 + Purpose 4 均已授权 → canPersonalize=true
```

### 双存储架构

- 设置 `DATABASE_URL` → 使用 PostgreSQL（生产）
- 未设置 `DATABASE_URL` → 使用内存存储（开发 / 测试，无需依赖）

---

## 数据库 Schema

```sql
-- consent_records
CREATE TABLE consent_records (
    id           BIGSERIAL PRIMARY KEY,
    user_id      TEXT        NOT NULL,
    consent_string TEXT,
    us_privacy   TEXT,
    gdpr_applies BOOLEAN,
    purposes     INT[],
    vendors      INT[],
    ip           TEXT,
    user_agent   TEXT,
    source       TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_consent_user_created
    ON consent_records (user_id, created_at DESC);
```

---

## 可观测性

Prometheus 指标通过独立端口 `9101` 暴露，与业务流量隔离。

推荐关注指标：

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `consent_check_total` | Counter | 合规检查总次数（按 allowed/denied 分类） |
| `consent_save_duration_seconds` | Histogram | 同意记录写入耗时 |
| `gvl_refresh_total` | Counter | GVL 刷新次数（按成功/失败分类） |
| `http_request_duration_seconds` | Histogram | HTTP 请求耗时 |

---

## 依赖

| 依赖 | 版本 | 用途 |
|------|------|------|
| `github.com/lib/pq` | v1.12.0 | PostgreSQL 驱动 |
| `github.com/prometheus/client_golang` | v1.20.0 | Prometheus 指标 |

详见 [docs/architecture.md](docs/architecture.md)。
