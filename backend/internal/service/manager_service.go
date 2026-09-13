package service

import (
	"context"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	// ErrManagerScopeForbidden 目标用户不在该经理负责的部门范围内。
	ErrManagerScopeForbidden = infraerrors.Forbidden("MANAGER_SCOPE_FORBIDDEN", "target user is outside your department scope")
	// ErrManagerRoleRequired 操作者不是部门经理。
	ErrManagerRoleRequired = infraerrors.Forbidden("MANAGER_ROLE_REQUIRED", "department manager role required")
)

// ManagerService 实现部门经理范围的查询与额度重置。
// 所有列表/进度/重置操作都在后端按操作者的负责部门做范围校验，
// 不信任 URL 中的用户或订阅 ID。
type ManagerService struct {
	scopeRepo        ManagerScopeRepository
	userRepo         UserRepository
	subRepo          UserSubscriptionRepository
	quotaResetter    func(ctx context.Context, subscriptionID int64, resetDaily, resetWeekly, resetMonthly bool) (*UserSubscription, error)
	progressProvider func(ctx context.Context, subscriptionID int64) (*SubscriptionProgress, error)
	now              func() time.Time
}

func NewManagerService(
	scopeRepo ManagerScopeRepository,
	userRepo UserRepository,
	subRepo UserSubscriptionRepository,
) *ManagerService {
	return &ManagerService{
		scopeRepo: scopeRepo,
		userRepo:  userRepo,
		subRepo:   subRepo,
		now:       time.Now,
	}
}

// SetQuotaResetter 注入订阅额度重置实现（SubscriptionService.AdminResetQuota），
// 由 Wire provider 调用，避免构造函数循环依赖。
func (s *ManagerService) SetQuotaResetter(resetter func(ctx context.Context, subscriptionID int64, resetDaily, resetWeekly, resetMonthly bool) (*UserSubscription, error)) {
	s.quotaResetter = resetter
}

// ListDepartments 返回本地部门目录（管理员配置控件使用）。
func (s *ManagerService) ListDepartments(ctx context.Context) ([]DingTalkDepartment, error) {
	return s.scopeRepo.ListDepartments(ctx)
}

// UpsertDepartment 写入/刷新部门目录记录（钉钉登录同步时调用）。
func (s *ManagerService) UpsertDepartment(ctx context.Context, department *DingTalkDepartment) error {
	return s.scopeRepo.UpsertDepartment(ctx, department)
}

// SetUserPrimaryDept 持久化用户主部门 ID（0 表示清除）。
func (s *ManagerService) SetUserPrimaryDept(ctx context.Context, userID, deptID int64) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	fields := UserUpdateFields{PrimaryDeptID: true}
	if deptID > 0 {
		user.PrimaryDeptID = &deptID
	} else {
		user.PrimaryDeptID = nil
	}
	return s.userRepo.Update(ctx, user, fields)
}

// ListManagerDepartments 返回经理负责的部门列表。
func (s *ManagerService) ListManagerDepartments(ctx context.Context, managerUserID int64) ([]DingTalkDepartment, error) {
	return s.scopeRepo.ListManagerDepartments(ctx, managerUserID)
}

// ReplaceManagerDepartments 管理员重设某经理负责的部门集合（nil 表示清空）。
func (s *ManagerService) ReplaceManagerDepartments(ctx context.Context, managerUserID int64, deptIDs []int64) error {
	user, err := s.userRepo.GetByID(ctx, managerUserID)
	if err != nil {
		return err
	}
	if user.Role != RoleManager {
		return fmt.Errorf("user %d is not a department manager", managerUserID)
	}
	return s.scopeRepo.ReplaceManagerDepartments(ctx, managerUserID, deptIDs)
}

// ListScopedMembers 返回经理负责部门内、且已有订阅记录的成员用户 ID（分页）。
func (s *ManagerService) ListScopedMembers(ctx context.Context, managerUserID int64, params pagination.PaginationParams) ([]int64, *pagination.PaginationResult, error) {
	return s.scopeRepo.ListManagerUserIDs(ctx, managerUserID, params)
}

// RequireUserInScope 校验目标用户属于经理负责的部门，否则返回 ErrManagerScopeForbidden。
func (s *ManagerService) RequireUserInScope(ctx context.Context, managerUserID, userID int64) error {
	ok, err := s.scopeRepo.IsUserInManagerScope(ctx, managerUserID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrManagerScopeForbidden
	}
	return nil
}

// ManagerMemberSubscription 是经理视图下的一条成员订阅。
type ManagerMemberSubscription struct {
	UserSubscription
	ManagerUserID int64
}

// ListMemberSubscriptions 返回目标成员的全部订阅（不限于有效订阅），先做范围校验。
func (s *ManagerService) ListMemberSubscriptions(ctx context.Context, managerUserID, userID int64) ([]UserSubscription, error) {
	if err := s.RequireUserInScope(ctx, managerUserID, userID); err != nil {
		return nil, err
	}
	return s.subRepo.ListByUserID(ctx, userID)
}

// ResetMemberQuota 在范围校验后复用订阅服务的额度重置逻辑（日/周/月窗口）。
// subscriptionID 属于范围外用户时返回 ErrManagerScopeForbidden。
func (s *ManagerService) ResetMemberQuota(ctx context.Context, managerUserID, subscriptionID int64, resetDaily, resetWeekly, resetMonthly bool) (*UserSubscription, error) {
	sub, err := s.subRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if err := s.RequireUserInScope(ctx, managerUserID, sub.UserID); err != nil {
		return nil, err
	}
	return s.resetQuota(ctx, sub, resetDaily, resetWeekly, resetMonthly)
}

// GetUserByID 返回成员用户信息（经理视图，不做敏感字段过滤——由 DTO 层裁剪）。
func (s *ManagerService) GetUserByID(ctx context.Context, userID int64) (*User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

// GetSubscriptionProgressInScope 在范围校验后返回订阅使用进度。
func (s *ManagerService) GetSubscriptionProgressInScope(ctx context.Context, managerUserID, subscriptionID int64) (*SubscriptionProgress, error) {
	sub, err := s.subRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if err := s.RequireUserInScope(ctx, managerUserID, sub.UserID); err != nil {
		return nil, err
	}
	if s.progressProvider == nil {
		return nil, ErrInvalidInput
	}
	return s.progressProvider(ctx, subscriptionID)
}

// SetProgressProvider 注入订阅进度查询实现（SubscriptionService.GetSubscriptionProgress）。
func (s *ManagerService) SetProgressProvider(provider func(ctx context.Context, subscriptionID int64) (*SubscriptionProgress, error)) {
	s.progressProvider = provider
}

// resetQuota 经理额度重置统一走订阅服务的 AdminResetQuota（含缓存失效）。
func (s *ManagerService) resetQuota(ctx context.Context, sub *UserSubscription, resetDaily, resetWeekly, resetMonthly bool) (*UserSubscription, error) {
	if s.quotaResetter == nil {
		return nil, ErrInvalidInput
	}
	return s.quotaResetter(ctx, sub.ID, resetDaily, resetWeekly, resetMonthly)
}
