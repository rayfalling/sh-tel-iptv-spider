package utils

import (
	"regexp"
	"strings"
)

const (
	macV1Regex  = "^[a-fA-F0-9]{2}(:[a-fA-F0-9]{2}){5}$"
	macV2Regex  = "^([A-Fa-f0-9]{2}[-,:]){5}[A-Fa-f0-9]{2}$"
	ipv4Regex   = `^(\b25[0-5]|\b2[0-4][0-9]|\b[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}$`
	userIdRegex = `^[0-9]{8}@etv[0-9]$`
	snRegex     = `^[0-9A-Za-z]{24}$`
)

// 机顶盒标签上常见的 MAC 区间写法，例如 9C:71:3A:6C:F6:F4-F5
var macRangeRegex = regexp.MustCompile(`^([0-9A-Fa-f]{2}(?::[0-9A-Fa-f]{2}){5})-([0-9A-Fa-f]{2})$`)

// NormalizeMac 去掉机顶盒标签上的 MAC 区间后缀：
//
//	9C:71:3A:6C:F6:F4-F5  →  9C:71:3A:6C:F6:F4
//
// 其余输入原样返回（不改大小写，避免影响已在正常使用的配置）。
// 第二个返回值为 true 表示发生了区间折叠，调用方可据此打印提示。
func NormalizeMac(mac string) (string, bool) {
	s := strings.TrimSpace(mac)
	if m := macRangeRegex.FindStringSubmatch(s); m != nil {
		return m[1], true
	}
	return s, false
}

// CheckMacAddressV1 检验Mac地址
func CheckMacAddressV1(mac string) bool {
	m, err := regexp.MatchString(macV1Regex, mac)
	if err != nil {
		return false
	}
	return m
}

// CheckIPv4Address 检验Mac地址
func CheckIPv4Address(ip string) bool {
	m, err := regexp.MatchString(ipv4Regex, ip)
	if err != nil {
		return false
	}
	return m
}

// CheckUserID 检验Mac地址
func CheckUserID(id string) bool {
	m, err := regexp.MatchString(userIdRegex, id)
	if err != nil {
		return false
	}
	return m
}

// CheckSNCode 简单检查SN码格式
func CheckSNCode(sn string) bool {
	m, err := regexp.MatchString(snRegex, sn)
	if err != nil {
		return false
	}
	return m
}
