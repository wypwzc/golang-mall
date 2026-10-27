// Package migrate 是一个极简的 SQL 迁移器。
//
// 约定：
//   - 每个服务把自己的迁移文件放在 migrations/*.sql，按文件名升序执行；
//   - 已执行的版本记录在 schema_migrations 表，重复启动不会重复执行；
//   - 只做向前迁移（forward-only），不做自动回滚。
//
// 为什么不用现成的迁移库：依赖少、行为透明，30 行就能读懂，
// 对学习项目来说比引入一个框架更合适。
package migrate

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// Run 执行 dir 目录下所有未执行的 .sql 文件。
func Run(db *gorm.DB, fsys fs.FS, dir string) error {
	const createTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(128) NOT NULL PRIMARY KEY,
		applied_at DATETIME(3) NOT NULL
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	if err := db.Exec(createTable).Error; err != nil {
		return fmt.Errorf("创建 schema_migrations 失败: %w", err)
	}

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("读取迁移目录失败: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", name).Scan(&count).Error; err != nil {
			return fmt.Errorf("查询迁移记录失败: %w", err)
		}
		if count > 0 {
			continue
		}

		content, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return fmt.Errorf("读取 %s 失败: %w", name, err)
		}

		// DSN 里开了 multiStatements，一个文件可以包含多条语句。
		// 注意：MySQL 的 DDL 无法回滚，所以这里不做事务包裹。
		if err := db.Exec(string(content)).Error; err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %w", name, err)
		}
		if err := db.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, UTC_TIMESTAMP(3))", name).Error; err != nil {
			return fmt.Errorf("记录迁移 %s 失败: %w", name, err)
		}
	}
	return nil
}
