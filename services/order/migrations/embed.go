// Package migrations 订单服务的建表脚本。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
