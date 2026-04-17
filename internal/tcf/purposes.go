// Package tcf 实现 IAB TCF 2.2 Transparency & Consent Framework 核心逻辑。
package tcf

// Purpose 代表 IAB TCF 2.2 定义的合规目的（Purpose）。
type Purpose int

const (
	// PurposeStoreAndAccess (1) 存储和访问设备信息
	PurposeStoreAndAccess Purpose = 1
	// PurposeBasicAds (2) 基础广告投放（无个人化）
	PurposeBasicAds Purpose = 2
	// PurposePersonalizedAds (3) 创建个性化广告画像
	PurposePersonalizedAds Purpose = 3
	// PurposeSelectPersonalizedAds (4) 选择个性化广告
	PurposeSelectPersonalizedAds Purpose = 4
	// PurposePersonalizedContent (5) 创建个性化内容画像
	PurposePersonalizedContent Purpose = 5
	// PurposeSelectPersonalizedContent (6) 选择个性化内容
	PurposeSelectPersonalizedContent Purpose = 6
	// PurposeMeasureAdPerformance (7) 衡量广告效果
	PurposeMeasureAdPerformance Purpose = 7
	// PurposeMeasureContentPerformance (8) 衡量内容效果
	PurposeMeasureContentPerformance Purpose = 8
	// PurposeResearchAudience (9) 受众研究与开发
	PurposeResearchAudience Purpose = 9
	// PurposeDevelopProducts (10) 开发和改进产品
	PurposeDevelopProducts Purpose = 10
)

// PurposeName 返回 purpose 的可读名称。
func PurposeName(p Purpose) string {
	names := map[Purpose]string{
		PurposeStoreAndAccess:            "Store and/or access information on a device",
		PurposeBasicAds:                  "Use limited data to select advertising",
		PurposePersonalizedAds:           "Create profiles for personalised advertising",
		PurposeSelectPersonalizedAds:     "Use profiles to select personalised advertising",
		PurposePersonalizedContent:       "Create profiles to personalise content",
		PurposeSelectPersonalizedContent: "Use profiles to select personalised content",
		PurposeMeasureAdPerformance:      "Measure advertising performance",
		PurposeMeasureContentPerformance: "Measure content performance",
		PurposeResearchAudience:          "Understand audiences through statistics or combinations of data",
		PurposeDevelopProducts:           "Develop and improve services",
	}
	if name, ok := names[p]; ok {
		return name
	}
	return "Unknown purpose"
}
