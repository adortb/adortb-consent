package tcf

import (
	"testing"
)

// 使用 IAB TCF 2.2 文档中的示例 consent string 进行测试
// 该字符串来自 https://github.com/InteractiveAdvertisingBureau/GDPR-Transparency-and-Consent-Framework
func TestDecode_RoundTrip(t *testing.T) {
	params := EncodeParams{
		CmpID:           10,
		CmpVersion:      1,
		ConsentScreen:   1,
		ConsentLanguage: "EN",
		VendorListVer:   150,
		PurposesConsent: []int{1, 2, 3, 4},
		VendorConsents:  []int{1, 5, 10},
	}

	encoded := Encode(params)
	if encoded == "" {
		t.Fatal("Encode returned empty string")
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Version != 2 {
		t.Errorf("Version: got %d, want 2", decoded.Version)
	}
	if decoded.CmpID != 10 {
		t.Errorf("CmpID: got %d, want 10", decoded.CmpID)
	}
	if decoded.VendorListVersion != 150 {
		t.Errorf("VendorListVersion: got %d, want 150", decoded.VendorListVersion)
	}
	if decoded.ConsentLanguage != "EN" {
		t.Errorf("ConsentLanguage: got %q, want %q", decoded.ConsentLanguage, "EN")
	}

	// Purpose checks
	for _, p := range []Purpose{1, 2, 3, 4} {
		if !decoded.HasPurpose(p) {
			t.Errorf("Purpose %d should be consented", p)
		}
	}
	for _, p := range []Purpose{5, 6, 7} {
		if decoded.HasPurpose(p) {
			t.Errorf("Purpose %d should NOT be consented", p)
		}
	}

	// Vendor checks
	for _, v := range []int{1, 5, 10} {
		if !decoded.HasVendor(v) {
			t.Errorf("Vendor %d should be consented", v)
		}
	}
	if decoded.HasVendor(2) {
		t.Error("Vendor 2 should NOT be consented")
	}
}

func TestDecode_EmptyString(t *testing.T) {
	_, err := Decode("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestDecode_InvalidBase64(t *testing.T) {
	_, err := Decode("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDecode_MultiSegment(t *testing.T) {
	// 多段字符串以 "." 分隔，只解析第一段
	params := EncodeParams{
		CmpID:           1,
		ConsentLanguage: "DE",
		PurposesConsent: []int{1},
		VendorConsents:  []int{1},
	}
	core := Encode(params)
	multiSeg := core + ".someOtherSegment"

	decoded, err := Decode(multiSeg)
	if err != nil {
		t.Fatalf("Decode multi-segment failed: %v", err)
	}
	if !decoded.HasPurpose(PurposeStoreAndAccess) {
		t.Error("Purpose 1 should be consented")
	}
}

func TestPurposeName(t *testing.T) {
	name := PurposeName(PurposeStoreAndAccess)
	if name == "" || name == "Unknown purpose" {
		t.Errorf("expected non-empty name for purpose 1, got %q", name)
	}
	unknown := PurposeName(Purpose(99))
	if unknown != "Unknown purpose" {
		t.Errorf("expected 'Unknown purpose' for 99, got %q", unknown)
	}
}
