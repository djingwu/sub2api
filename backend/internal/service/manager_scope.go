package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// DingTalkDepartment is the locally cached department directory entry used by
// manager authorization and displayed by the admin configuration UI.
type DingTalkDepartment struct {
	DeptID    int64
	ParentID  int64
	Name      string
	IsActive  bool
	SyncedAt  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ManagerScopeRepository contains the narrow SQL-backed department scope
// operations. It deliberately does not reuse the subscription-group mapping.
type ManagerScopeRepository interface {
	UpsertDepartment(ctx context.Context, department *DingTalkDepartment) error
	ListDepartments(ctx context.Context) ([]DingTalkDepartment, error)
	ListManagerDepartments(ctx context.Context, managerUserID int64) ([]DingTalkDepartment, error)
	ReplaceManagerDepartments(ctx context.Context, managerUserID int64, deptIDs []int64) error
	IsUserInManagerScope(ctx context.Context, managerUserID, userID int64) (bool, error)
	ListManagerUserIDs(ctx context.Context, managerUserID int64, params pagination.PaginationParams) ([]int64, *pagination.PaginationResult, error)
}
