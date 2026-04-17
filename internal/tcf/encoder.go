package tcf

import (
	"encoding/base64"
	"time"
)

// EncodeParams 是构建 TCF 2.2 consent string 所需参数。
type EncodeParams struct {
	CmpID           int
	CmpVersion      int
	ConsentScreen   int
	ConsentLanguage string // 2 字母，如 "EN"
	VendorListVer   int
	PurposesConsent []int // 已同意的 purpose IDs（1-24）
	VendorConsents  []int // 已同意的 vendor IDs
}

// Encode 将参数编码为 TCF 2.2 Core String（base64url）。
func Encode(p EncodeParams) string {
	now := time.Now().UTC()
	bw := &bitWriter{}

	bw.writeInt(2, 6)                                // Version = 2
	bw.writeInt64(toDeciseconds(now), 36)             // Created
	bw.writeInt64(toDeciseconds(now), 36)             // LastUpdated
	bw.writeInt(p.CmpID, 12)                         // CmpId
	bw.writeInt(p.CmpVersion, 12)                    // CmpVersion
	bw.writeInt(p.ConsentScreen, 6)                  // ConsentScreen
	lang := normLang(p.ConsentLanguage)
	bw.writeInt(int(lang[0]-'A'), 6)                 // ConsentLanguage[0]
	bw.writeInt(int(lang[1]-'A'), 6)                 // ConsentLanguage[1]
	bw.writeInt(p.VendorListVer, 12)                 // VendorListVersion
	bw.writeInt(4, 6)                                // TcfPolicyVersion = 4 (TCF 2.2)
	bw.writeBool(false)                              // IsServiceSpecific
	bw.writeBool(false)                              // UseNonStandardStacks
	bw.writeInt(0, 12)                               // SpecialFeatureOptIns (none)

	// PurposesConsent: bits 1-24
	purposes := toSet(p.PurposesConsent)
	for i := 1; i <= 24; i++ {
		bw.writeBool(purposes[i])
	}
	// PurposesLITransparency: 24 bits (all false)
	for i := 0; i < 24; i++ {
		bw.writeBool(false)
	}
	bw.writeBool(false) // PurposeOneTreatment
	bw.writeInt(int('E'-'A'), 6) // PublisherCC "EN"
	bw.writeInt(int('N'-'A'), 6)

	// VendorConsents: BitField mode
	vendors := toSet(p.VendorConsents)
	maxVendor := maxKey(vendors)
	if maxVendor < 0 {
		maxVendor = 0
	}
	bw.writeInt(maxVendor, 16) // MaxVendorId
	bw.writeBool(false)        // IsRangeEncoding = false (BitField)
	for i := 1; i <= maxVendor; i++ {
		bw.writeBool(vendors[i])
	}

	// VendorLegitimateInterests: empty
	bw.writeInt(0, 16) // MaxVendorId = 0
	bw.writeBool(false)

	return base64.RawURLEncoding.EncodeToString(bw.bytes())
}

func toDeciseconds(t time.Time) int64 {
	return t.UnixNano() / int64(100*time.Millisecond)
}

func normLang(lang string) string {
	if len(lang) >= 2 {
		b := []byte{lang[0], lang[1]}
		if b[0] >= 'a' {
			b[0] -= 32
		}
		if b[1] >= 'a' {
			b[1] -= 32
		}
		return string(b)
	}
	return "EN"
}

func toSet(ids []int) map[int]bool {
	s := make(map[int]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}

func maxKey(m map[int]bool) int {
	max := -1
	for k := range m {
		if k > max {
			max = k
		}
	}
	return max
}

// bitWriter 大端序位写入器。
type bitWriter struct {
	buf     []byte
	current byte
	bitPos  int // 当前字节中已写入的位数（0-7）
}

func (w *bitWriter) writeBool(v bool) {
	if v {
		w.current |= 1 << (7 - uint(w.bitPos))
	}
	w.bitPos++
	if w.bitPos == 8 {
		w.buf = append(w.buf, w.current)
		w.current = 0
		w.bitPos = 0
	}
}

func (w *bitWriter) writeInt(v, n int) {
	for i := n - 1; i >= 0; i-- {
		w.writeBool((v>>uint(i))&1 == 1)
	}
}

func (w *bitWriter) writeInt64(v int64, n int) {
	for i := n - 1; i >= 0; i-- {
		w.writeBool((v>>uint(i))&1 == 1)
	}
}

func (w *bitWriter) bytes() []byte {
	if w.bitPos > 0 {
		return append(w.buf, w.current)
	}
	return w.buf
}
