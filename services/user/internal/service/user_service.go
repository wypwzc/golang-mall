// Package service 业务逻辑层：校验、权限、事务边界都在这里，Handler 只做参数绑定。
package service

import (
	"context"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"go-mall/pkg/apperr"
	"go-mall/pkg/jwt"
	"go-mall/pkg/password"
	"go-mall/pkg/ratelimit"
	"go-mall/services/user/internal/model"
	"go-mall/services/user/internal/repository"
)

// UserRepo 是仓库的抽象。抽接口的目的只有一个：让业务逻辑能在没有数据库的情况下做单元测试。
type UserRepo interface {
	Create(ctx context.Context, u *model.User) error
	ByID(ctx context.Context, id int64) (*model.User, error)
	ByUsername(ctx context.Context, username string) (*model.User, error)
	ByEmail(ctx context.Context, email string) (*model.User, error)
	UpdateProfile(ctx context.Context, id int64, nickname, email string) (*model.User, error)
	UpdatePassword(ctx context.Context, id int64, hash string) error
	UpdateStatus(ctx context.Context, id int64, status int8) error
	UpdateRole(ctx context.Context, id int64, role string) error
	List(ctx context.Context, f repository.ListFilter) ([]model.User, int64, error)
	CountByRole(ctx context.Context, role string) (int64, error)
}

// Service 用户服务。
type Service struct {
	repo    UserRepo
	tokens  *jwt.Manager
	limiter ratelimit.Limiter
	log     *slog.Logger
}

// New 创建服务。limiter 在本地是内存实现，上了 Redis 换成 Redis 实现即可，本函数不用改。
func New(repo UserRepo, tokens *jwt.Manager, limiter ratelimit.Limiter, log *slog.Logger) *Service {
	return &Service{repo: repo, tokens: tokens, limiter: limiter, log: log}
}

// ---------- 入参 ----------

// RegisterInput 注册参数。
type RegisterInput struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Nickname string `json:"nickname"`
}

// LoginInput 登录参数。
type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// UpdateProfileInput 修改资料。
type UpdateProfileInput struct {
	Nickname string `json:"nickname"`
	Email    string `json:"email" binding:"required"`
}

// ChangePasswordInput 修改密码。
type ChangePasswordInput struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// LoginResult 登录结果。
type LoginResult struct {
	AccessToken string       `json:"access_token"`
	ExpiresAt   time.Time    `json:"expires_at"`
	User        model.Public `json:"user"`
}

// ---------- 注册 / 登录 ----------

// Register 注册新用户。默认角色 user、状态正常。
func (s *Service) Register(ctx context.Context, in RegisterInput) (*model.Public, error) {
	username := strings.TrimSpace(in.Username)
	email := strings.ToLower(strings.TrimSpace(in.Email))
	nickname := strings.TrimSpace(in.Nickname)

	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	if err := validatePassword(in.Password); err != nil {
		return nil, err
	}
	if nickname == "" {
		nickname = username
	}

	// 先查一次给出友好提示；真正的并发兜底是数据库唯一索引
	if _, err := s.repo.ByUsername(ctx, username); err == nil {
		return nil, apperr.Conflict.WithMessage("用户名已被占用")
	}
	if _, err := s.repo.ByEmail(ctx, email); err == nil {
		return nil, apperr.Conflict.WithMessage("邮箱已被注册")
	}

	hash, err := password.Hash(in.Password)
	if err != nil {
		return nil, apperr.Internal.WithCause(err)
	}

	u := &model.User{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Nickname:     nickname,
		Role:         model.RoleUser,
		Status:       model.StatusActive,
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}

	pub := u.ToPublic()
	return &pub, nil
}

// Login 登录：先限流，再验密码，成功后清零计数并签发 JWT。
func (s *Service) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" || in.Password == "" {
		return nil, apperr.InvalidParam.WithMessage("用户名和密码不能为空")
	}

	// 限流键带上用户名：同一账号连续失败才锁定，不影响其他用户
	key := "login:fail:" + strings.ToLower(username)
	allowed, retryAfter, err := s.limiter.Allow(ctx, key)
	if err != nil {
		// 限流器故障不应阻断登录，记录日志后放行
		s.log.Warn("限流器异常，本次跳过限流", "err", err)
	} else if !allowed {
		minutes := int(retryAfter.Minutes()) + 1
		return nil, apperr.TooManyRequests.WithMessage("登录失败次数过多，请 %d 分钟后再试", minutes)
	}

	u, err := s.repo.ByUsername(ctx, username)
	if err != nil {
		// 不区分"用户不存在"和"密码错误"，避免被用来枚举账号
		return nil, apperr.Unauthorized.WithMessage("用户名或密码错误")
	}
	if !u.IsActive() {
		return nil, apperr.Forbidden.WithMessage("账号已被禁用，请联系管理员")
	}
	if !password.Verify(u.PasswordHash, in.Password) {
		s.log.Warn("登录失败：密码错误", "username", username)
		return nil, apperr.Unauthorized.WithMessage("用户名或密码错误")
	}

	// 登录成功，清空失败计数
	if err := s.limiter.Reset(ctx, key); err != nil {
		s.log.Warn("清除登录失败计数失败", "err", err)
	}

	token, expireAt, err := s.tokens.Sign(u.ID, u.Role)
	if err != nil {
		return nil, err
	}

	return &LoginResult{AccessToken: token, ExpiresAt: expireAt, User: u.ToPublic()}, nil
}

// ---------- 当前用户 ----------

// Me 查询当前登录用户。
func (s *Service) Me(ctx context.Context, id int64) (*model.Public, error) {
	u, err := s.repo.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !u.IsActive() {
		return nil, apperr.Forbidden.WithMessage("账号已被禁用")
	}
	pub := u.ToPublic()
	return &pub, nil
}

// UpdateProfile 修改昵称与邮箱。
func (s *Service) UpdateProfile(ctx context.Context, id int64, in UpdateProfileInput) (*model.Public, error) {
	nickname := strings.TrimSpace(in.Nickname)
	email := strings.ToLower(strings.TrimSpace(in.Email))

	if len([]rune(nickname)) > 32 {
		return nil, apperr.InvalidParam.WithMessage("昵称最多 32 个字符")
	}
	if err := validateEmail(email); err != nil {
		return nil, err
	}

	u, err := s.repo.UpdateProfile(ctx, id, nickname, email)
	if err != nil {
		return nil, err
	}
	pub := u.ToPublic()
	return &pub, nil
}

// ChangePassword 修改密码，必须校验旧密码。
func (s *Service) ChangePassword(ctx context.Context, id int64, in ChangePasswordInput) error {
	u, err := s.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	if !password.Verify(u.PasswordHash, in.OldPassword) {
		return apperr.Unauthorized.WithMessage("原密码不正确")
	}
	if err := validatePassword(in.NewPassword); err != nil {
		return err
	}
	if in.OldPassword == in.NewPassword {
		return apperr.InvalidParam.WithMessage("新密码不能与原密码相同")
	}

	hash, err := password.Hash(in.NewPassword)
	if err != nil {
		return apperr.Internal.WithCause(err)
	}
	return s.repo.UpdatePassword(ctx, id, hash)
}

// ---------- 管理端 ----------

// AdminList 用户分页查询。
func (s *Service) AdminList(ctx context.Context, page, pageSize int, keyword, role string, status *int8) ([]model.Public, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	if role != "" && role != model.RoleUser && role != model.RoleAdmin {
		return nil, 0, apperr.InvalidParam.WithMessage("角色只能是 user 或 admin")
	}

	users, total, err := s.repo.List(ctx, repository.ListFilter{
		Page:     page,
		PageSize: pageSize,
		Keyword:  strings.TrimSpace(keyword),
		Role:     role,
		Status:   status,
	})
	if err != nil {
		return nil, 0, err
	}
	return model.ToPublicList(users), total, nil
}

// AdminSetStatus 启用/禁用账号。管理员不能禁用自己。
func (s *Service) AdminSetStatus(ctx context.Context, operatorID, targetID int64, status int8) error {
	if operatorID == targetID {
		return apperr.Forbidden.WithMessage("不能修改自己的账号状态")
	}
	if status != model.StatusActive && status != model.StatusDisabled {
		return apperr.InvalidParam.WithMessage("状态只能是 0 或 1")
	}
	return s.repo.UpdateStatus(ctx, targetID, status)
}

// AdminSetRole 修改角色。管理员不能降级自己，也不能把最后一个管理员降级。
func (s *Service) AdminSetRole(ctx context.Context, operatorID, targetID int64, role string) error {
	if role != model.RoleUser && role != model.RoleAdmin {
		return apperr.InvalidParam.WithMessage("角色只能是 user 或 admin")
	}
	if operatorID == targetID {
		return apperr.Forbidden.WithMessage("不能修改自己的角色")
	}
	if role == model.RoleUser {
		n, err := s.repo.CountByRole(ctx, model.RoleAdmin)
		if err != nil {
			return err
		}
		if n <= 1 {
			return apperr.Conflict.WithMessage("系统至少需要保留一个管理员")
		}
	}
	return s.repo.UpdateRole(ctx, targetID, role)
}

// ---------- 校验工具 ----------

func validateUsername(name string) error {
	n := len([]rune(name))
	if n < 3 || n > 32 {
		return apperr.InvalidParam.WithMessage("用户名长度需在 3-32 个字符之间")
	}
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
		if !ok {
			return apperr.InvalidParam.WithMessage("用户名只能包含字母、数字、下划线和短横线")
		}
	}
	return nil
}

func validateEmail(email string) error {
	if email == "" || len(email) > 128 {
		return apperr.InvalidParam.WithMessage("邮箱格式不正确")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return apperr.InvalidParam.WithMessage("邮箱格式不正确")
	}
	return nil
}

func validatePassword(pwd string) error {
	if len(pwd) < 8 || len(pwd) > 72 {
		return apperr.InvalidParam.WithMessage("密码长度需在 8-72 位之间")
	}
	var hasLetter, hasDigit bool
	for _, r := range pwd {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return apperr.InvalidParam.WithMessage("密码必须同时包含字母和数字")
	}
	return nil
}

// normalizePage 统一分页参数，防止 pageSize 过大拖垮数据库。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
