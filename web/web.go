// Package web 嵌入管理面板页面，使二进制自包含（无需额外静态文件）。
package web

import _ "embed"

//go:embed status.html
var StatusHTML []byte
