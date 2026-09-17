package auth

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"iptv-spider-sh/config"
	"iptv-spider-sh/global"
	"iptv-spider-sh/model"
	"iptv-spider-sh/modules/m3u"
	"iptv-spider-sh/utils"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-module/carbon"
)

const timeFormat = carbon.ShortDateTimeLayout + " -0700"

// compareStrings 比较两个字符串，支持中文排序
// 返回 true 表示 str1 应该排在 str2 前面
func compareStrings(str1, str2 string) bool {
	if str1 == str2 {
		return false
	}

	// 尝试数字比较（如果都是数字）
	num1, err1 := strconv.Atoi(str1)
	num2, err2 := strconv.Atoi(str2)
	if err1 == nil && err2 == nil {
		return num1 < num2
	}

	// 使用 strings.Compare 进行字符串比较
	// strings.Compare 返回：
	// -1 如果 str1 < str2
	// 0  如果 str1 == str2
	// 1  如果 str1 > str2
	return strings.Compare(str1, str2) < 0
}

// getChannelInfoList 获取频道信息列表，包含错误处理
func getChannelInfoList(orderBy string) ([]model.ChannelInfo, error) {
	var channelInfoList []model.ChannelInfo

	var err error
	if orderBy != "" {
		err = global.DB.Order(orderBy).Find(&channelInfoList).Error
	} else {
		err = global.DB.Find(&channelInfoList).Error
	}

	if err != nil {
		global.LOG.Error("查询频道信息失败: " + err.Error())
		return nil, err
	}

	// 填充分组信息
	for i := range channelInfoList {
		channelInfoList[i].Group = autoGroupByName(channelInfoList[i].Name)
	}
	return channelInfoList, nil
}

// getChannelUrlsList 根据 comm_name 获取频道URL信息列表
func getChannelUrlsList(commName string) ([]model.ChannelUrlInfo, error) {
	var channelUrlsList []model.ChannelUrlInfo

	query := global.DB.Table("channels as a").
		Select("a.user_channel_id, a.channel_url, a.channel_sdp, a.time_shift_url, a.channel_fcc_port, a.channel_fcc_ip, f.comm_name, f.name, f.mix_no, f.is_show").
		Joins("INNER JOIN channel_infos as f ON a.user_channel_id = f.mix_no").
		Order("f.comm_name")

	if commName != "" {
		query = query.Where("f.comm_name = ?", commName)
	}

	err := query.Find(&channelUrlsList).Error
	if err != nil {
		global.LOG.Error("查询频道URL信息失败: " + err.Error())
		return nil, err
	}

	// 填充分组信息
	for i := range channelUrlsList {
		channelUrlsList[i].Group = autoGroupByName(channelUrlsList[i].Name)
	}

	return channelUrlsList, nil
}

// getM3u8Mapping 获取频道映射信息，包含错误处理
func getM3u8Mapping(commName string) (model.M3u8Mapping, error) {
	var m3u8Mapping model.M3u8Mapping

	if commName == "" {
		return m3u8Mapping, nil
	}

	err := global.DB.Where("comm_name = ?", commName).Find(&m3u8Mapping).Error
	if err != nil {
		global.LOG.Error(fmt.Sprintf("查询频道映射失败 (CommName: %s): %s", commName, err.Error()))
		return m3u8Mapping, err
	}

	return m3u8Mapping, nil
}

func GenerateM3u8(udpxy, scheme, xteve, all, ku9 string) []byte {
	// 配置空值检查
	if global.CONFIG == nil || global.CONFIG.Epg.XmlUrl == "" {
		global.LOG.Error("配置文件未正确加载:Epg.XmlUrl")
		return nil
	}

	m3uWriter := m3u.NewWriter()
	m3uWriter.WriteHeaderWithInfo(global.CONFIG.Epg.XmlUrl)            //加载配置文件参数，
	fmt.Println("ChannelMappings:", global.CONFIG.Epg.ChannelMappings) //确认配置是否加载调试

	// 查询数据库
	channelInfoList, err := getChannelInfoList("mix_no asc")
	if err != nil {
		global.LOG.Error("查询频道信息失败: " + err.Error())
		return nil
	}

	fmt.Println("查询到的频道数量:", len(channelInfoList))
	// 去重
	newChanInfo := model.RemoveDuplicateChannelInfo(channelInfoList, false)
	// 输出去重后的频道数量和信息
	fmt.Println("去重后的频道数量:", len(newChanInfo))
	//fmt.Println("去重后的频道信息:", newChanInfo)

	// 构建映射表
	mappingMap := make(map[string]config.ChannelMapping)
	if global.CONFIG.Epg.ChannelMappings != nil {
		for _, m := range global.CONFIG.Epg.ChannelMappings {
			mappingMap[m.Igmp] = m
		}
	}

	// 构建最终列表：先加 channel_infos，再加未匹配的 ChannelMappings
	type M3uItem struct {
		Info    model.ChannelInfo
		Channel model.Channel
		Mapping *config.ChannelMapping
	}

	// 性能优化：批量查询频道详情
	// 1. 收集所有需要查询的 MixNo
	var mixNos []string
	for _, info := range newChanInfo {
		// all=true 时把隐藏频道也纳入（对应 API.md：包含所有频道（含已隐藏））
		if info.IsShow || all == "true" {
			mixNos = append(mixNos, info.MixNo)
		}
	}

	// 2. 批量查询所有 Channel
	var channels []model.Channel
	if len(mixNos) > 0 {
		if err := global.DB.Where("user_channel_id IN (?)", mixNos).Find(&channels).Error; err != nil {
			global.LOG.Error("批量查询频道详情失败: " + err.Error())
			// 不返回，继续处理，部分频道可能无法获取
		}
	}

	// 3. 构建 Channel 映射表
	channelMap := make(map[string]model.Channel)
	for _, channel := range channels {
		channelMap[channel.UserChannelID] = channel
	}

	var finalList []M3uItem
	processed := make(map[string]bool)
	for _, info := range newChanInfo {
		if !info.IsShow && all != "true" {
			continue
		}

		channel, ok := channelMap[info.MixNo]
		if !ok {
			global.LOG.Warn(fmt.Sprintf("未找到频道详情 (MixNo: %s)，跳过该频道", info.MixNo))
			continue
		}

		key := fmt.Sprintf("%v", channel.ChannelURL)
		processed[key] = true

		var mapping *config.ChannelMapping
		if m, ok := mappingMap[channel.ChannelURL]; ok {
			mapping = &m
		}

		finalList = append(finalList, M3uItem{
			Info:    info,
			Channel: channel,
			Mapping: mapping,
		})
	}

	// 加入 ChannelMappings 中未匹配的 IGMP 频道
	if global.CONFIG.Epg.ChannelMappings != nil {
		for _, m := range global.CONFIG.Epg.ChannelMappings {
			if _, ok := processed[m.Igmp]; !ok {
				channel := model.Channel{}
				// 使用 IGMP 去 channels 表匹配 channel_url
				if err := global.DB.Where("channel_url = ?", m.Igmp).Find(&channel).Error; err != nil {
					global.LOG.Error(fmt.Sprintf("查询IGMP频道失败 (IGMP: %s): %s", m.Igmp, err.Error()))
					continue // 跳过这条记录，继续处理其他
				}

				info := model.ChannelInfo{
					MixNo:    m.Id,
					CommName: m.Name,
					Name:     m.Name,
					IsShow:   true,
				}
				finalList = append(finalList, M3uItem{
					Info:    info,
					Channel: channel,
					Mapping: &m,
				})
			}
		}
	}
	// 构建 name_sequence 顺序表
	orderMap := make(map[string]int)
	if global.CONFIG.Epg.NameSequence != nil {
		for i, n := range global.CONFIG.Epg.NameSequence {
			orderMap[n.Name] = i
		}
	}

	// 构建Exclude_Channels 映射表
	excludeMap := make(map[string]bool)
	for _, m := range global.CONFIG.Epg.ChannelMappings {
		for _, ex := range m.Exclude_channels {
			excludeMap[ex] = true
		}
	}

	// 预加载频道映射表：排序（sort_order）与写入（custom_name/logo/分组）都要用。
	// 原先在写入循环里逐条 getM3u8Mapping 查库，这里改成一次查询。
	var allMappings []model.M3u8Mapping
	if err := global.DB.Find(&allMappings).Error; err != nil {
		global.LOG.Error("查询频道映射表失败: " + err.Error())
	}
	mappingByComm := make(map[string]model.M3u8Mapping, len(allMappings))
	for _, m := range allMappings {
		mappingByComm[m.CommName] = m
	}

	// 对 finalList 排序：sort_order（0=默认） > name_sequence > 频道号
	sort.SliceStable(finalList, func(i, j int) bool {
		si := mappingByComm[finalList[i].Info.CommName].SortOrder
		sj := mappingByComm[finalList[j].Info.CommName].SortOrder
		if (si == 0) != (sj == 0) {
			return si != 0 // 手动排过序的频道优先
		}
		if si != sj {
			return si < sj
		}

		nameI := finalList[i].Info.Name
		nameJ := finalList[j].Info.Name

		indexI, okI := orderMap[nameI]
		indexJ, okJ := orderMap[nameJ]

		// 情况1：两个频道都在排序表中
		if okI && okJ {
			// 按排序表顺序排序
			return indexI < indexJ
		}

		// 情况2：两个频道都不在排序表中
		if !okI && !okJ {
			// 使用辅助函数进行排序比较
			return compareStrings(finalList[i].Info.MixNo, finalList[j].Info.MixNo)
		}

		// 情况3：一个在排序表，一个不在（在排序表中的优先）
		return okI
	})
	// ✅ 统一循环写入 m3u
	for _, item := range finalList {
		info := item.Info
		channel := item.Channel
		// 排除被标记为Exclude_channels的频道
		if excludeMap[info.Name] {
			continue
		}
		// 频道映射信息（已预加载，避免逐条查库）
		m3u8Mapping := mappingByComm[info.CommName]
		// 自定义显示名称（频道重命名接口写入）优先于原始名称
		if m3u8Mapping.CustomName != "" {
			info.Name = m3u8Mapping.CustomName
		}
		if m3u8Mapping.AutoGroups == "" {
			m3u8Mapping.AutoGroups = autoGroupByName(info.Name)
		}

		// 默认 logo
		if m3u8Mapping.Logo == "" {
			logoBaseUrl := global.CONFIG.Epg.LogoUrl
			logoImageName := fmt.Sprintf("%s.png", info.CommName)
			m3u8Mapping.Logo = fmt.Sprintf("%s%s", logoBaseUrl, logoImageName)
		}

		// 用户自定义映射覆盖
		if item.Mapping != nil {
			if item.Mapping.Name != "" {
				info.CommName = item.Mapping.Name

			}
			if item.Mapping.Logo != "" {
				m3u8Mapping.Logo = global.CONFIG.Epg.LogoUrl + item.Mapping.Logo
			}

		}

		uri := assemblyUrl(udpxy, scheme, xteve, channel.ChannelURL, channel.ChannelFCCIP, channel.ChannelFCCPort)

		catchupSource := ""
		if channel.TimeShiftURL != "" {
			trimmed := strings.TrimPrefix(channel.TimeShiftURL, "rtsp://")
			playseek := global.CONFIG.Epg.Playseek
			if ku9 == "true" {
				playseek = "&playseek=${(b)yyyyMMddHHmmss}-${(e)yyyyMMddHHmmss}"
			}
			catchupSource = fmt.Sprintf("%s%s%s", global.CONFIG.Epg.RtspUrl, trimmed, playseek)
		}

		m3uWriter.WriteWithCatchup(uri, catchupSource, info, m3u8Mapping)
	}

	// 追加数据库中的自定义频道（/api/channel/custom/add 写入的行：is_custom=true 且带 igmp）。
	// 作者原先只支持 config.yaml 里 channel_mappings 的自定义频道，在线新增的频道需要在这里输出。
	for _, m := range allMappings {
		if !m.IsCustom || m.Igmp == "" {
			continue
		}
		display := m.CommName
		if m.CustomName != "" {
			display = m.CustomName
		}
		info := model.ChannelInfo{
			MixNo:    m.TvgId,
			CommName: display,
			Name:     display,
			IsShow:   true,
		}
		if info.MixNo == "" {
			info.MixNo = m.CommName
		}
		if m.Logo == "" {
			m.Logo = global.CONFIG.Epg.LogoUrl + m.CommName + ".png"
		}
		uri := assemblyUrl(udpxy, scheme, xteve, m.Igmp, "", "")
		m3uWriter.Write(uri, info, m)
	}

	return m3uWriter.Bytes()
}

func GenerateTimeShiftM3u8(udpxy, scheme, xteve, all string) []byte {
	// 配置空值检查
	if global.CONFIG == nil || global.CONFIG.Epg.XmlUrl == "" {
		global.LOG.Error("配置文件未正确加载:Epg.XmlUrl")
		return nil
	}

	m3uWriter := m3u.NewWriter()
	m3uWriter.WriteHeaderWithInfo(global.CONFIG.Epg.XmlUrl)
	// 查询数据库
	channelInfoList, err := getChannelInfoList("")
	if err != nil {
		global.LOG.Error("查询时移频道信息失败: " + err.Error())
		return nil
	}
	// 去重
	newChannelInfoList := model.RemoveDuplicateChannelInfo(channelInfoList, false)

	// 构建 name_sequence 顺序表
	orderMap := make(map[string]int)
	if global.CONFIG.Epg.NameSequence != nil {
		for i, n := range global.CONFIG.Epg.NameSequence {
			orderMap[n.Name] = i
		}
	}
	SortChannelsByFields(newChannelInfoList, orderMap,
		func(item model.ChannelInfo) string { return item.Name },
		func(item model.ChannelInfo) string { return item.Group },
		func(item model.ChannelInfo) string { return item.CommName },
	)

	// 构建Exclude_Channels 映射表
	excludeMap := make(map[string]bool)
	for _, m := range global.CONFIG.Epg.ChannelMappings {
		for _, ex := range m.Exclude_channels {
			excludeMap[ex] = true
		}
	}

	for _, info := range newChannelInfoList {
		// 不展示（all=true 时包含隐藏频道）
		if !info.IsShow && all != "true" {
			continue
		}
		// 排除被标记为Exclude_channels的频道
		if excludeMap[info.Name] {
			continue
		}
		channel := model.Channel{}
		if err := global.DB.Where("user_channel_id = ?", info.MixNo).Find(&channel).Error; err != nil {
			global.LOG.Error(fmt.Sprintf("查询时移频道详情失败 (MixNo: %s): %s", info.MixNo, err.Error()))
			continue
		}

		m3u8Mapping, err := getM3u8Mapping(info.CommName)
		if err != nil {
			global.LOG.Error(fmt.Sprintf("查询时移频道映射失败 (CommName: %s): %s", info.CommName, err.Error()))
			continue
		}

		uri := assemblyUrl(udpxy, scheme, xteve, channel.ChannelURL, "", "")
		m3uWriter.Write(uri, info, m3u8Mapping)
	}
	return m3uWriter.Bytes()
}

func GenerateDiyp(udpxy, scheme, xteve, all string) []byte {
	// 配置空值检查
	if global.CONFIG == nil || global.CONFIG.Epg.XmlUrl == "" {
		global.LOG.Error("配置文件未正确加载:Epg.XmlUrl")
		return nil
	}

	m3uWriter := m3u.NewWriter()

	// 查询数据库
	channelUrlsList, err := getChannelUrlsList("")
	if err != nil {
		global.LOG.Error("查询Diyp频道信息失败: " + err.Error())
		return nil
	}

	// 构建 name_sequence 顺序表
	orderMap := make(map[string]int)
	if global.CONFIG.Epg.NameSequence != nil {
		for i, n := range global.CONFIG.Epg.NameSequence {
			orderMap[n.Name] = i
		}
	} else {
		global.LOG.Info("提示:节目排序配置name_sequence为空.")
	}

	SortChannelsByFields(channelUrlsList, orderMap,
		func(item model.ChannelUrlInfo) string { return item.Name },
		func(item model.ChannelUrlInfo) string { return item.Group },
		func(item model.ChannelUrlInfo) string { return item.CommName },
	)

	/*
		// 对根据channelUrlsList.Name 进行排序
		sort.SliceStable(channelUrlsList, func(i, j int) bool {
			orderI, okI := orderMap[channelUrlsList[i].Name]
			orderJ, okJ := orderMap[channelUrlsList[j].Name]
			if okI != okJ {
				return okI
			}
			return orderI < orderJ
		})
		// 根据channelUrlsList.Group以节目分组进行排序, 相同分组内再以CommName排序
		sort.SliceStable(channelUrlsList, func(i, j int) bool {
			if channelUrlsList[i].Group != channelUrlsList[j].Group {
				return channelUrlsList[i].Group < channelUrlsList[j].Group
			}
			return channelUrlsList[i].CommName < channelUrlsList[j].CommName
		})
	*/
	// 构建Exclude_Channels 映射表
	excludeMap := make(map[string]bool)
	for _, m := range global.CONFIG.Epg.ChannelMappings {
		for _, ex := range m.Exclude_channels {
			excludeMap[ex] = true
		}
	}

	prev_groupName := ""
	for _, info := range channelUrlsList {
		// 不展示（all=true 时包含隐藏频道）
		if !info.IsShow && all != "true" {
			continue
		}
		// 排除被标记为Exclude_channels的频道
		if excludeMap[info.Name] {
			continue
		}
		// 写入分组头
		if info.Group != prev_groupName {
			m3uWriter.WriteGroupHeader(info.Group)
			prev_groupName = info.Group
		}

		m3u8Mapping, err := getM3u8Mapping(info.CommName)
		if err != nil {
			global.LOG.Error(fmt.Sprintf("查询时移频道映射失败 (CommName: %s): %s", info.CommName, err.Error()))
			continue
		}

		uri := assemblyUrl(udpxy, scheme, xteve, info.ChannelURL, info.ChannelFCCIP, info.ChannelFCCPort)
		m3uWriter.WriteDiyp(uri, info, m3u8Mapping)
		catchupSource := ""
		if info.TimeShiftURL != "" {
			trimmed := strings.TrimPrefix(info.TimeShiftURL, "rtsp://")
			catchupSource = fmt.Sprintf("%s%s%s", global.CONFIG.Epg.RtspUrl, trimmed, global.CONFIG.Epg.Playseek)
			m3uWriter.WriteDiyp(catchupSource, info, m3u8Mapping)
		}

	}
	return m3uWriter.Bytes()
}

// func assemblyUrl(udpxy, scheme, xteve, uri string) string //修改
func assemblyUrl(udpxy, scheme, xteve, uri, fccIp, fccPort string) string {
	// 配置空值检查
	if global.CONFIG == nil {
		global.LOG.Error("配置文件未正确加载，无法生成URL")
		return ""
	}

	// 添加URL解析错误处理
	if uri == "" {
		return ""
	}
	u, err := url.Parse(uri)
	if err != nil {
		global.LOG.Error(fmt.Sprintf("URL解析失败 (URI: %s): %s", uri, err.Error()))
		return ""
	}
	// xteve模式（输出 udp://@，由 xteve 的 UDPxy 设置改写）
	if xteve == "true" {
		return fmt.Sprintf("udp://@%s", u.Host)
	}

	// udpxy模式
	if udpxy != "" {
		return fmt.Sprintf("http://%s/udp/%s", udpxy, u.Host)
	}

	// 自定义 scheme（文档中声明但此前未实现）：接受 rtp / rtp:// / rtsp:// 等写法
	if scheme != "" {
		return fmt.Sprintf("%s%s", normalizeScheme(scheme), u.Host)
	}

	// ⚠️ 只有默认 RTP 代理模式才需要 rtp_url。
	// 原实现在函数开头就检查它，导致 rtp_url 为空时连 xteve / udpxy / scheme 模式
	// 也一起返回空串（M3U 里每条地址都是空的）。
	if global.CONFIG.Epg.RtpUrl == "" {
		global.LOG.Error("配置文件未正确加载:Epg.RtpUrl，无法生成默认 RTP 代理地址")
		return ""
	}

	// HTTP RTP + FCC 使用动态加载的 rtp_url
	if fccIp != "" && fccPort != "" {
		return fmt.Sprintf(
			"%s%s?fcc=%s:%s",
			global.CONFIG.Epg.RtpUrl, // 使用动态加载的 rtp_url
			u.Host,
			fccIp,
			fccPort,
		)
	}

	// HTTP RTP 无FCC 使用动态加载的 rtp_url
	return fmt.Sprintf(
		"%s%s",
		global.CONFIG.Epg.RtpUrl, // 使用动态加载的 rtp_url
		u.Host,
	)
}

// normalizeScheme 把 "rtp" / "rtp://" / "rtp:" 统一成 "rtp://"
func normalizeScheme(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "://")
	s = strings.TrimSuffix(s, ":")
	return s + "://"
}

func GenerateXmlTv(daysAgo int) ([]byte, error) {
	// 配置空值检查
	if global.CONFIG == nil || global.CONFIG.Epg.Generator == "" {
		global.LOG.Error("配置文件未正确加载:Epg.Generator")
		return nil, errors.New("配置文件未正确加载")
	}

	if daysAgo < 1 {
		daysAgo = 1
	} else if daysAgo > 7 {
		daysAgo = 7
	}
	var now = carbon.Now()
	var xmlTv = model.XmlTV{
		Generator: fmt.Sprintf("%s %s", global.CONFIG.Epg.Generator, now.ToDateTimeString()),
		Source:    global.CONFIG.Epg.Source,
	}
	// 取数据
	channelInfoList, err := getChannelInfoList("")
	if err != nil {
		global.LOG.Error("查询EPG频道信息失败: " + err.Error())
		return nil, errors.New("查询频道信息失败")
	}
	// 去重
	newChanInfo := model.RemoveDuplicateChannelInfo(channelInfoList, true)

	// 性能优化：批量查询EPG数据
	// 1. 收集所有需要拉取EPG的频道名称
	var epgChannelNames []string
	for _, info := range newChanInfo {
		if info.IsShow && info.IsPullEPG && info.CommName != "" {
			epgChannelNames = append(epgChannelNames, info.CommName)
		}
	}

	// 2. 批量查询所有EPG数据
	var allEpgData []model.EPGDetails
	if len(epgChannelNames) > 0 {
		if err := global.DB.Where("comm_name IN (?)", epgChannelNames).
			Where("end_time > ?", now.SubDays(daysAgo).TimestampMilli()).
			Order("comm_name, start_time asc").
			Find(&allEpgData).Error; err != nil {
			global.LOG.Error("批量查询EPG数据失败: " + err.Error())
			// 不返回，继续处理，部分EPG数据可能无法获取
		}
	}

	// 3. 构建EPG数据映射表
	epgDataMap := make(map[string][]model.EPGDetails)
	for _, epg := range allEpgData {
		epgDataMap[epg.CommName] = append(epgDataMap[epg.CommName], epg)
	}

	for _, info := range newChanInfo {
		// 不展示
		if !info.IsShow {
			continue
		}
		chId := info.MixNo
		xmlTv.Channel = append(xmlTv.Channel, &model.XmlTvChannel{
			ID:          chId,
			DisplayName: []model.DisplayName{{Lang: "zh", Value: info.CommName}},
		})
		if !info.IsPullEPG {
			xmlTv.Program = append(xmlTv.Program, &model.Program{
				Channel: chId,
				Title:   []*model.Title{{Lang: "zh"}},
				Desc:    []*model.Desc{{Lang: "zh"}},
			})
			continue
		}

		// 从映射表中获取EPG数据
		epgData := epgDataMap[info.CommName]

		for _, epg := range epgData {
			startTime := carbon.CreateFromTimestampMilli(epg.StartTime).Layout(timeFormat)
			endTime := carbon.CreateFromTimestampMilli(epg.EndTime).Layout(timeFormat)
			xmlTv.Program = append(xmlTv.Program, &model.Program{
				Channel: chId,
				Start:   startTime,
				Stop:    endTime,
				Title:   []*model.Title{{Lang: "zh", Value: epg.Name}},
				Desc:    []*model.Desc{{Lang: "zh"}},
			})
		}
	}
	// 序列化
	epgBytes, err := xml.MarshalIndent(&xmlTv, "", "  ")
	if err != nil {
		global.LOG.Error("节目表单生成出错: " + err.Error())
		return nil, errors.New("节目表单生成出错")
	}
	epgBytes = append([]byte(model.PrefixHeader+"\n"), epgBytes...)
	return epgBytes, nil
}

func GenerateEpgJson(daysAgo int) ([]byte, error) {
	// 配置空值检查
	if global.CONFIG == nil || global.CONFIG.Epg.Generator == "" {
		global.LOG.Error("配置文件未正确加载:Epg.Generator")
		return nil, errors.New("配置文件未正确加载")
	}

	if daysAgo < 1 {
		daysAgo = 1
	} else if daysAgo > 7 {
		daysAgo = 7
	}
	var now = carbon.Now()
	var epgJson = model.XmlTV{
		Generator: fmt.Sprintf("%s %s", global.CONFIG.Epg.Generator, now.ToDateTimeString()),
		Source:    global.CONFIG.Epg.Source,
	}
	// 取数据
	channelInfoList, err := getChannelInfoList("")
	if err != nil {
		global.LOG.Error("查询EPG频道信息失败: " + err.Error())
		return nil, errors.New("查询频道信息失败")
	}
	// 去重
	newChanInfo := model.RemoveDuplicateChannelInfo(channelInfoList, false)

	// 性能优化：批量查询EPG数据
	// 1. 收集所有需要拉取EPG的频道名称
	var epgChannelNames []string
	for _, info := range newChanInfo {
		if info.IsShow && info.IsPullEPG && info.CommName != "" {
			epgChannelNames = append(epgChannelNames, info.CommName)
		}
	}

	// 2. 批量查询所有EPG数据
	var allEpgData []model.EPGDetails
	if len(epgChannelNames) > 0 {
		if err := global.DB.Where("comm_name IN (?)", epgChannelNames).
			Where("end_time > ?", now.SubDays(daysAgo).TimestampMilli()).
			Order("comm_name, start_time asc").
			Find(&allEpgData).Error; err != nil {
			global.LOG.Error("批量查询EPG数据失败: " + err.Error())
			// 不返回，继续处理，部分EPG数据可能无法获取
		}
	}

	// 3. 构建EPG数据映射表
	epgDataMap := make(map[string][]model.EPGDetails)
	for _, epg := range allEpgData {
		epgDataMap[epg.CommName] = append(epgDataMap[epg.CommName], epg)
	}

	for _, info := range newChanInfo {
		// 不展示
		if !info.IsShow {
			continue
		}
		chId := info.MixNo
		epgJson.Channel = append(epgJson.Channel, &model.XmlTvChannel{
			ID:          chId,
			DisplayName: []model.DisplayName{{Lang: "zh", Value: info.CommName}},
		})
		if !info.IsPullEPG {
			epgJson.Program = append(epgJson.Program, &model.Program{
				Channel: chId,
				Title:   []*model.Title{{Lang: "zh"}},
				Desc:    []*model.Desc{{Lang: "zh"}},
			})
			continue
		}

		// 从映射表中获取EPG数据
		epgData := epgDataMap[info.CommName]

		for _, epg := range epgData {
			startTime := carbon.CreateFromTimestampMilli(epg.StartTime).Layout(timeFormat)
			endTime := carbon.CreateFromTimestampMilli(epg.EndTime).Layout(timeFormat)
			epgJson.Program = append(epgJson.Program, &model.Program{
				Channel: chId,
				Start:   startTime,
				Stop:    endTime,
				Title:   []*model.Title{{Lang: "zh", Value: epg.Name}},
				Desc:    []*model.Desc{{Lang: "zh"}},
			})
		}
	}
	// 序列化
	epgBytes, err := json.MarshalIndent(&epgJson, "", "  ")
	if err != nil {
		global.LOG.Error("节目表单生成出错: " + err.Error())
		return nil, errors.New("节目表单生成出错")
	}
	return epgBytes, nil
}

func GenerateAndUploadEpgJson() {
	jsonBytes, _ := GenerateEpgJson(1)
	utils.UploadToOSS("/tv/tel-epg.json", jsonBytes)
}

func GenerateAndUploadEpgJsonDays7() {
	jsonBytes, _ := GenerateEpgJson(7)
	utils.UploadToOSS("/tv/tel-epg-7.json", jsonBytes)
}

func GenerateAndUploadM3u() {
	m3uBytes := GenerateM3u8("", "", "true", "", "")
	utils.UploadToOSS("/tv/tel-xteve.m3u", m3uBytes)
}
func GenerateAndUploadDiyp(updateSite string) {
	m3uBytes := GenerateDiyp("", "", "", "")
	utils.SaveToLogDir(m3uBytes, "iptvdiyp.txt")
	if updateSite == "oss" {
		utils.UploadToOSS("/tv/iptvdiyp.txt", m3uBytes)
	} else if updateSite == "scp" {
		remotePath := global.CONFIG.SCP.Bucket
		utils.ScpCopy(fmt.Sprintf("%s/tv/iptvdiyp.txt", global.CONFIG.Zap.Director), remotePath, global.CONFIG.SCP.EndPoint, global.CONFIG.SCP.AccessKey, global.CONFIG.SCP.SecretKey)
	}
}

func GenerateAndUploadXmlTv() {
	xmlTvBytes, _ := GenerateXmlTv(1)
	utils.UploadToOSS("/tv/tel-epg.xml", xmlTvBytes)
}

func GenerateAndUploadXmlTvDays7() {
	xmlTvBytes, _ := GenerateXmlTv(7)
	utils.UploadToOSS("/tv/tel-epg-7.xml", xmlTvBytes)
}
func autoGroupByName(name string) string {
	if strings.Contains(name, "CCTV") {
		return "1.央视"
	} else if strings.Contains(name, "卫视") {
		return "2.卫视"
	} else if strings.Contains(name, "购物") {
		return "6.购物"
	} else if strings.Contains(name, "年级") {
		return "5.空中课堂"
	} else if strings.Contains(name, "百事通") {
		return "3.百事通"
	} else if strings.Contains(name, "影") {
		return "4.电影"
	}
	return "7.其他"
}

// AutoGroupByName 导出给 router 使用（新增自定义频道时自动分配分组）
func AutoGroupByName(name string) string {
	return autoGroupByName(name)
}

// GenerateSingleChannelM3u8 生成单个频道的 M3U8 片段（供 GET /api/channel/m3u8 使用）。
// name 可用「通用频道名(comm_name)」或「原始频道名(name)」匹配，精确匹配。
func GenerateSingleChannelM3u8(name, udpxy, scheme, xteve string) ([]byte, error) {
	if global.CONFIG == nil || global.DB == nil {
		return nil, errors.New("配置或数据库未就绪")
	}

	var infos []model.ChannelInfo
	if err := global.DB.Where("comm_name = ? OR name = ?", name, name).Find(&infos).Error; err != nil {
		return nil, err
	}
	if len(infos) == 0 {
		return nil, fmt.Errorf("未找到频道: %s", name)
	}

	// 复用全局去重规则（4K > HD），保证与整表输出选中的是同一个变体
	picked := model.RemoveDuplicateChannelInfo(infos, false)
	info := picked[0]

	var channel model.Channel
	if err := global.DB.Where("user_channel_id = ?", info.MixNo).First(&channel).Error; err != nil {
		return nil, fmt.Errorf("未找到频道播放地址 (MixNo: %s)", info.MixNo)
	}

	mapping, _ := getM3u8Mapping(info.CommName)
	if mapping.CustomName != "" {
		info.Name = mapping.CustomName
	}
	if mapping.TvgId != "" {
		info.MixNo = mapping.TvgId
	}
	if mapping.AutoGroups == "" {
		mapping.AutoGroups = autoGroupByName(info.Name)
	}
	// 与整表输出保持一致：没有台标时按 logo_url + 频道名拼一个
	if mapping.Logo == "" {
		mapping.Logo = global.CONFIG.Epg.LogoUrl + info.CommName + ".png"
	} else if !strings.HasPrefix(mapping.Logo, "http://") && !strings.HasPrefix(mapping.Logo, "https://") {
		mapping.Logo = global.CONFIG.Epg.LogoUrl + mapping.Logo
	}

	w := m3u.NewWriter()
	w.WriteHeaderWithInfo(global.CONFIG.Epg.XmlUrl)

	uri := assemblyUrl(udpxy, scheme, xteve, channel.ChannelURL, channel.ChannelFCCIP, channel.ChannelFCCPort)

	catchupSource := ""
	if channel.TimeShiftURL != "" {
		catchupSource = fmt.Sprintf("%s%s%s",
			global.CONFIG.Epg.RtspUrl,
			strings.TrimPrefix(channel.TimeShiftURL, "rtsp://"),
			global.CONFIG.Epg.Playseek)
	}

	w.WriteWithCatchup(uri, catchupSource, info, mapping)
	return w.Bytes(), nil
}
