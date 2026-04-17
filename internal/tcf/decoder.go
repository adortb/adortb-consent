package tcf

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// CoreConsent 包含 TCF 2.2 Core String 解析结果。
type CoreConsent struct {
	Version             int
	Created             time.Time
	LastUpdated         time.Time
	CmpID               int
	CmpVersion          int
	ConsentScreen       int
	ConsentLanguage     string
	VendorListVersion   int
	TcfPolicyVersion    int
	IsServiceSpecific   bool
	PurposesConsent     [25]bool // 索引 1-24 有效（Purpose 1~24）
	PurposesLI          [25]bool // LegitimateInterests
	VendorConsents      map[int]bool
}

// decisecondEpoch 是 TCF 时间戳的基准（UTC 1970-01-01）。
const decisecondDivisor = int64(100 * time.Millisecond)

// Decode 将 TCF 2.2 consent string 解析为 CoreConsent。
// 支持多段 consent string（取第一段 core string）。
func Decode(consentString string) (*CoreConsent, error) {
	if consentString == "" {
		return nil, errors.New("tcf: empty consent string")
	}
	// 多段 consent string 以 "." 分隔，取第一段（core string）
	parts := strings.SplitN(consentString, ".", 2)
	core := parts[0]

	raw, err := base64.RawURLEncoding.DecodeString(core)
	if err != nil {
		return nil, fmt.Errorf("tcf: base64url decode failed: %w", err)
	}

	b := &bitReader{data: raw}

	version := b.readInt(6)
	if version != 2 {
		return nil, fmt.Errorf("tcf: unsupported version %d (only TCF 2.x supported)", version)
	}

	createdDs := b.readInt64(36)
	lastUpdatedDs := b.readInt64(36)
	cmpID := b.readInt(12)
	cmpVersion := b.readInt(12)
	consentScreen := b.readInt(6)
	lang := decodeLang(b.readInt(6), b.readInt(6))
	vendorListVersion := b.readInt(12)
	tcfPolicyVersion := b.readInt(6)
	isServiceSpecific := b.readBool()
	_ = b.readBool() // useNonStandardStacks

	// SpecialFeatureOptIns: 12 bits（跳过）
	b.skip(12)

	// PurposesConsent: 24 bits
	var purposesConsent [25]bool
	for i := 1; i <= 24; i++ {
		purposesConsent[i] = b.readBool()
	}

	// PurposesLITransparency: 24 bits
	var purposesLI [25]bool
	for i := 1; i <= 24; i++ {
		purposesLI[i] = b.readBool()
	}

	_ = b.readBool() // purposeOneTreatment
	b.skip(12)       // publisherCC

	if b.err != nil {
		return nil, fmt.Errorf("tcf: bit read error: %w", b.err)
	}

	// VendorConsents
	vendorConsents, err := readVendorBitfield(b)
	if err != nil {
		return nil, fmt.Errorf("tcf: vendor consents: %w", err)
	}

	return &CoreConsent{
		Version:           version,
		Created:           deciToTime(createdDs),
		LastUpdated:       deciToTime(lastUpdatedDs),
		CmpID:             cmpID,
		CmpVersion:        cmpVersion,
		ConsentScreen:     consentScreen,
		ConsentLanguage:   lang,
		VendorListVersion: vendorListVersion,
		TcfPolicyVersion:  tcfPolicyVersion,
		IsServiceSpecific: isServiceSpecific,
		PurposesConsent:   purposesConsent,
		PurposesLI:        purposesLI,
		VendorConsents:    vendorConsents,
	}, nil
}

// HasPurpose 检查是否有指定 purpose 的 consent。
func (c *CoreConsent) HasPurpose(p Purpose) bool {
	idx := int(p)
	if idx < 1 || idx > 24 {
		return false
	}
	return c.PurposesConsent[idx]
}

// HasVendor 检查是否有指定 vendor 的 consent。
func (c *CoreConsent) HasVendor(vendorID int) bool {
	return c.VendorConsents[vendorID]
}

// readVendorBitfield 读取 VendorConsents 段（位字段或范围编码两种模式）。
func readVendorBitfield(b *bitReader) (map[int]bool, error) {
	maxVendorID := b.readInt(16)
	isRange := b.readBool()
	if b.err != nil {
		return nil, b.err
	}

	vendors := make(map[int]bool, maxVendorID)

	if !isRange {
		// BitField 模式：每个 vendor 一个 bit
		for i := 1; i <= maxVendorID; i++ {
			if b.readBool() {
				vendors[i] = true
			}
		}
	} else {
		// Range 模式：numEntries + 每条 isRange+startID+(endID)
		numEntries := b.readInt(12)
		for i := 0; i < numEntries; i++ {
			isARange := b.readBool()
			startID := b.readInt(16)
			if isARange {
				endID := b.readInt(16)
				for v := startID; v <= endID; v++ {
					vendors[v] = true
				}
			} else {
				vendors[startID] = true
			}
		}
	}

	if b.err != nil {
		return nil, b.err
	}
	return vendors, nil
}

// deciToTime 将 TCF 决秒数转换为 time.Time。
func deciToTime(deciseconds int64) time.Time {
	return time.Unix(0, deciseconds*int64(100*time.Millisecond)).UTC()
}

// decodeLang 将两个 6-bit 字符编码还原为 2 字母语言代码。
func decodeLang(b1, b2 int) string {
	return string([]byte{byte('A' + b1), byte('A' + b2)})
}

// bitReader 是大端序位读取器（不可变 data，pos 游标）。
type bitReader struct {
	data []byte
	pos  int
	err  error
}

func (r *bitReader) readBool() bool {
	if r.err != nil {
		return false
	}
	if r.pos >= len(r.data)*8 {
		r.err = errors.New("tcf: read past end of data")
		return false
	}
	byteIdx := r.pos / 8
	bitIdx := 7 - (r.pos % 8)
	r.pos++
	return (r.data[byteIdx]>>uint(bitIdx))&1 == 1
}

func (r *bitReader) readInt(n int) int {
	val := 0
	for i := 0; i < n; i++ {
		val = (val << 1)
		if r.readBool() {
			val |= 1
		}
	}
	return val
}

func (r *bitReader) readInt64(n int) int64 {
	val := int64(0)
	for i := 0; i < n; i++ {
		val = (val << 1)
		if r.readBool() {
			val |= 1
		}
	}
	return val
}

func (r *bitReader) skip(n int) {
	r.pos += n
}
