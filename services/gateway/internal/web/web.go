// Package web 把前端静态文件打包进网关二进制。
//
// 前端是纯 HTML + CSS + JS，没有构建步骤，改完重新编译网关即可。
// 用 embed 的好处是部署时只需要一个 exe，不用再带一堆静态文件。
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// all: 前缀表示连以 _ 或 . 开头的文件也一起打包
//
//go:embed all:dist
var files embed.FS

// Handler 返回静态文件处理器，根目录已经剥掉 dist 这一层。
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(files, "dist")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(sub)), nil
}
