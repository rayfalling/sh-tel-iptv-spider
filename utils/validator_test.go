package utils

import "testing"

func TestCheckMacAddressV1(t *testing.T) {
	data := map[string]bool{
		"01:02:03:04:ab:cd":    true,
		"definitely:not:a:mac": false,
		"01-02-03-04-ab-cd":    false,
	}

	for s, b := range data {
		r := CheckMacAddressV1(s)
		if r != b {
			t.Fatalf("错误: 数据 -- %s, 期待结果 %t, 计算结果 %t", s, b, r)
		}
	}
}

func TestCheckIPv4Address(t *testing.T) {
	data := map[string]bool{
		"127.0.0.1":         true,
		"0.0.0.0":           true,
		"255.255.255.255.1": false,
		"256.256.256.256":   false,
		"999.999.999.999":   false,
		"1.2.3":             false,
		"1.2.3.4":           true,
	}

	for s, b := range data {
		r := CheckIPv4Address(s)
		if r != b {
			t.Fatalf("错误: 数据 -- %s, 期待结果 %t, 计算结果 %t", s, b, r)
		}
	}
}

func TestCheckUserID(t *testing.T) {
	data := map[string]bool{
		"123456737AE8@etv1": false,
		"80560422@etv1":     true,
		"12345678@etv1":     true,
		"12345678@etv11":    false,
	}

	for s, b := range data {
		r := CheckUserID(s)
		if r != b {
			t.Fatalf("错误: 数据 -- %s, 期待结果 %t, 计算结果 %t", s, b, r)
		}
	}
}

// TestCheckSNCode 校验 SN 的格式约定：恰好 24 位字母数字。
//
// 注意：SN 的前 4 位是机型/批次（仓库文档用 0003…，现网机顶盒是 0004…），
// **不做前缀白名单校验**。原用例把 「0013…」 期望成 false，等于隐含要求
// 前缀必须是 0001/0002/0003；照这个改实现会直接拒绝现网在用的 0004… 机顶盒，
// 因此这里把期望修正为 true，并补上 0004… 的回归用例锁住该约定。
func TestCheckSNCode(t *testing.T) {
	valid := []string{
		"000100161231015063002D47", // 测试用例中的 0001 前缀
		"000200325207021201504CB6", // 0002 前缀
		"00030021535101708110051Q", // shdxiptv.md 文档示例（含字母）
		"001300325407022101504DC9", // 0013 前缀：仅格式校验，应通过
		"0004001720180529000020E8", // 现网机顶盒 SN：绝不能被拒绝
	}
	for _, s := range valid {
		if !CheckSNCode(s) {
			t.Errorf("CheckSNCode(%q) = false, 期待 true（24 位字母数字应通过）", s)
		}
	}

	invalid := map[string]string{
		"":                          "空字符串",
		"000300325407022101504DC":   "只有 23 位",
		"000300325407022101504DC11": "有 25 位",
		"000300325407022101504DC-":  "含非字母数字字符",
		"000300325407022101504D ":   "含空格",
		"000300325407022101504DC9A": "有 25 位（字母）",
	}
	for s, why := range invalid {
		if CheckSNCode(s) {
			t.Errorf("CheckSNCode(%q) = true, 期待 false（%s）", s, why)
		}
	}
}
