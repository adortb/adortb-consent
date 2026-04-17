// Package api 提供 adortb-consent 服务的 HTTP API 处理器。
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/adortb/adortb-consent/internal/gvl"
	"github.com/adortb/adortb-consent/internal/policy"
	"github.com/adortb/adortb-consent/internal/store"
	"github.com/adortb/adortb-consent/internal/tcf"
	"github.com/adortb/adortb-consent/internal/usp"
)

// Handler 汇总所有 consent API 路由。
type Handler struct {
	store  store.ConsentStore
	gvl    *gvl.Client
	logger *slog.Logger
}

// NewHandler 创建 API Handler。
func NewHandler(s store.ConsentStore, gvlClient *gvl.Client, logger *slog.Logger) *Handler {
	return &Handler{store: s, gvl: gvlClient, logger: logger}
}

// Register 将所有路由注册到 mux。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/consent", h.handleSaveConsent)
	mux.HandleFunc("GET /v1/consent/{user_id}", h.handleGetConsent)
	mux.HandleFunc("POST /v1/consent/decode", h.handleDecode)
	mux.HandleFunc("POST /v1/consent/check", h.handleCheck)
	mux.HandleFunc("GET /v1/vendors", h.handleVendors)
	mux.HandleFunc("GET /v1/gvl", h.handleGVL)
	mux.HandleFunc("GET /health", handleHealth)
}

// saveConsentRequest 是 POST /v1/consent 请求体。
type saveConsentRequest struct {
	UserID        string `json:"user_id"`
	ConsentString string `json:"consent_string"`
	USPrivacy     string `json:"us_privacy"`
	GDPRApplies   bool   `json:"gdpr_applies"`
	Purposes      []int  `json:"purposes"`
	Vendors       []int  `json:"vendors"`
	Source        string `json:"source"`
}

func (h *Handler) handleSaveConsent(w http.ResponseWriter, r *http.Request) {
	var req saveConsentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if strings.TrimSpace(req.UserID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id required"})
		return
	}

	record := &store.ConsentRecord{
		UserID:        req.UserID,
		ConsentString: req.ConsentString,
		USPrivacy:     req.USPrivacy,
		GDPRApplies:   req.GDPRApplies,
		Purposes:      req.Purposes,
		Vendors:       req.Vendors,
		IP:            extractIP(r),
		UserAgent:     r.Header.Get("User-Agent"),
		Source:        req.Source,
	}

	if err := h.store.Save(r.Context(), record); err != nil {
		h.logger.ErrorContext(r.Context(), "save consent failed", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         record.ID,
		"created_at": record.CreatedAt,
	})
}

func (h *Handler) handleGetConsent(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id required"})
		return
	}

	rec, err := h.store.GetLatest(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if rec == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// decodeRequest 是 POST /v1/consent/decode 请求体。
type decodeRequest struct {
	ConsentString string `json:"consent_string"`
}

func (h *Handler) handleDecode(w http.ResponseWriter, r *http.Request) {
	var req decodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	core, err := tcf.Decode(req.ConsentString)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, core)
}

// checkRequest 是 POST /v1/consent/check 请求体。
type checkRequest struct {
	ConsentString string      `json:"consent_string"`
	USPrivacy     string      `json:"us_privacy"`
	GDPRApplies   bool        `json:"gdpr_applies"`
	VendorID      int         `json:"vendor_id"`
	Purpose       tcf.Purpose `json:"purpose"`
}

func (h *Handler) handleCheck(w http.ResponseWriter, r *http.Request) {
	var req checkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	result := policy.Check(&policy.CheckRequest{
		ConsentString: req.ConsentString,
		USPrivacy:     req.USPrivacy,
		GDPRApplies:   req.GDPRApplies,
		VendorID:      req.VendorID,
		Purpose:       req.Purpose,
	})

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleVendors(w http.ResponseWriter, r *http.Request) {
	gvlData := h.gvl.Get()
	if gvlData == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GVL not loaded yet"})
		return
	}
	writeJSON(w, http.StatusOK, gvlData.Vendors)
}

func (h *Handler) handleGVL(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version": h.gvl.Version(),
	})
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleUSPDecode 解析 CCPA USP string（内部工具接口）。
func handleUSPDecode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		USPrivacy string `json:"us_privacy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	parsed, err := usp.Parse(req.USPrivacy)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, parsed)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		return addr[:idx]
	}
	return addr
}
