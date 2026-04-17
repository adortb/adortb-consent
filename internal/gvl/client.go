// Package gvl 实现 IAB Global Vendor List (GVL) 的定期拉取与缓存管理。
package gvl

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	gvlURL          = "https://vendor-list.consensu.org/v3/vendor-list.json"
	defaultRefresh  = 24 * time.Hour
	requestTimeout  = 15 * time.Second
)

// VendorInfo 表示 GVL 中单个厂商的信息。
type VendorInfo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Purposes []int  `json:"purposes"`
}

// GVLResponse 是 IAB GVL JSON 的顶层结构（简化）。
type GVLResponse struct {
	GVLSpecificationVersion int                   `json:"gvlSpecificationVersion"`
	VendorListVersion       int                   `json:"vendorListVersion"`
	LastUpdated             string                `json:"lastUpdated"`
	Vendors                 map[string]VendorInfo `json:"vendors"`
}

// Client 定期拉取并缓存 GVL 数据。
type Client struct {
	mu          sync.RWMutex
	cache       *GVLResponse
	lastFetched time.Time
	refreshTTL  time.Duration
	httpClient  *http.Client
	logger      *slog.Logger
	stopCh      chan struct{}
}

// NewClient 创建 GVL Client 实例。
func NewClient(logger *slog.Logger, refreshTTL time.Duration) *Client {
	if refreshTTL <= 0 {
		refreshTTL = defaultRefresh
	}
	return &Client{
		refreshTTL: refreshTTL,
		httpClient: &http.Client{Timeout: requestTimeout},
		logger:     logger,
		stopCh:     make(chan struct{}),
	}
}

// Start 启动后台定期刷新 goroutine（立即拉取一次）。
func (c *Client) Start(ctx context.Context) {
	// 首次拉取（忽略错误，不阻塞启动）
	if err := c.Refresh(ctx); err != nil {
		c.logger.Warn("GVL initial fetch failed", slog.String("error", err.Error()))
	}

	go func() {
		ticker := time.NewTicker(c.refreshTTL)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := c.Refresh(ctx); err != nil {
					c.logger.Warn("GVL refresh failed", slog.String("error", err.Error()))
				}
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			}
		}
	}()
}

// Stop 停止后台刷新。
func (c *Client) Stop() {
	select {
	case c.stopCh <- struct{}{}:
	default:
	}
}

// Refresh 立即拉取最新 GVL 并更新缓存。
func (c *Client) Refresh(ctx context.Context) error {
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, gvlURL, nil)
	if err != nil {
		return fmt.Errorf("gvl: create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gvl: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gvl: unexpected status %d", resp.StatusCode)
	}

	var gvl GVLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gvl); err != nil {
		return fmt.Errorf("gvl: decode: %w", err)
	}

	c.mu.Lock()
	c.cache = &gvl
	c.lastFetched = time.Now()
	c.mu.Unlock()

	c.logger.Info("GVL refreshed",
		slog.Int("version", gvl.VendorListVersion),
		slog.Int("vendor_count", len(gvl.Vendors)),
	)
	return nil
}

// Get 返回当前缓存的 GVL（可能为 nil，若尚未成功拉取）。
func (c *Client) Get() *GVLResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache
}

// Version 返回当前缓存的 GVL 版本号（0 表示未加载）。
func (c *Client) Version() int {
	gvl := c.Get()
	if gvl == nil {
		return 0
	}
	return gvl.VendorListVersion
}
