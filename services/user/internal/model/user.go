// Package model 定义用户服务的持久化模型与对外 DTO。
package model

import "time"

// 角色
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// 用户状态
const (
	StatusDisabled int8 = 0
	StatusActive   int8 = 1
)

// User 对应 users 表。
// 注意：PasswordHash 带 json:"-"，即使误把整个结构体丢给 JSON 也不会泄漏哈希。
type User struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Username     string    `gorm:"column:username;size:32"`
	Email        string    `gorm:"column:email;size:128"`
	PasswordHash string    `gorm:"column:password_hash;size:72" json:"-"`
	Nickname     string    `gorm:"column:nickname;size:32"`
	Role         string    `gorm:"column:role;size:16"`
	Status       int8      `gorm:"column:status"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

// TableName 显式指定表名，避免 GORM 复数化规则带来的意外。
func (User) TableName() string { return "users" }

// IsActive 账号是否可用。
func (u *User) IsActive() bool { return u.Status == StatusActive }

// Public 对外展示用的用户信息，不含密码哈希。
type Public struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Nickname  string    `json:"nickname"`
	Role      string    `json:"role"`
	Status    int8      `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ToPublic 转换成对外结构。
func (u *User) ToPublic() Public {
	return Public{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Nickname:  u.Nickname,
		Role:      u.Role,
		Status:    u.Status,
		CreatedAt: u.CreatedAt,
	}
}

// ToPublicList 批量转换。
func ToPublicList(users []User) []Public {
	out := make([]Public, 0, len(users))
	for i := range users {
		out = append(out, users[i].ToPublic())
	}
	return out
}
