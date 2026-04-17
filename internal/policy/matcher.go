// Package policy 基于 consent 数据判断竞价参与资格和数据使用权限。
package policy

import (
	"github.com/adortb/adortb-consent/internal/tcf"
	"github.com/adortb/adortb-consent/internal/usp"
)

// CheckRequest 是合规检查的输入。
type CheckRequest struct {
	ConsentString string // TCF 2.2 consent string
	USPrivacy     string // CCPA USP string，如 "1YNN"
	GDPRApplies   bool
	VendorID      int    // 要检查的 vendor
	Purpose       tcf.Purpose
}

// CheckResult 是合规检查结果。
type CheckResult struct {
	Allowed        bool
	Reason         string
	CanPersonalize bool // 是否允许个性化投放
}

// Check 根据 consent string 和 USP string 判断是否允许指定操作。
func Check(req *CheckRequest) *CheckResult {
	// CCPA 优先检查：若用户已选择退出数据销售，禁止个性化
	if req.USPrivacy != "" {
		uspData, err := usp.Parse(req.USPrivacy)
		if err == nil && uspData.IsOptedOut() {
			return &CheckResult{
				Allowed:        true, // 仍可展示广告，但不能个性化
				Reason:         "CCPA opt-out: contextual-only allowed",
				CanPersonalize: false,
			}
		}
	}

	// 若不适用 GDPR，默认允许
	if !req.GDPRApplies {
		return &CheckResult{Allowed: true, CanPersonalize: true}
	}

	// GDPR 适用：必须有有效 consent string
	if req.ConsentString == "" {
		return &CheckResult{
			Allowed: false,
			Reason:  "GDPR applies but no consent string provided",
		}
	}

	consent, err := tcf.Decode(req.ConsentString)
	if err != nil {
		return &CheckResult{
			Allowed: false,
			Reason:  "GDPR: invalid consent string: " + err.Error(),
		}
	}

	// Purpose 1（存储和访问设备信息）是基础要求
	if !consent.HasPurpose(tcf.PurposeStoreAndAccess) {
		return &CheckResult{
			Allowed: false,
			Reason:  "GDPR: Purpose 1 (store/access device info) not consented",
		}
	}

	// 检查目标 purpose
	if req.Purpose > 0 && !consent.HasPurpose(req.Purpose) {
		return &CheckResult{
			Allowed: false,
			Reason:  "GDPR: required purpose not consented",
		}
	}

	// 检查 vendor consent
	if req.VendorID > 0 && !consent.HasVendor(req.VendorID) {
		return &CheckResult{
			Allowed: false,
			Reason:  "GDPR: vendor not consented",
		}
	}

	canPersonalize := consent.HasPurpose(tcf.PurposePersonalizedAds) &&
		consent.HasPurpose(tcf.PurposeSelectPersonalizedAds)

	return &CheckResult{
		Allowed:        true,
		CanPersonalize: canPersonalize,
	}
}
