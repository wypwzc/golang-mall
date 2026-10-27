// Package migrations 通过 go:embed 把 SQL 文件打进二进制，
// 部署时不需要额外拷贝 sql 目录。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
