// Package logo 嵌入仓库中的频道台标图片。
//
// 背景：M3U 与 XMLTV 里的 tvg-logo 一直指向 http://<host>:8888/logo/xxx.png，
// 但原实现没有任何静态文件服务，这些地址全部 404（管理面板里也看不到图标）。
// 用 embed 嵌入后，二进制自包含，无需额外部署静态目录。
package logo

import "embed"

// FS 内嵌的台标文件集合（根目录即 logo/，访问路径为 /logo/<文件名>）
//
//go:embed *.png
var FS embed.FS
