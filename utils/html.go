package utils

import (
	"bytes"
	"fmt"
	"iptv-spider-sh/global"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func CreateHtmlDocByBytes(uri string, resp []byte) *goquery.Document {
	if len(bytes.TrimSpace(resp)) == 0 {
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp))
	if err != nil {
		global.LOG.Error(err.Error())
		return nil
	}
	if doc == nil {
		return nil
	}
	// 只有解析成功才写 Url；url.Parse 失败时会返回 nil，
	// 原实现先写 doc.Url（err != nil 时 doc 可能是 nil）再判 err，会空指针 panic。
	if u, uErr := url.Parse(uri); uErr == nil {
		doc.Url = u
	}
	return doc
}

// GetFromParamByHtml 解析表单参数。
//
// d 为 nil（响应体为空 / 认证过程中断）时直接返回空值，
// 原实现会 d.Find(...) 空指针 panic。
// 注意：任何返回路径上的 formMap 都保证非 nil —— 原实现在找不到表单时返回 nil map，
// 调用方紧接着的 formMap["authenticator"] = ... / formMap["stbtype"] = ...
// 会命中 "assignment to entry in nil map" panic。uri 为空即表示解析失败。
func GetFromParamByHtml(d *goquery.Document, sec ...string) (uri, method string, formMap map[string]string) {
	formMap = map[string]string{}
	if d == nil {
		return "", "", formMap
	}
	s := "form"
	if len(sec) == 1 {
		s = sec[0]
	}
	nodes := d.Find(s)
	// 处理Form表单
	if len(nodes.Nodes) != 1 {
		return "", "", formMap
	}
	form := nodes.First()
	uri = form.AttrOr("action", "")
	if !strings.HasPrefix(uri, "http") {
		// d.Url 可能为 nil（解析 uri 失败时），此时无法拼出绝对地址
		if d.Url == nil {
			return "", "", formMap
		}
		u := fmt.Sprintf("%s://%s", d.Url.Scheme, d.Url.Host)
		if strings.HasPrefix(uri, "/") {
			u += uri
		} else {
			i := strings.LastIndex(d.Url.Path, "/")
			if i > 0 {
				u = fmt.Sprintf("%s%s/%s", u, d.Url.Path[:i], uri)
			}
		}
		uri = u
	}
	method = form.AttrOr("method", "get")
	child := form.Children()
	child.Each(func(_ int, s *goquery.Selection) {
		k := s.AttrOr("name", "")
		v := s.AttrOr("value", "")
		formMap[k] = v
	})
	return
}

func GetScriptsFormHtml(d *goquery.Document) []string {
	var scriptsArr []string
	if d == nil {
		return scriptsArr
	}
	scripts := d.Find("script")
	scripts.Each(func(_ int, s *goquery.Selection) {
		if len(s.Nodes) == 0 {
			return
		}
		c := s.Nodes[0].FirstChild
		if c == nil {
			return
		}
		scriptsArr = append(scriptsArr, c.Data)
	})
	return scriptsArr
}
