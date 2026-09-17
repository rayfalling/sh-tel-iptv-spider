package model

import "testing"

// TestSelectChannelInfo 锁定「同一 comm_name 分组里哪个变体会被输出」的规则。
// 频道管理接口用它判断某一行是否被同组 HD/4K 频道覆盖，
// 规则必须与 RemoveDuplicateChannelInfo 完全一致，否则面板显示与实际输出会再次分叉。
func TestSelectChannelInfo(t *testing.T) {
	sd := ChannelInfo{MixNo: "1", Name: "新闻综合"}
	hd := ChannelInfo{MixNo: "107", Name: "新闻综合HD", IsHD: true}
	uhd := ChannelInfo{MixNo: "200", Name: "新闻综合4K", IsHD: true, Is4K: true}
	sd2 := ChannelInfo{MixNo: "2", Name: "新闻综合"}

	cases := []struct {
		name string
		in   []ChannelInfo
		want string
	}{
		{"单条", []ChannelInfo{sd}, "1"},
		{"SD 在前 HD 在后", []ChannelInfo{sd, hd}, "107"},
		{"HD 在前 SD 在后", []ChannelInfo{hd, sd}, "107"},
		{"4K 优先于 HD", []ChannelInfo{sd, hd, uhd}, "200"},
		{"4K 在最后", []ChannelInfo{sd, hd, uhd}, "200"},
		{"两条 SD 取后者（与去重折叠一致）", []ChannelInfo{sd, sd2}, "2"},
		{"空列表返回零值", nil, ""},
	}
	for _, c := range cases {
		if got := SelectChannelInfo(c.in).MixNo; got != c.want {
			t.Errorf("%s: SelectChannelInfo() = %q, want %q", c.name, got, c.want)
		}
	}

	// 与去重结果交叉验证：去重留下的那一条必须等于 SelectChannelInfo 选中的那一条
	groups := [][]ChannelInfo{
		{sd, hd},
		{hd, sd},
		{sd, hd, uhd},
		{sd, sd2},
		{uhd, hd, sd},
	}
	for i, g := range groups {
		dedup := RemoveDuplicateChannelInfo(g, false)
		if len(dedup) != 1 {
			t.Fatalf("group %d: RemoveDuplicateChannelInfo returned %d rows, want 1", i, len(dedup))
		}
		if want := SelectChannelInfo(g).MixNo; dedup[0].MixNo != want {
			t.Errorf("group %d: dedup kept MixNo=%q but SelectChannelInfo chose %q", i, dedup[0].MixNo, want)
		}
	}
}
