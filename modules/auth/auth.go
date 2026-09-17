package auth

import (
	"errors"
	"fmt"
	"iptv-spider-sh/global"
	"iptv-spider-sh/model"
	"iptv-spider-sh/modules/http_client"
	"iptv-spider-sh/modules/jsvm"
	"iptv-spider-sh/utils"
	"net/http"
	"net/url"
	"time"

	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

var (
	ErrUserID  = errors.New("错误的userID")
	ErrMacAddr = errors.New("错误的Mac地址")
	ErrSNCode  = errors.New("错误的SN码")
	ErrIPAddr  = errors.New("错误的IP地址")
)

var globalClient *Client

type baseConfig struct {
	userID           string
	sn               string
	ip               string
	macAddr          string
	stbType          string
	userAgent        string
	pre4kLogAuthAddr string
}

type Client struct {
	jsVM        *jsvm.VM
	httpClient  *http_client.HttpClient
	htmlDocTemp *goquery.Document
	baseConfig
	model.AuthInfo
}

func (c *Client) authSetupOne() (*goquery.Document, error) {
	global.LOG.Info("认证流程一")
	doc := c.pre4kLogAuth()
	if doc == nil {
		return nil, fmt.Errorf("pre4kLogAuth Failed")
	}
	doc = c.r4kLogAuth(doc)
	if doc == nil {
		return nil, fmt.Errorf("r4kLogAuth Failed")
	}
	doc = c.ottAuth(doc)
	if doc == nil {
		return nil, fmt.Errorf("ottAuth Failed")
	}
	channels := c.processChannel(doc)
	// 频道列表入库
	global.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_channel_id"}},
		UpdateAll: true,
	}).Create(&channels)
	c.htmlDocTemp = doc
	return doc, nil
}

func (c *Client) authSetupTwo() (*goquery.Document, error) {
	global.LOG.Info("认证流程二")
	doc := c.epgIndex(c.htmlDocTemp)
	global.LOG.Info(">>认证流程二, EPG epgLoadBalance: ")
	doc = c.epgLoadBalance(doc)
	global.LOG.Info(">>认证流程二, EPG epgPortalAuth: ")
	doc, err := c.epgPortalAuth(doc)
	if err != nil {
		global.LOG.Info("认证流程结束, 出现错误")
		return nil, err
	}
	c.epgGetPortal()
	// Cookies 入库
	global.LOG.Info("认证流程结束, 认证信息入库")
	global.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uid"}},
		UpdateAll: true,
	}).Create(&c.AuthInfo)
	c.updateCookies()
	return doc, nil
}

func (c *Client) StartAuth() error {
	global.LOG.Info("开始认证流程")
	_, error := c.authSetupOne()
	if error != nil {
		return error
	}
	_, err := c.authSetupTwo()
	return err
}

func (c *Client) HeartBeat() {
	p := "iptvepg/heartbeat.jsp"
	u, _ := url.Parse(c.EPGHostUrl)
	uri := fmt.Sprintf("http://%s/%s", u.Host, p)
	c.httpClient.Request(uri, "GET", nil)
}

func (c *Client) fetchAuthInfoFormDB() error {
	global.LOG.Info("从数据库获取 AuthInfo")
	global.DB.Find(&c.AuthInfo, c.AuthInfo)
	if c.AuthInfo.ID > 0 {
		c.updateCookies()
		err := c.checkSessionState()
		if err != nil {
			global.LOG.Error("从数据库获取 AuthInfo 失败", zap.Any("Msg", err.Error()))
			return err
		}
	} else {
		err := c.StartAuth()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) updateCookies() {
	u, err := url.Parse(c.AuthInfo.EPGHostUrl)
	if err != nil {
		global.LOG.Error("更新认证信息至运行环境失败 ", zap.Any("Msg", err.Error()))
		return
	}
	c.httpClient.SetCookies(&http.Cookie{
		Name:     "JSESSIONID",
		Value:    c.AuthInfo.JSESSIONID,
		Path:     "/",
		Domain:   u.Host,
		Secure:   false,
		HttpOnly: true,
	})
}

func NewClient(uid, sn, mac, ip string, options ...ClientOption) (*Client, error) {
	// 机顶盒标签常把 MAC 印成区间（如 9C:71:3A:6C:F6:F4-F5），这里自动取区间内的第一个地址。
	// 若认证失败，改用区间内的另一个地址再试。
	if normalized, folded := utils.NormalizeMac(mac); folded {
		global.LOG.Warn(fmt.Sprintf(
			"MAC 配置为区间写法 %s，已自动取第一个：%s；若认证失败请改为区间内的另一个地址",
			mac, normalized))
		mac = normalized
	}
	// 验证参数
	if !utils.CheckUserID(uid) {
		return nil, ErrUserID
	}
	if !utils.CheckSNCode(sn) {
		return nil, ErrSNCode
	}
	if !utils.CheckMacAddressV1(mac) {
		return nil, ErrMacAddr
	}
	if !utils.CheckIPv4Address(ip) {
		return nil, ErrIPAddr
	}

	c := &Client{
		jsVM: jsvm.New(),
		baseConfig: baseConfig{
			userID:           uid,
			sn:               sn,
			macAddr:          mac,
			ip:               ip,
			userAgent:        "webkit;Resolution(PAL,720P,1080P,2106P,4K)",
			pre4kLogAuthAddr: "222.68.208.73:7001",
			stbType:          "B860A",
		},
	}
	for _, opt := range options {
		opt(c)
	}
	c.httpClient = http_client.NewHttpClient(http_client.WithUserAgent(c.userAgent))
	c.UID = uid
	err := c.fetchAuthInfoFormDB()
	return c, err
}

func NewGlobalClient(uid, sn, mac, ip string, options ...ClientOption) (client *Client, err error) {
	if globalClient == nil {
		globalClient, err = NewClient(uid, sn, mac, ip, options...)
		if err != nil {
			return nil, err
		}
	}
	return globalClient, nil
}

func GetGlobalClient() *Client {
	return globalClient
}

// Status 返回认证会话的只读快照，供 /api/health 展示。
// 这里只读内存中的认证信息，不发起网络请求（避免状态接口给专网增加负担）。
func (c *Client) Status() (uid string, updatedAt time.Time, epgHost string, hasSession bool) {
	if c == nil {
		return "", time.Time{}, "", false
	}
	return c.AuthInfo.UID,
		c.AuthInfo.UpdatedAt,
		c.AuthInfo.EPGHostUrl,
		c.AuthInfo.JSESSIONID != ""
}

// EpgHost 返回 EPG 门户的 host:port（可能为空字符串）
func (c *Client) EpgHost() string {
	if c == nil || c.AuthInfo.EPGHostUrl == "" {
		return ""
	}
	u, err := url.Parse(c.AuthInfo.EPGHostUrl)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Host
}
