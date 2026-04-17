package store

import (
	"context"
	"sync"
)

// MemoryStore 是用于测试的内存 ConsentStore 实现。
type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]*ConsentRecord
	nextID  int64
}

// NewMemoryStore 创建内存 store 实例。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records: make(map[string]*ConsentRecord),
		nextID:  1,
	}
}

func (m *MemoryStore) Save(_ context.Context, r *ConsentRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.ID = m.nextID
	m.nextID++
	cp := *r
	m.records[r.UserID] = &cp
	return nil
}

func (m *MemoryStore) GetLatest(_ context.Context, userID string) (*ConsentRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.records[userID]
	if !ok {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}
