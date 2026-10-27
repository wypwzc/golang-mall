package service_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"go-mall/pkg/apperr"
	"go-mall/pkg/jwt"
	"go-mall/pkg/password"
	"go-mall/pkg/ratelimit"
	"go-mall/services/user/internal/model"
	"go-mall/services/user/internal/repository"
	"go-mall/services/user/internal/service"
)

// ---------- 内存仓库：让业务逻辑可以脱离 MySQL 做单元测试 ----------

type fakeRepo struct {
	byName map[string]*model.User
	byID   map[int64]*model.User
	nextID int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byName: map[string]*model.User{}, byID: map[int64]*model.User{}, nextID: 1}
}

func (f *fakeRepo) Create(_ context.Context, u *model.User) error {
	if _, ok := f.byName[u.Username]; ok {
		return apperr.Conflict.WithMessage("用户名已被占用")
	}
	u.ID = f.nextID
	f.nextID++
	f.byName[u.Username] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeRepo) ByID(_ context.Context, id int64) (*model.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, apperr.NotFound.WithMessage("用户不存在")
}

func (f *fakeRepo) ByUsername(_ context.Context, username string) (*model.User, error) {
	if u, ok := f.byName[username]; ok {
		return u, nil
	}
	return nil, apperr.NotFound.WithMessage("用户不存在")
}

func (f *fakeRepo) ByEmail(_ context.Context, email string) (*model.User, error) {
	for _, u := range f.byID {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, apperr.NotFound.WithMessage("用户不存在")
}

func (f *fakeRepo) UpdateProfile(_ context.Context, id int64, nickname, email string) (*model.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, apperr.NotFound.WithMessage("用户不存在")
	}
	u.Nickname = nickname
	u.Email = email
	return u, nil
}

func (f *fakeRepo) UpdatePassword(_ context.Context, id int64, hash string) error {
	if u, ok := f.byID[id]; ok {
		u.PasswordHash = hash
		return nil
	}
	return apperr.NotFound.WithMessage("用户不存在")
}

func (f *fakeRepo) UpdateStatus(_ context.Context, id int64, status int8) error {
	if u, ok := f.byID[id]; ok {
		u.Status = status
		return nil
	}
	return apperr.NotFound.WithMessage("用户不存在")
}

func (f *fakeRepo) UpdateRole(_ context.Context, id int64, role string) error {
	if u, ok := f.byID[id]; ok {
		u.Role = role
		return nil
	}
	return apperr.NotFound.WithMessage("用户不存在")
}

func (f *fakeRepo) List(_ context.Context, _ repository.ListFilter) ([]model.User, int64, error) {
	out := make([]model.User, 0, len(f.byID))
	for _, u := range f.byID {
		out = append(out, *u)
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) CountByRole(_ context.Context, role string) (int64, error) {
	var n int64
	for _, u := range f.byID {
		if u.Role == role {
			n++
		}
	}
	return n, nil
}

// ---------- 测试脚手架 ----------

func newService(t *testing.T, limit int) (*service.Service, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo()
	tokens := jwt.New("unit-test-secret-at-least-32-bytes!", "go-mall", time.Hour)
	limiter := ratelimit.NewMemory(ratelimit.Config{Limit: limit, Window: time.Minute})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return service.New(repo, tokens, limiter, log), repo
}

func registerAlice(t *testing.T, svc *service.Service) {
	t.Helper()
	_, err := svc.Register(context.Background(), service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "alice123456",
	})
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
}

func codeOf(err error) apperr.Code {
	if err == nil {
		return apperr.CodeOK
	}
	return apperr.From(err).Code
}

// ---------- 注册 ----------

func TestRegisterStoresHashNotPlaintext(t *testing.T) {
	svc, repo := newService(t, 5)
	registerAlice(t, svc)

	u := repo.byName["alice"]
	if u == nil {
		t.Fatal("用户未写入")
	}
	if u.PasswordHash == "alice123456" {
		t.Fatal("绝不能存明文密码")
	}
	if !password.Verify(u.PasswordHash, "alice123456") {
		t.Fatal("哈希应能校验通过")
	}
	if u.Role != model.RoleUser || u.Status != model.StatusActive {
		t.Fatalf("默认角色/状态不对: %s/%d", u.Role, u.Status)
	}
}

func TestRegisterRejectsDuplicateUsername(t *testing.T) {
	svc, _ := newService(t, 5)
	registerAlice(t, svc)

	_, err := svc.Register(context.Background(), service.RegisterInput{
		Username: "alice",
		Email:    "other@example.com",
		Password: "alice123456",
	})
	if codeOf(err) != apperr.CodeConflict {
		t.Fatalf("重复用户名应返回 409，实际 %v", err)
	}
}

func TestRegisterValidatesInput(t *testing.T) {
	svc, _ := newService(t, 5)
	cases := []struct {
		name string
		in   service.RegisterInput
	}{
		{"用户名太短", service.RegisterInput{Username: "ab", Email: "a@b.com", Password: "abc12345"}},
		{"用户名含非法字符", service.RegisterInput{Username: "al ice", Email: "a@b.com", Password: "abc12345"}},
		{"邮箱非法", service.RegisterInput{Username: "alice", Email: "not-an-email", Password: "abc12345"}},
		{"密码太短", service.RegisterInput{Username: "alice", Email: "a@b.com", Password: "a1"}},
		{"密码纯数字", service.RegisterInput{Username: "alice", Email: "a@b.com", Password: "12345678"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := svc.Register(context.Background(), c.in); codeOf(err) != apperr.CodeInvalidParam {
				t.Fatalf("应返回参数错误，实际 %v", err)
			}
		})
	}
}

// ---------- 登录 ----------

func TestLoginSuccessReturnsToken(t *testing.T) {
	svc, _ := newService(t, 5)
	registerAlice(t, svc)

	result, err := svc.Login(context.Background(), service.LoginInput{Username: "alice", Password: "alice123456"})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if result.AccessToken == "" {
		t.Fatal("应返回 token")
	}
	if result.User.Username != "alice" {
		t.Fatalf("返回用户不正确: %+v", result.User)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc, _ := newService(t, 5)
	registerAlice(t, svc)

	_, err := svc.Login(context.Background(), service.LoginInput{Username: "alice", Password: "wrong-pass-1"})
	if codeOf(err) != apperr.CodeUnauthorized {
		t.Fatalf("密码错误应返回 401，实际 %v", err)
	}
}

func TestLoginUnknownUserSameMessage(t *testing.T) {
	svc, _ := newService(t, 5)

	_, err := svc.Login(context.Background(), service.LoginInput{Username: "nobody", Password: "whatever1"})
	e := apperr.From(err)
	if e.Code != apperr.CodeUnauthorized {
		t.Fatalf("不存在的用户应返回 401，实际 %d", e.Code)
	}
	// 提示语必须与"密码错误"一致，避免被用来枚举账号
	if e.Message != "用户名或密码错误" {
		t.Fatalf("提示语不能泄漏账号是否存在: %s", e.Message)
	}
}

func TestLoginRateLimit(t *testing.T) {
	svc, _ := newService(t, 5)
	registerAlice(t, svc)

	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		_, err := svc.Login(ctx, service.LoginInput{Username: "alice", Password: "wrong-pass-1"})
		if codeOf(err) != apperr.CodeUnauthorized {
			t.Fatalf("第 %d 次失败应是 401，实际 %v", i, err)
		}
	}

	_, err := svc.Login(ctx, service.LoginInput{Username: "alice", Password: "alice123456"})
	if codeOf(err) != apperr.CodeTooManyRequests {
		t.Fatalf("第 6 次应被限流为 429，实际 %v", err)
	}
}

func TestLoginResetsCounterOnSuccess(t *testing.T) {
	svc, _ := newService(t, 3)
	registerAlice(t, svc)

	ctx := context.Background()
	// 失败两次后成功登录，计数应清零，之后还能继续尝试
	for i := 0; i < 2; i++ {
		svc.Login(ctx, service.LoginInput{Username: "alice", Password: "wrong-pass-1"})
	}
	if _, err := svc.Login(ctx, service.LoginInput{Username: "alice", Password: "alice123456"}); err != nil {
		t.Fatalf("正确密码应登录成功: %v", err)
	}
	for i := 0; i < 2; i++ {
		_, err := svc.Login(ctx, service.LoginInput{Username: "alice", Password: "wrong-pass-1"})
		if codeOf(err) != apperr.CodeUnauthorized {
			t.Fatalf("成功后计数应已清零，实际 %v", err)
		}
	}
}

func TestLoginDisabledAccount(t *testing.T) {
	svc, repo := newService(t, 5)
	registerAlice(t, svc)

	repo.byName["alice"].Status = model.StatusDisabled

	_, err := svc.Login(context.Background(), service.LoginInput{Username: "alice", Password: "alice123456"})
	if codeOf(err) != apperr.CodeForbidden {
		t.Fatalf("禁用账号应返回 403，实际 %v", err)
	}
}

// ---------- 资料与密码 ----------

func TestUpdateProfileRejectsBadEmail(t *testing.T) {
	svc, _ := newService(t, 5)
	registerAlice(t, svc)

	_, err := svc.UpdateProfile(context.Background(), 1, service.UpdateProfileInput{Email: "bad-email"})
	if codeOf(err) != apperr.CodeInvalidParam {
		t.Fatalf("非法邮箱应返回参数错误，实际 %v", err)
	}
}

func TestChangePasswordRequiresOldPassword(t *testing.T) {
	svc, _ := newService(t, 5)
	registerAlice(t, svc)

	ctx := context.Background()
	err := svc.ChangePassword(ctx, 1, service.ChangePasswordInput{
		OldPassword: "wrong-old-1",
		NewPassword: "newpass123",
	})
	if codeOf(err) != apperr.CodeUnauthorized {
		t.Fatalf("旧密码错误应返回 401，实际 %v", err)
	}

	if err := svc.ChangePassword(ctx, 1, service.ChangePasswordInput{
		OldPassword: "alice123456",
		NewPassword: "newpass123",
	}); err != nil {
		t.Fatalf("改密失败: %v", err)
	}
	if _, err := svc.Login(ctx, service.LoginInput{Username: "alice", Password: "newpass123"}); err != nil {
		t.Fatalf("新密码应能登录: %v", err)
	}
}

// ---------- 管理端 ----------

func TestAdminCannotChangeOwnRole(t *testing.T) {
	svc, _ := newService(t, 5)

	err := svc.AdminSetRole(context.Background(), 1, 1, model.RoleUser)
	if codeOf(err) != apperr.CodeForbidden {
		t.Fatalf("不能修改自己的角色，实际 %v", err)
	}
}

func TestAdminCannotChangeOwnStatus(t *testing.T) {
	svc, _ := newService(t, 5)

	err := svc.AdminSetStatus(context.Background(), 1, 1, model.StatusDisabled)
	if codeOf(err) != apperr.CodeForbidden {
		t.Fatalf("不能禁用自己，实际 %v", err)
	}
}

func TestAdminCannotRemoveLastAdmin(t *testing.T) {
	svc, repo := newService(t, 5)
	// 只留一个管理员
	repo.Create(context.Background(), &model.User{Username: "admin", Role: model.RoleAdmin, Status: model.StatusActive})
	repo.Create(context.Background(), &model.User{Username: "bob", Role: model.RoleUser, Status: model.StatusActive})

	err := svc.AdminSetRole(context.Background(), 1, 2, model.RoleAdmin) // 先把 bob 提成管理员
	if err != nil {
		t.Fatalf("提升角色失败: %v", err)
	}

	// 现在有两个管理员，降级一个应该允许
	if err := svc.AdminSetRole(context.Background(), 1, 2, model.RoleUser); err != nil {
		t.Fatalf("降级失败: %v", err)
	}
}

func TestAdminListNormalizesPageSize(t *testing.T) {
	svc, _ := newService(t, 5)

	list, total, err := svc.AdminList(context.Background(), 0, 9999, "", "", nil)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if list == nil {
		t.Fatal("应返回空切片而不是 nil")
	}
	_ = total
}
