# 架构设计文档

> adortb-consent 内部架构、核心算法流程及系统交互时序

---

## 1. 内部架构图

```
┌──────────────────────────────────────────────────────────────┐
│                        HTTP :8089                            │
│                   internal/api/handler.go                    │
│   POST /v1/consent      GET /v1/consent/{id}                │
│   POST /v1/consent/decode   POST /v1/consent/check          │
│   GET /v1/vendors       GET /v1/gvl      GET /health        │
└───────────┬──────────────────────┬───────────────────────────┘
            │                      │
     ┌──────▼──────┐        ┌──────▼───────────────────────┐
     │ Store       │        │ policy/matcher.go             │
     │ interface   │        │  合规检查核心                  │
     │─────────────│        │─────────────────────────────  │
     │ pg_repo.go  │        │ ← tcf/decoder.go  (TCF 2.2)  │
     │  (生产)     │        │ ← usp/parser.go   (CCPA)     │
     │─────────────│        │ ← gvl/client.go   (GVL 缓存) │
     │memory_store │        └──────────────────────────────┘
     │  (测试/dev) │
     └─────────────┘
            │
     ┌──────▼──────────────────┐
     │   PostgreSQL            │
     │   consent_records       │
     │   idx: (user_id,        │
     │         created_at DESC)│
     └─────────────────────────┘

     ┌─────────────────────────┐
     │   metrics/metrics.go    │  → Prometheus :9101
     │   （全局指标注册）        │
     └─────────────────────────┘
```

### 各层职责说明

| 层 | 包 | 职责 | IO |
|----|----|------|----|
| 传输层 | `internal/api` | 路由、参数绑定、响应序列化、IP 提取 | HTTP |
| 编解码层 | `internal/tcf`, `internal/usp` | 纯位运算，无副作用 | 无 |
| 合规层 | `internal/policy` | 按优先级组合 TCF/USP/GVL 结果 | 无（依赖注入） |
| 数据层 | `internal/store` | consent_records CRUD | PostgreSQL / 内存 |
| 外部缓存层 | `internal/gvl` | GVL 拉取与本地缓存 | HTTP |
| 可观测层 | `internal/metrics` | Prometheus Counter/Histogram 注册 | Prometheus |

---

## 2. TCF 2.2 解码流程

TCF Consent String 是 Base64url 编码的二进制字符串，按 **BigEndian** 位序排列。

```
输入: Base64url Consent String
        │
        ▼
  Base64url 解码
  → []byte（原始位流）
        │
        ▼
┌───────────────────────────────────────────────────────┐
│  Header 解析（固定段，按位偏移读取）                    │
│                                                        │
│  bits  0- 5  : Version          (6 bits)              │
│  bits  6-35  : Created          (36 bits, 1/10s)      │
│  bits 36-65  : LastUpdated      (36 bits, 1/10s)      │
│  bits 66-71  : CmpId            (12 bits)             │  ← 实际偏移按规范
│  bits 78-83  : ConsentScreen    (6 bits)              │
│  bits 84-87  : ConsentLanguage  (12 bits, 2字符)      │
│  bits 96-106 : VendorListVersion(12 bits)             │
│  bits 108-117: PurposesConsent  (24 bits, BitField)   │
└───────────────────────────────┬───────────────────────┘
                                │
                                ▼
                  读取 IsRangeEncoding 标志位
                                │
               ┌────────────────┴────────────────┐
               │ false                           │ true
               ▼                                 ▼
        ┌─────────────┐                  ┌──────────────────┐
        │  BitField   │                  │  Range Encoding  │
        │  厂商解码   │                  │  厂商解码        │
        │─────────────│                  │──────────────────│
        │ MaxVendorId │                  │ NumEntries       │
        │ bits 依次   │                  │ 循环读取         │
        │ 对应厂商 ID │                  │ IsRange=false:   │
        │ 1=同意      │                  │   单个 VendorId  │
        └──────┬──────┘                  │ IsRange=true:    │
               │                         │   StartId~EndId  │
               │                         └────────┬─────────┘
               └────────────┬────────────────────┘
                            │
                            ▼
              ┌─────────────────────────────┐
              │  ConsentData struct         │
              │  {                          │
              │    Version int              │
              │    Created time.Time        │
              │    PurposesConsent []int    │
              │    VendorConsent   []int    │
              │    VendorListVersion int    │
              │    ...                      │
              │  }                          │
              └─────────────────────────────┘
```

### 位读取器设计

```
type bitReader struct {
    data   []byte
    offset int   // 当前位偏移（从 0 开始，BigEndian）
}

readBits(n int) uint64   // 读取 n 位，右对齐返回
readBool()    bool       // 读取 1 位
```

所有读取操作均不修改 `data`，通过偏移量推进，保证无副作用。

---

## 3. 合规检查决策树

`policy/matcher.go` 实现以下优先级决策：

```
输入: CheckRequest {
  user_id, vendor_id, purposes[]
  consent_string (TCF), us_privacy (CCPA)
}
        │
        ▼
┌───────────────────────────────────┐
│  Step 1: 解析 CCPA US Privacy     │
│  usp.Parse(us_privacy)            │
└───────────────┬───────────────────┘
                │
        OptOut == 'Y' ?
          │           │
         YES          NO
          │           │
          ▼           ▼
    ┌──────────┐  ┌───────────────────────────────────┐
    │ DENY     │  │  Step 2: 解码 TCF Consent String  │
    │ CCPA     │  │  tcf.Decode(consent_string)       │
    │ opt-out  │  └───────────────┬───────────────────┘
    └──────────┘                  │
                          Purpose 1 in
                         PurposesConsent?
                            │         │
                           NO        YES
                            │         │
                            ▼         ▼
                      ┌──────────┐  ┌──────────────────────────────┐
                      │ DENY     │  │  Step 3: 检查所需 Purposes   │
                      │ Purpose1 │  │  所有 purposes[] 均已同意?   │
                      │ missing  │  └──────────────┬───────────────┘
                      └──────────┘                 │
                                          NO               YES
                                           │                │
                                           ▼                ▼
                                    ┌──────────┐  ┌───────────────────────────┐
                                    │ DENY     │  │  Step 4: GVL 厂商检查     │
                                    │ Purpose  │  │  gvl.IsVendorAllowed(     │
                                    │ missing  │  │    vendor_id,             │
                                    └──────────┘  │    consent.VendorConsent) │
                                                  └──────────────┬────────────┘
                                                                 │
                                                      NO                 YES
                                                       │                  │
                                                       ▼                  ▼
                                                ┌──────────┐   ┌─────────────────────┐
                                                │ DENY     │   │  Step 5: 个性化判断 │
                                                │ vendor   │   │  Purpose 3 AND 4    │
                                                │ not      │   │  in PurposesConsent │
                                                │ allowed  │   └──────────┬──────────┘
                                                └──────────┘              │
                                                                 YES              NO
                                                                  │                │
                                                                  ▼                ▼
                                                        ┌───────────────┐ ┌───────────────┐
                                                        │ ALLOW         │ │ ALLOW         │
                                                        │ canPersonalize│ │ canPersonalize│
                                                        │ = true        │ │ = false       │
                                                        └───────────────┘ └───────────────┘
```

### 返回结构

```go
type CheckResult struct {
    Allowed        bool   // 是否允许投放
    Reason         string // 拒绝原因（allowed 时为 "consent granted"）
    CanPersonalize bool   // 是否允许个性化（Purpose 3 + 4 均已授权）
}
```

---

## 4. GVL 刷新流程时序图

```
服务启动                gvl/client.go              IAB GVL Server
    │                        │                           │
    │  NewClient()           │                           │
    │───────────────────────>│                           │
    │                        │                           │
    │  返回 client（缓存空）  │                           │
    │<───────────────────────│                           │
    │                        │                           │
    │  go client.Start()     │                           │
    │  （异步后台协程）        │                           │
    │───────────────────────>│                           │
    │                        │                           │
    │  主服务正常对外提供服务  │                           │
    │  （不等待 GVL 就绪）    │                           │
    │                        │                           │
    │                        │  GET /v2/vendor-list.json │
    │                        │──────────────────────────>│
    │                        │                           │
    │                        │  200 OK + VendorList JSON │
    │                        │<──────────────────────────│
    │                        │                           │
    │                        │  原子替换缓存              │
    │                        │  atomicStore(vendorList)  │
    │                        │                           │
    │  /v1/consent/check     │                           │
    │─────────────────────── >│                          │
    │                        │  读取缓存（无锁）          │
    │                        │  atomicLoad(vendorList)   │
    │  CheckResult           │                           │
    │<───────────────────────│                           │
    │                        │                           │
    │                        │  ── 24h 后 ──             │
    │                        │                           │
    │                        │  GET /v2/vendor-list.json │
    │                        │──────────────────────────>│
    │                        │                           │
    │                        │  200 OK（新版本）          │
    │                        │<──────────────────────────│
    │                        │                           │
    │                        │  原子替换缓存（新版本）     │
    │                        │                           │
    │                        │  ── 刷新失败时 ──          │
    │                        │                           │
    │                        │  GET /v2/vendor-list.json │
    │                        │──────────────────────────>│
    │                        │                           │
    │                        │  5xx / timeout            │
    │                        │<──────────────────────────│
    │                        │                           │
    │                        │  保留上一次缓存            │
    │                        │  记录 metrics 失败计数     │
    │                        │  下次定时重试              │
    │                        │                           │
```

### GVL 客户端并发安全

```
vendorList 使用 sync/atomic.Value 存储（原子读写）
读路径：零锁开销，高并发安全
写路径：仅后台单一 goroutine 写入，原子 Store
```

---

## 5. 数据库 Schema

```sql
CREATE TABLE consent_records (
    id             BIGSERIAL    PRIMARY KEY,
    user_id        TEXT         NOT NULL,
    consent_string TEXT,                       -- TCF 2.2 Base64url
    us_privacy     TEXT,                       -- CCPA "1YNN" 格式
    gdpr_applies   BOOLEAN,
    purposes       INT[],                      -- 已授权 Purpose ID 列表
    vendors        INT[],                      -- 已授权 Vendor ID 列表
    ip             TEXT,                       -- 优先取 X-Forwarded-For
    user_agent     TEXT,
    source         TEXT,                       -- sdk-web / sdk-ios / sdk-android / ctv
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 按用户查询最新记录的主要索引
CREATE INDEX idx_consent_user_created
    ON consent_records (user_id, created_at DESC);
```

典型查询：

```sql
-- 获取用户最新同意记录
SELECT * FROM consent_records
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 1;
```

---

## 6. Prometheus 指标

| 指标名 | 类型 | Labels | 说明 |
|--------|------|--------|------|
| `consent_check_total` | Counter | `result=allowed\|denied`, `reason` | 合规检查结果计数 |
| `consent_save_total` | Counter | `status=ok\|error` | 同意记录写入计数 |
| `consent_save_duration_seconds` | Histogram | — | 写入耗时分布 |
| `gvl_refresh_total` | Counter | `status=ok\|error` | GVL 刷新次数 |
| `gvl_vendor_count` | Gauge | — | 当前 GVL 厂商数量 |
| `http_request_duration_seconds` | Histogram | `method`, `path`, `status` | HTTP 请求耗时 |

指标端口与业务端口分离（`:9101` vs `:8089`），避免指标抓取影响业务延迟。
