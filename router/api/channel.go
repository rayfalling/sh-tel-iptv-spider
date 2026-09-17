package api

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"iptv-spider-sh/global"
	"iptv-spider-sh/model"
	"iptv-spider-sh/modules/auth"

	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
)

// channelItem GET /api/channel/list 的一行
type channelItem struct {
	CommName   string `json:"comm_name"`
	Name       string `json:"name"`
	MixNo      string `json:"mix_no"`
	IsShow     bool   `json:"is_show"`
	IsHD       bool   `json:"is_hd"`
	Is4K       bool   `json:"is_4k"`
	CustomName string `json:"custom_name"`
	SortOrder  int    `json:"sort_order"`
	IsCustom   bool   `json:"is_custom"`
	TvgID      string `json:"tvg_id"`
	Igmp       string `json:"igmp"`
	Logo       string `json:"logo"`
	Group      string `json:"group"`
}

// effectiveLogo 按 M3U 的规则拼出完整台标地址
func effectiveLogo(logo, commName string) string {
	base := global.CONFIG.Epg.LogoUrl
	if logo == "" {
		if base == "" {
			return ""
		}
		return base + commName + ".png"
	}
	if strings.HasPrefix(logo, "http://") || strings.HasPrefix(logo, "https://") {
		return logo
	}
	return base + logo
}

func effectiveGroup(m model.M3u8Mapping) string {
	if m.CustomGroups != "" {
		return m.CustomGroups
	}
	return m.AutoGroups
}

// listChannels GET /api/channel/list
// 返回 IPTV 频道（channel_infos）与自定义频道（m3u8_mappings.is_custom）合并后的列表。
func listChannels(ctx iris.Context) {
	if global.DB == nil {
		jsonErr(ctx, iris.StatusServiceUnavailable, "数据库未就绪")
		return
	}

	var infos []model.ChannelInfo
	if err := global.DB.Order("mix_no asc").Find(&infos).Error; err != nil {
		jsonErr(ctx, iris.StatusInternalServerError, "查询频道失败: "+err.Error())
		return
	}

	var mappings []model.M3u8Mapping
	if err := global.DB.Find(&mappings).Error; err != nil {
		jsonErr(ctx, iris.StatusInternalServerError, "查询频道映射失败: "+err.Error())
		return
	}
	mappingByComm := make(map[string]model.M3u8Mapping, len(mappings))
	for _, m := range mappings {
		mappingByComm[m.CommName] = m
	}

	items := make([]channelItem, 0, len(infos))
	seen := make(map[string]bool, len(infos))
	for _, info := range infos {
		if info.CommName == "" {
			continue
		}
		m := mappingByComm[info.CommName]
		tvgID := m.TvgId
		if tvgID == "" {
			tvgID = info.MixNo
		}
		items = append(items, channelItem{
			CommName:   info.CommName,
			Name:       info.Name,
			MixNo:      info.MixNo,
			IsShow:     info.IsShow,
			IsHD:       info.IsHD,
			Is4K:       info.Is4K,
			CustomName: m.CustomName,
			SortOrder:  m.SortOrder,
			IsCustom:   m.IsCustom,
			TvgID:      tvgID,
			Igmp:       m.Igmp,
			Logo:       effectiveLogo(m.Logo, info.CommName),
			Group:      effectiveGroup(m),
		})
		seen[info.CommName] = true
	}

	// 自定义频道（无对应 channel_infos 行）也要出现在管理列表里
	for _, m := range mappings {
		if !m.IsCustom || seen[m.CommName] {
			continue
		}
		items = append(items, channelItem{
			CommName:   m.CommName,
			Name:       m.CommName,
			MixNo:      "",
			IsShow:     true,
			IsHD:       false,
			Is4K:       false,
			CustomName: m.CustomName,
			SortOrder:  m.SortOrder,
			IsCustom:   true,
			TvgID:      m.TvgId,
			Igmp:       m.Igmp,
			Logo:       effectiveLogo(m.Logo, m.CommName),
			Group:      effectiveGroup(m),
		})
	}

	// 稳定排序：排序权重（0 视为默认）→ 频道号
	sort.SliceStable(items, func(i, j int) bool {
		oi, oj := items[i].SortOrder, items[j].SortOrder
		if (oi == 0) != (oj == 0) {
			return oi != 0 // 设了权重的排在前面
		}
		if oi != oj {
			return oi < oj
		}
		return compareMixNo(items[i].MixNo, items[j].MixNo)
	})

	jsonOK(ctx, items)
}

// compareMixNo 频道号比较：纯数字按数值比（1 < 2 < 10），否则按字典序。
// 直接用字符串比较会得到 1,10,100,11... 这种顺序。
func compareMixNo(a, b string) bool {
	na, ea := strconv.Atoi(a)
	nb, eb := strconv.Atoi(b)
	if ea == nil && eb == nil {
		return na < nb
	}
	return a < b
}

type commNameReq struct {
	CommName string `json:"comm_name"`
}

type renameReq struct {
	CommName   string `json:"comm_name"`
	CustomName string `json:"custom_name"`
}

type sortReq struct {
	Orders []struct {
		CommName  string `json:"comm_name"`
		SortOrder int    `json:"sort_order"`
	} `json:"orders"`
}

type customAddReq struct {
	Name  string `json:"name"`
	Igmp  string `json:"igmp"`
	TvgID string `json:"tvg_id"`
	Logo  string `json:"logo"`
	Group string `json:"group"`
}

type customUpdateReq struct {
	CommName string  `json:"comm_name"`
	Igmp     *string `json:"igmp"`
	TvgID    *string `json:"tvg_id"`
	Logo     *string `json:"logo"`
	Group    *string `json:"group"`
}

// toggleChannel POST /api/channel/toggle
func toggleChannel(ctx iris.Context) {
	if !guardWrite(ctx) {
		return
	}
	var req commNameReq
	if err := ctx.ReadJSON(&req); err != nil || strings.TrimSpace(req.CommName) == "" {
		jsonErr(ctx, iris.StatusBadRequest, "参数错误")
		return
	}
	commName := strings.TrimSpace(req.CommName)

	var cnt int64
	if err := global.DB.Model(&model.ChannelInfo{}).Where("comm_name = ?", commName).Count(&cnt).Error; err != nil {
		jsonErr(ctx, iris.StatusInternalServerError, "查询频道失败: "+err.Error())
		return
	}
	if cnt == 0 {
		jsonErr(ctx, iris.StatusNotFound, "频道不存在")
		return
	}

	// 同一 comm_name 可能有多条（HD/SD 变体），与 M3U 去重维度保持一致，整组切换
	if err := global.DB.Model(&model.ChannelInfo{}).Where("comm_name = ?", commName).
		Update("is_show", gorm.Expr("NOT is_show")).Error; err != nil {
		jsonErr(ctx, iris.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}

	var first model.ChannelInfo
	if err := global.DB.Where("comm_name = ?", commName).First(&first).Error; err != nil {
		jsonErr(ctx, iris.StatusInternalServerError, "读取新状态失败: "+err.Error())
		return
	}

	global.BumpM3UCache()
	jsonOK(ctx, iris.Map{"success": true, "comm_name": commName, "is_show": first.IsShow})
}

// renameChannel POST /api/channel/rename
func renameChannel(ctx iris.Context) {
	if !guardWrite(ctx) {
		return
	}
	var req renameReq
	if err := ctx.ReadJSON(&req); err != nil || strings.TrimSpace(req.CommName) == "" {
		jsonErr(ctx, iris.StatusBadRequest, "参数错误")
		return
	}
	commName := strings.TrimSpace(req.CommName)

	if err := upsertMapping(commName, func(m *model.M3u8Mapping) {
		m.CustomName = req.CustomName // 传空字符串即恢复默认
	}); err != nil {
		jsonErr(ctx, iris.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}

	global.BumpM3UCache()
	jsonOK(ctx, iris.Map{"success": true, "comm_name": commName, "custom_name": req.CustomName})
}

// sortChannels POST /api/channel/sort
func sortChannels(ctx iris.Context) {
	if !guardWrite(ctx) {
		return
	}
	var req sortReq
	if err := ctx.ReadJSON(&req); err != nil || len(req.Orders) == 0 {
		jsonErr(ctx, iris.StatusBadRequest, "参数错误")
		return
	}

	count := 0
	for _, o := range req.Orders {
		commName := strings.TrimSpace(o.CommName)
		if commName == "" {
			continue
		}
		order := o.SortOrder
		if err := upsertMapping(commName, func(m *model.M3u8Mapping) {
			m.SortOrder = order
		}); err != nil {
			jsonErr(ctx, iris.StatusInternalServerError, "保存失败: "+err.Error())
			return
		}
		count++
	}

	global.BumpM3UCache()
	jsonOK(ctx, iris.Map{"success": true, "count": count})
}

// addCustomChannel POST /api/channel/custom/add
func addCustomChannel(ctx iris.Context) {
	if !guardWrite(ctx) {
		return
	}
	var req customAddReq
	if err := ctx.ReadJSON(&req); err != nil {
		jsonErr(ctx, iris.StatusBadRequest, "参数错误：name 和 igmp 为必填")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Igmp = strings.TrimSpace(req.Igmp)
	if req.Name == "" || req.Igmp == "" {
		jsonErr(ctx, iris.StatusBadRequest, "参数错误：name 和 igmp 为必填")
		return
	}

	commName := strings.ToUpper(req.Name)

	var exist model.M3u8Mapping
	err := global.DB.Where("comm_name = ?", commName).First(&exist).Error
	if err == nil {
		jsonErr(ctx, iris.StatusConflict, "频道名称已存在")
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		jsonErr(ctx, iris.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}

	group := strings.TrimSpace(req.Group)
	if group == "" {
		group = auth.AutoGroupByName(commName)
	}

	row := model.M3u8Mapping{
		CommName:     commName,
		Logo:         req.Logo,
		AutoGroups:   group,
		CustomGroups: strings.TrimSpace(req.Group),
		IsCustom:     true,
		TvgId:        req.TvgID,
		Igmp:         req.Igmp,
	}
	if err := global.DB.Create(&row).Error; err != nil {
		jsonErr(ctx, iris.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}

	global.BumpM3UCache()
	jsonOK(ctx, iris.Map{"success": true, "data": row})
}

// updateCustomChannel POST /api/channel/custom/update
func updateCustomChannel(ctx iris.Context) {
	if !guardWrite(ctx) {
		return
	}
	var req customUpdateReq
	if err := ctx.ReadJSON(&req); err != nil || strings.TrimSpace(req.CommName) == "" {
		jsonErr(ctx, iris.StatusBadRequest, "参数错误")
		return
	}
	commName := strings.TrimSpace(req.CommName)

	var row model.M3u8Mapping
	if err := global.DB.Where("comm_name = ? AND is_custom = ?", commName, true).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			jsonErr(ctx, iris.StatusNotFound, "频道不存在")
			return
		}
		jsonErr(ctx, iris.StatusInternalServerError, "查询失败: "+err.Error())
		return
	}

	if req.Igmp != nil {
		row.Igmp = *req.Igmp
	}
	if req.TvgID != nil {
		row.TvgId = *req.TvgID
	}
	if req.Logo != nil {
		row.Logo = *req.Logo
	}
	if req.Group != nil {
		row.AutoGroups = *req.Group
		row.CustomGroups = *req.Group
	}
	if err := global.DB.Save(&row).Error; err != nil {
		jsonErr(ctx, iris.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}

	global.BumpM3UCache()
	jsonOK(ctx, iris.Map{"success": true})
}

// deleteCustomChannel POST /api/channel/custom/delete
func deleteCustomChannel(ctx iris.Context) {
	if !guardWrite(ctx) {
		return
	}
	var req commNameReq
	if err := ctx.ReadJSON(&req); err != nil || strings.TrimSpace(req.CommName) == "" {
		jsonErr(ctx, iris.StatusBadRequest, "参数错误")
		return
	}
	commName := strings.TrimSpace(req.CommName)

	res := global.DB.Where("comm_name = ? AND is_custom = ?", commName, true).Delete(&model.M3u8Mapping{})
	if res.Error != nil {
		jsonErr(ctx, iris.StatusInternalServerError, "删除失败: "+res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		jsonErr(ctx, iris.StatusNotFound, "自定义频道不存在")
		return
	}

	global.BumpM3UCache()
	jsonOK(ctx, iris.Map{"success": true})
}

// channelM3u8 GET /api/channel/m3u8?name=xxx
// 返回单个频道的 M3U8 片段（纯文本）
func channelM3u8(ctx iris.Context) {
	name := strings.TrimSpace(ctx.FormValue("name"))
	if name == "" {
		ctx.StatusCode(iris.StatusBadRequest)
		ctx.WriteString("缺少 name 参数")
		return
	}

	data, err := auth.GenerateSingleChannelM3u8(name,
		ctx.FormValue("udpxy"), ctx.FormValue("scheme"), ctx.FormValue("xteve"))
	if err != nil {
		ctx.StatusCode(iris.StatusNotFound)
		ctx.WriteString(err.Error())
		return
	}

	ctx.ContentType("text/plain; charset=utf-8")
	ctx.Write(data)
}

// upsertMapping 按 comm_name 读取或新建映射行后应用变更
func upsertMapping(commName string, apply func(m *model.M3u8Mapping)) error {
	var row model.M3u8Mapping
	err := global.DB.Where("comm_name = ?", commName).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = model.M3u8Mapping{CommName: commName}
	} else if err != nil {
		return err
	}
	apply(&row)
	return global.DB.Save(&row).Error
}
