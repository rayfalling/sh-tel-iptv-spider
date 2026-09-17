package http_client

import (
	"fmt"
	"iptv-spider-sh/global"
	"net"
	"net/http"
	"strings"

	"github.com/go-resty/resty/v2"
)

type HttpClient struct {
	client    *resty.Client
	userAgent string
	resp      *resty.Response
}

// Request 发送请求。
// 注意：网络层失败（连接被拒/DNS/超时）时 resty 返回 nil 响应，
// 这里明确置 c.resp=nil 并直接返回；原实现在失败后仍调用 c.resp.Body()，
// 会空指针 panic 导致整个进程退出。
func (c *HttpClient) Request(uri, method string, form map[string]string) *HttpClient {
	r := c.client.R()
	method = strings.ToUpper(method)
	switch method {
	case "GET":
		r.SetQueryParams(form)
	case "POST":
		r.SetFormData(form)
	}
	global.LOG.Debug(fmt.Sprintf("%s %s Data: %v", method, uri, form))
	var err error
	c.resp, err = r.Execute(method, uri)
	if err != nil {
		global.LOG.Error(fmt.Sprintf("%s %s 请求失败: %s", method, uri, err.Error()))
		c.resp = nil
		return c
	}
	global.LOG.Debug(fmt.Sprintf("Resp Body: %s", string(c.resp.Body())))
	return c
}

// OK 本次请求是否成功拿到响应
func (c *HttpClient) OK() bool {
	return c.resp != nil
}

// GetResp 可能返回 nil；调用方请先判空，或改用 Header()/StatusCode()
func (c *HttpClient) GetResp() *resty.Response {
	return c.resp
}

// GetRespBytes 请求失败时返回 nil，不 panic
func (c *HttpClient) GetRespBytes() []byte {
	if c.resp == nil {
		return nil
	}
	return c.resp.Body()
}

// Header 安全读取响应头；请求失败时返回空字符串
func (c *HttpClient) Header(key string) string {
	if c.resp == nil || c.resp.RawResponse == nil {
		return ""
	}
	return c.resp.Header().Get(key)
}

// StatusCode 安全读取状态码；请求失败时返回 0
func (c *HttpClient) StatusCode() int {
	if c.resp == nil {
		return 0
	}
	return c.resp.StatusCode()
}

// CookieJar 返回当前 cookie 列表（可能为空）
func (c *HttpClient) CookieJar() []*http.Cookie {
	if c.client == nil {
		return nil
	}
	return c.client.Cookies
}

func NewHttpClient(opts ...HttpClientOption) *HttpClient {
	c := &HttpClient{
		userAgent: "webkit;Resolution(PAL,720P,1080P,2106P,4K)",
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.client == nil {
		c.client = resty.New()
	}
	c.afterAction()
	return c
}

func (c *HttpClient) afterAction() {
	// setUserAgent
	c.client.SetHeader("User-Agent", c.userAgent)
}

func (c *HttpClient) SetCookies(cookies ...*http.Cookie) {
	if len(cookies) <= 0 {
		return
	}
	c.client.SetCookies(cookies)
}

func (c *HttpClient) Cookies() []*http.Cookie {
	return c.client.Cookies
}

type HttpClientOption func(client *HttpClient)

func WithUserAgent(ua string) HttpClientOption {
	return func(c *HttpClient) {
		c.userAgent = ua
	}
}

func WithLocalAddr(addr string) HttpClientOption {
	return func(c *HttpClient) {
		tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			global.LOG.Warn("ResolveTCPAddr failed: " + addr)
		} else {
			global.LOG.Info("ResolveTCPAddr success: " + addr)
		}
		c.client = resty.NewWithLocalAddr(tcpAddr)
	}
}
