// Package repository 数据访问层：只负责和数据库打交道，不含业务判断。
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"go-mall/pkg/apperr"
	"go-mall/services/user/internal/model"
)

// UserRepository 用户数据访问。
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建仓库。db 是启动时创建的连接池，全局复用。
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create 插入新用户。用户名/邮箱唯一键冲突时返回 409。
func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		if isDuplicateKey(err) {
			return apperr.Conflict.WithMessage("用户名或邮箱已被注册").WithCause(err)
		}
		return apperr.Internal.WithCause(err)
	}
	return nil
}

// ByID 按主键查询。
func (r *UserRepository) ByID(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if err != nil {
		return nil, wrapFindErr(err)
	}
	return &u, nil
}

// ByUsername 按登录名查询，登录时使用。
func (r *UserRepository) ByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error
	if err != nil {
		return nil, wrapFindErr(err)
	}
	return &u, nil
}

// ByEmail 按邮箱查询，注册时查重。
func (r *UserRepository) ByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error
	if err != nil {
		return nil, wrapFindErr(err)
	}
	return &u, nil
}

// UpdateProfile 更新昵称和邮箱，返回更新后的用户。
func (r *UserRepository) UpdateProfile(ctx context.Context, id int64, nickname, email string) (*model.User, error) {
	u, err := r.ByID(ctx, id)
	if err != nil {
		return nil, err
	}

	err = r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"nickname":   nickname,
			"email":      email,
			"updated_at": time.Now().UTC(),
		}).Error
	if err != nil {
		if isDuplicateKey(err) {
			return nil, apperr.Conflict.WithMessage("该邮箱已被使用").WithCause(err)
		}
		return nil, apperr.Internal.WithCause(err)
	}

	u.Nickname = nickname
	u.Email = email
	return u, nil
}

// UpdatePassword 更新密码哈希。
func (r *UserRepository) UpdatePassword(ctx context.Context, id int64, hash string) error {
	return r.updateOne(ctx, id, map[string]any{"password_hash": hash})
}

// UpdateStatus 启用/禁用账号。
func (r *UserRepository) UpdateStatus(ctx context.Context, id int64, status int8) error {
	return r.updateOne(ctx, id, map[string]any{"status": status})
}

// UpdateRole 修改角色。
func (r *UserRepository) UpdateRole(ctx context.Context, id int64, role string) error {
	return r.updateOne(ctx, id, map[string]any{"role": role})
}

// updateOne 先确认记录存在再更新。
//
// 为什么不直接看 RowsAffected：MySQL 在"新值和旧值相同"时受影响行数也是 0，
// 那样会把正常请求误判成 404。
func (r *UserRepository) updateOne(ctx context.Context, id int64, fields map[string]any) error {
	if _, err := r.ByID(ctx, id); err != nil {
		return err
	}
	fields["updated_at"] = time.Now().UTC()

	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return apperr.Internal.WithCause(err)
	}
	return nil
}

// ListFilter 管理端查询条件。
type ListFilter struct {
	Page     int
	PageSize int
	Keyword  string // 匹配用户名/邮箱/昵称
	Role     string
	Status   *int8
}

// List 分页查询用户，返回列表与总数。
func (r *UserRepository) List(ctx context.Context, f ListFilter) ([]model.User, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.User{})

	if f.Keyword != "" {
		// 用参数占位符，绝不用字符串拼接，避免 SQL 注入
		like := "%" + f.Keyword + "%"
		q = q.Where("username LIKE ? OR email LIKE ? OR nickname LIKE ?", like, like, like)
	}
	if f.Role != "" {
		q = q.Where("role = ?", f.Role)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, apperr.Internal.WithCause(err)
	}

	var users []model.User
	err := q.Order("id DESC").
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		Find(&users).Error
	if err != nil {
		return nil, 0, apperr.Internal.WithCause(err)
	}
	return users, total, nil
}

// CountByRole 统计某角色数量，用于防止把最后一个管理员降级。
func (r *UserRepository) CountByRole(ctx context.Context, role string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("role = ?", role).Count(&n).Error
	if err != nil {
		return 0, apperr.Internal.WithCause(err)
	}
	return n, nil
}

// wrapFindErr 把 GORM 的"未找到"翻译成业务错误。
func wrapFindErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound.WithMessage("用户不存在")
	}
	return apperr.Internal.WithCause(err)
}

// isDuplicateKey 判断是否是 MySQL 唯一键冲突（错误码 1062）。
func isDuplicateKey(err error) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1062
	}
	return strings.Contains(err.Error(), "Duplicate entry")
}
