// Package migrations 商品服务的数据库迁移脚本。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
