-- Consent 审计记录表
CREATE TABLE IF NOT EXISTS consent_records (
    id          BIGSERIAL PRIMARY KEY,
    user_id     VARCHAR(128)  NOT NULL,
    consent_string TEXT        NOT NULL DEFAULT '',
    us_privacy  VARCHAR(10)   NOT NULL DEFAULT '',
    gdpr_applies BOOLEAN      NOT NULL DEFAULT FALSE,
    purposes    INT[]         NOT NULL DEFAULT '{}',
    vendors     INT[]         NOT NULL DEFAULT '{}',
    ip          VARCHAR(64)   NOT NULL DEFAULT '',
    user_agent  TEXT          NOT NULL DEFAULT '',
    source      VARCHAR(30)   NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_consent_user
    ON consent_records(user_id, created_at DESC);

-- 厂商信息缓存表（从 GVL 同步）
CREATE TABLE IF NOT EXISTS consent_vendors (
    vendor_id   INT          PRIMARY KEY,
    name        VARCHAR(128) NOT NULL DEFAULT '',
    purposes    INT[]        NOT NULL DEFAULT '{}',
    legal_basis VARCHAR(30)  NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
