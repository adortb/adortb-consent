// Package store 提供用户 consent 审计记录的持久化存储。
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// ConsentRecord 是一条用户同意记录。
type ConsentRecord struct {
	ID            int64
	UserID        string
	ConsentString string
	USPrivacy     string
	GDPRApplies   bool
	Purposes      []int
	Vendors       []int
	IP            string
	UserAgent     string
	Source        string
	CreatedAt     time.Time
}

// ConsentStore 定义存储接口，便于测试时 mock。
type ConsentStore interface {
	Save(ctx context.Context, r *ConsentRecord) error
	GetLatest(ctx context.Context, userID string) (*ConsentRecord, error)
}

// PGStore 使用 PostgreSQL 存储 consent 审计记录。
type PGStore struct {
	db *sql.DB
}

// NewPGStore 创建 PGStore，dsn 为 PostgreSQL 连接字符串。
func NewPGStore(dsn string) (*PGStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open db: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &PGStore{db: db}, nil
}

// Ping 检查数据库连通性。
func (s *PGStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Close 关闭数据库连接。
func (s *PGStore) Close() error {
	return s.db.Close()
}

// Save 将一条 consent 记录插入数据库。
func (s *PGStore) Save(ctx context.Context, r *ConsentRecord) error {
	const q = `
		INSERT INTO consent_records
			(user_id, consent_string, us_privacy, gdpr_applies, purposes, vendors, ip, user_agent, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`

	row := s.db.QueryRowContext(ctx, q,
		r.UserID,
		r.ConsentString,
		r.USPrivacy,
		r.GDPRApplies,
		intSliceToArray(r.Purposes),
		intSliceToArray(r.Vendors),
		r.IP,
		r.UserAgent,
		r.Source,
	)
	return row.Scan(&r.ID, &r.CreatedAt)
}

// GetLatest 返回指定用户最近的 consent 记录。
func (s *PGStore) GetLatest(ctx context.Context, userID string) (*ConsentRecord, error) {
	const q = `
		SELECT id, user_id, consent_string, us_privacy, gdpr_applies, ip, user_agent, source, created_at
		FROM consent_records
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	row := s.db.QueryRowContext(ctx, q, userID)
	r := &ConsentRecord{}
	err := row.Scan(
		&r.ID,
		&r.UserID,
		&r.ConsentString,
		&r.USPrivacy,
		&r.GDPRApplies,
		&r.IP,
		&r.UserAgent,
		&r.Source,
		&r.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get latest: %w", err)
	}
	return r, nil
}

// intSliceToArray 将 Go int slice 转换为 PostgreSQL array 字符串，如 "{1,2,3}"。
func intSliceToArray(ids []int) string {
	if len(ids) == 0 {
		return "{}"
	}
	result := "{"
	for i, id := range ids {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%d", id)
	}
	return result + "}"
}
