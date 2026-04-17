package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"

	"github.com/adortb/adortb-consent/internal/gvl"
	"github.com/adortb/adortb-consent/internal/store"
	"github.com/adortb/adortb-consent/internal/tcf"
)

func newTestHandler() *Handler {
	return NewHandler(
		store.NewMemoryStore(),
		gvl.NewClient(slog.Default(), 0),
		slog.Default(),
	)
}

func TestHandleSaveConsent(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	body, _ := json.Marshal(map[string]interface{}{
		"user_id":       "user123",
		"gdpr_applies":  true,
		"consent_string": tcf.Encode(tcf.EncodeParams{
			CmpID: 1, ConsentLanguage: "EN", PurposesConsent: []int{1}, VendorConsents: []int{1},
		}),
		"purposes": []int{1},
		"vendors":  []int{1},
		"source":   "sdk_web",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/consent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetConsent_NotFound(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/consent/nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleGetConsent_Found(t *testing.T) {
	ms := store.NewMemoryStore()
	_ = ms.Save(context.Background(), &store.ConsentRecord{
		UserID: "user456",
		Source: "sdk_web",
	})

	h := NewHandler(ms, gvl.NewClient(slog.Default(), 0), slog.Default())
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/consent/user456", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDecode(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	encoded := tcf.Encode(tcf.EncodeParams{
		CmpID: 5, ConsentLanguage: "DE", PurposesConsent: []int{1, 2}, VendorConsents: []int{10},
	})
	body, _ := json.Marshal(map[string]string{"consent_string": encoded})

	req := httptest.NewRequest(http.MethodPost, "/v1/consent/decode", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCheck_Allowed(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	encoded := tcf.Encode(tcf.EncodeParams{
		CmpID: 1, ConsentLanguage: "EN", PurposesConsent: []int{1, 2}, VendorConsents: []int{1},
	})
	body, _ := json.Marshal(map[string]interface{}{
		"consent_string": encoded,
		"gdpr_applies":   true,
		"vendor_id":      1,
		"purpose":        1,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/consent/check", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var result map[string]interface{}
	_ = json.NewDecoder(rr.Body).Decode(&result)
	if result["Allowed"] != true {
		t.Errorf("expected Allowed=true, got %v", result["Allowed"])
	}
}

func TestHandleHealth(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
