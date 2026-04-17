// Package usp 实现 IAB CCPA US Privacy String 解析。
// 格式: "1YNN" = version(1) + explicit notice(Y/N) + opt-out of sale(Y/N) + LSPA(Y/N)
package usp

import (
	"errors"
	"fmt"
)

// USPrivacy 表示解析后的 CCPA US Privacy String。
type USPrivacy struct {
	Version         int
	ExplicitNotice  Flag // 是否提供了明确的 CCPA 告知
	OptOutSale      Flag // 用户是否选择退出数据销售
	LSPAApplied     Flag // LSPA 协议是否适用
}

// Flag 是三值标志：Yes / No / NotApplicable。
type Flag byte

const (
	FlagNotApplicable Flag = '-'
	FlagYes           Flag = 'Y'
	FlagNo            Flag = 'N'
)

// Parse 解析 CCPA US Privacy String，如 "1YNN"。
func Parse(s string) (*USPrivacy, error) {
	if len(s) != 4 {
		return nil, fmt.Errorf("usp: invalid length %d, expected 4", len(s))
	}

	version := int(s[0] - '0')
	if version != 1 {
		return nil, fmt.Errorf("usp: unsupported version %d", version)
	}

	explicit, err := parseFlag(s[1])
	if err != nil {
		return nil, fmt.Errorf("usp: explicit notice: %w", err)
	}
	optOut, err := parseFlag(s[2])
	if err != nil {
		return nil, fmt.Errorf("usp: opt-out sale: %w", err)
	}
	lspa, err := parseFlag(s[3])
	if err != nil {
		return nil, fmt.Errorf("usp: LSPA: %w", err)
	}

	return &USPrivacy{
		Version:        version,
		ExplicitNotice: explicit,
		OptOutSale:     optOut,
		LSPAApplied:    lspa,
	}, nil
}

// IsOptedOut 返回用户是否已选择退出数据销售。
func (u *USPrivacy) IsOptedOut() bool {
	return u.OptOutSale == FlagYes
}

// String 重新编码为 USP string 格式。
func (u *USPrivacy) String() string {
	return fmt.Sprintf("%d%c%c%c",
		u.Version,
		u.ExplicitNotice,
		u.OptOutSale,
		u.LSPAApplied,
	)
}

func parseFlag(b byte) (Flag, error) {
	switch Flag(b) {
	case FlagYes, FlagNo, FlagNotApplicable:
		return Flag(b), nil
	default:
		return 0, errors.New("invalid flag value, expected Y/N/-")
	}
}
