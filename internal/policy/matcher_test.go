package policy

import (
	"testing"

	"github.com/adortb/adortb-consent/internal/tcf"
)

func TestCheck_NoGDPR(t *testing.T) {
	result := Check(&CheckRequest{GDPRApplies: false})
	if !result.Allowed {
		t.Error("expected allowed when GDPR does not apply")
	}
	if !result.CanPersonalize {
		t.Error("expected personalization allowed when no GDPR")
	}
}

func TestCheck_GDPRNoConsent(t *testing.T) {
	result := Check(&CheckRequest{GDPRApplies: true, ConsentString: ""})
	if result.Allowed {
		t.Error("expected denied when GDPR applies with no consent string")
	}
}

func TestCheck_GDPRWithConsent_Purpose1(t *testing.T) {
	// 只有 Purpose 1 同意
	encoded := tcf.Encode(tcf.EncodeParams{
		CmpID:           1,
		ConsentLanguage: "EN",
		PurposesConsent: []int{1},
		VendorConsents:  []int{10},
	})

	result := Check(&CheckRequest{
		GDPRApplies:   true,
		ConsentString: encoded,
		VendorID:      10,
		Purpose:       tcf.PurposeStoreAndAccess,
	})
	if !result.Allowed {
		t.Errorf("expected allowed, reason: %s", result.Reason)
	}
	// 没有 Purpose 3/4，不应允许个性化
	if result.CanPersonalize {
		t.Error("expected no personalization without Purpose 3+4")
	}
}

func TestCheck_GDPRMissingPurpose1(t *testing.T) {
	// 没有 Purpose 1
	encoded := tcf.Encode(tcf.EncodeParams{
		CmpID:           1,
		ConsentLanguage: "EN",
		PurposesConsent: []int{2, 3, 4},
		VendorConsents:  []int{1},
	})

	result := Check(&CheckRequest{
		GDPRApplies:   true,
		ConsentString: encoded,
	})
	if result.Allowed {
		t.Error("expected denied when Purpose 1 is missing")
	}
}

func TestCheck_CCPAOptOut(t *testing.T) {
	result := Check(&CheckRequest{
		USPrivacy: "1YYN",
	})
	if !result.Allowed {
		t.Error("expected allowed even with CCPA opt-out (contextual ads ok)")
	}
	if result.CanPersonalize {
		t.Error("expected no personalization with CCPA opt-out")
	}
}

func TestCheck_CCPANoOptOut(t *testing.T) {
	result := Check(&CheckRequest{
		USPrivacy: "1YNN",
	})
	if !result.Allowed {
		t.Error("expected allowed when no CCPA opt-out")
	}
}
