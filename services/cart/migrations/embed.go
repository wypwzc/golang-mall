// Package migrations 把建表 SQL 打进二进制，服务启动时自动执行。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
