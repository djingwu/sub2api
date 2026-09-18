package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
	deptGroupMap     func(ctx context.Context) (map[int64]string, error)
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

// ManagerDepartmentOption 是管理员为经理分配范围时的可选部门。
// 一个订阅分组（settings.dingtalk_dept_group_map 的取值）通常对应多个钉钉部门，
// 因此用 GroupName 归类展示，用 Path 展示到末级的层级路径。
type ManagerDepartmentOption struct {
	DeptID    int64
	ParentID  int64
	Name      string
	GroupName string
	Path      []string
	IsActive  bool
	// Synced 表示该部门已出现在本地钉钉部门目录缓存中（名称/层级可信）。
	Synced bool
}

// ListDepartments 返回“订阅分组对应的部门”目录（管理员配置控件使用）：
// 数据源是设置项 dingtalk_dept_group_map（dept_id → 订阅分组名），只有这些部门
// 会在钉钉登录时绑定专属订阅分组。名称/层级取本地部门目录缓存，
// 缓存缺失时 Name 为空、Synced=false，由前端提示“未同步”。
// 映射不可用时退化为本地部门目录。
func (s *ManagerService) ListDepartments(ctx context.Context) ([]ManagerDepartmentOption, error) {
	index, err := s.loadDepartmentIndex(ctx)
	if err != nil {
		return nil, err
	}
	if s.deptGroupMap == nil {
		out := make([]ManagerDepartmentOption, 0, len(index))
		for deptID := range index {
			out = append(out, buildManagerDepartmentOption(index, deptID, ""))
		}
		sortDepartmentOptions(out)
		return out, nil
	}
	mapping, err := s.deptGroupMap(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ManagerDepartmentOption, 0, len(mapping))
	for deptID, groupName := range mapping {
		out = append(out, buildManagerDepartmentOption(index, deptID, strings.TrimSpace(groupName)))
	}
	sortDepartmentOptions(out)
	return out, nil
}

// UpsertDepartment 写入/刷新部门目录记录（钉钉登录同步或全量同步时调用）。
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

// SetDeptGroupMapReader 注入钉钉部门 → 订阅分组映射读取实现（SettingService），
// 由 Wire provider 调用，避免构造函数循环依赖。
func (s *ManagerService) SetDeptGroupMapReader(reader func(ctx context.Context) (map[int64]string, error)) {
	s.deptGroupMap = reader
}

// loadDepartmentIndex 读取本地钉钉部门目录缓存并建立 dept_id → 部门 的索引。
func (s *ManagerService) loadDepartmentIndex(ctx context.Context) (map[int64]DingTalkDepartment, error) {
	directory, err := s.scopeRepo.ListDepartments(ctx)
	if err != nil {
		return nil, err
	}
	index := make(map[int64]DingTalkDepartment, len(directory))
	for _, department := range directory {
		index[department.DeptID] = department
	}
	return index, nil
}

// buildManagerDepartmentOption 组装单个可选部门：名称/父级/停用状态取本地目录，
// 目录缺失时仅保留 dept_id 与所属订阅分组，Synced=false。
func buildManagerDepartmentOption(index map[int64]DingTalkDepartment, deptID int64, groupName string) ManagerDepartmentOption {
	option := ManagerDepartmentOption{
		DeptID:    deptID,
		GroupName: groupName,
		IsActive:  true,
	}
	if department, ok := index[deptID]; ok {
		option.ParentID = department.ParentID
		option.Name = strings.TrimSpace(department.Name)
		option.IsActive = department.IsActive
		option.Synced = true
	}
	option.Path = buildDepartmentPath(index, deptID)
	return option
}

// buildDepartmentPath 沿父级链拼出从根到该部门的名称路径（根在前）。
// 目录缺失或数据成环时提前终止，保证不会死循环。
func buildDepartmentPath(index map[int64]DingTalkDepartment, deptID int64) []string {
	names := make([]string, 0, 4)
	visited := make(map[int64]struct{}, 4)
	for current := deptID; current > 0; {
		if _, ok := visited[current]; ok {
			break
		}
		visited[current] = struct{}{}
		department, ok := index[current]
		if !ok {
			break
		}
		if name := strings.TrimSpace(department.Name); name != "" {
			names = append(names, name)
		}
		if department.ParentID <= 0 || department.ParentID == current {
			break
		}
		current = department.ParentID
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	return names
}

// ListManagerDepartments 返回经理负责的部门列表。授权只认 dept_id，
// 因此即使部门未出现在本地目录缓存中也要回显。
func (s *ManagerService) ListManagerDepartments(ctx context.Context, managerUserID int64) ([]ManagerDepartmentOption, error) {
	deptIDs, err := s.scopeRepo.ListManagerDepartmentIDs(ctx, managerUserID)
	if err != nil {
		return nil, err
	}
	index, err := s.loadDepartmentIndex(ctx)
	if err != nil {
		return nil, err
	}
	var mapping map[int64]string
	if s.deptGroupMap != nil {
		mapping, err = s.deptGroupMap(ctx)
		if err != nil {
			return nil, err
		}
	}
	out := make([]ManagerDepartmentOption, 0, len(deptIDs))
	for _, deptID := range deptIDs {
		groupName := ""
		if mapping != nil {
			groupName = strings.TrimSpace(mapping[deptID])
		}
		out = append(out, buildManagerDepartmentOption(index, deptID, groupName))
	}
	sortDepartmentOptions(out)
	return out, nil
}

// sortDepartmentOptions 先按订阅分组名、再按部门名、最后按 dept_id 排序，
// 便于前端按分组归类展示。
func sortDepartmentOptions(options []ManagerDepartmentOption) {
	sort.Slice(options, func(i, j int) bool {
		if options[i].GroupName != options[j].GroupName {
			return options[i].GroupName < options[j].GroupName
		}
		if options[i].Name != options[j].Name {
			return options[i].Name < options[j].Name
		}
		return options[i].DeptID < options[j].DeptID
	})
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
