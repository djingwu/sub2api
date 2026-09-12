package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type stubScopeRepo struct {
	departments    []DingTalkDepartment
	managerDepts   []DingTalkDepartment
	inScope        bool
	inScopeErr     error
	upsertCalled   bool
	replaceCalled  bool
	replaceDeptIDs []int64
}

func (s *stubScopeRepo) UpsertDepartment(_ context.Context, _ *DingTalkDepartment) error {
	s.upsertCalled = true
	return nil
}

func (s *stubScopeRepo) ListDepartments(_ context.Context) ([]DingTalkDepartment, error) {
	return s.departments, nil
}

func (s *stubScopeRepo) ListManagerDepartments(_ context.Context, _ int64) ([]DingTalkDepartment, error) {
	return s.managerDepts, nil
}

func (s *stubScopeRepo) ReplaceManagerDepartments(_ context.Context, _ int64, deptIDs []int64) error {
	s.replaceCalled = true
	s.replaceDeptIDs = deptIDs
	return nil
}

func (s *stubScopeRepo) IsUserInManagerScope(_ context.Context, _, _ int64) (bool, error) {
	return s.inScope, s.inScopeErr
}

func (s *stubScopeRepo) ListManagerUserIDs(_ context.Context, _ int64, params pagination.PaginationParams) ([]int64, *pagination.PaginationResult, error) {
	return []int64{1, 2}, &pagination.PaginationResult{Total: 2, Page: params.Page, PageSize: params.Limit()}, nil
}

type stubUserRepo struct {
	UserRepository
	users map[int64]*User
}

func (s *stubUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	if u, ok := s.users[id]; ok {
		return u, nil
	}
	return nil, ErrUserNotFound
}

type stubSubRepo struct {
	UserSubscriptionRepository
	subs map[int64]*UserSubscription
}

func (s *stubSubRepo) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	if sub, ok := s.subs[id]; ok {
		return sub, nil
	}
	return nil, ErrSubscriptionNotFound
}

func newTestManagerService(inScope bool) (*ManagerService, *stubScopeRepo) {
	scope := &stubScopeRepo{inScope: inScope}
	userRepo := &stubUserRepo{users: map[int64]*User{
		1: {ID: 1, Role: RoleManager},
		2: {ID: 2, Role: RoleUser},
	}}
	subRepo := &stubSubRepo{subs: map[int64]*UserSubscription{
		10: {ID: 10, UserID: 2},
		11: {ID: 11, UserID: 99},
	}}
	return NewManagerService(scope, userRepo, subRepo), scope
}

func TestRequireUserInScope_AllowsInScope(t *testing.T) {
	svc, _ := newTestManagerService(true)
	if err := svc.RequireUserInScope(context.Background(), 1, 2); err != nil {
		t.Fatalf("expected in-scope user to pass, got %v", err)
	}
}

func TestRequireUserInScope_RejectsOutOfScope(t *testing.T) {
	svc, _ := newTestManagerService(false)
	err := svc.RequireUserInScope(context.Background(), 1, 2)
	if err == nil {
		t.Fatal("expected out-of-scope user to be rejected")
	}
	if !errors.Is(err, ErrManagerScopeForbidden) {
		t.Fatalf("expected ErrManagerScopeForbidden, got %v", err)
	}
}

func TestResetMemberQuota_RejectsOutOfScopeSubscription(t *testing.T) {
	svc, _ := newTestManagerService(false)
	// Subscription 11 belongs to user 99, outside the manager's scope.
	_, err := svc.ResetMemberQuota(context.Background(), 1, 11, true, false, false)
	if !errors.Is(err, ErrManagerScopeForbidden) {
		t.Fatalf("expected ErrManagerScopeForbidden, got %v", err)
	}
}

func TestResetMemberQuota_UsesInjectedResetter(t *testing.T) {
	svc, _ := newTestManagerService(true)
	called := false
	svc.SetQuotaResetter(func(_ context.Context, subscriptionID int64, _, _, _ bool) (*UserSubscription, error) {
		called = true
		sub := &UserSubscription{ID: subscriptionID}
		return sub, nil
	})
	sub, err := svc.ResetMemberQuota(context.Background(), 1, 10, true, false, false)
	if err != nil {
		t.Fatalf("expected reset to succeed, got %v", err)
	}
	if !called {
		t.Fatal("expected injected resetter to be called")
	}
	if sub == nil || sub.ID != 10 {
		t.Fatalf("unexpected subscription result: %+v", sub)
	}
}

func TestReplaceManagerDepartments_RejectsNonManager(t *testing.T) {
	svc, scope := newTestManagerService(true)
	// user 2 is a plain user, not a manager
	err := svc.ReplaceManagerDepartments(context.Background(), 2, []int64{42})
	if err == nil {
		t.Fatal("expected error when target is not a manager")
	}
	if scope.replaceCalled {
		t.Fatal("scope must not be modified for non-manager targets")
	}
}

func TestReplaceManagerDepartments_AcceptsManager(t *testing.T) {
	svc, scope := newTestManagerService(true)
	if err := svc.ReplaceManagerDepartments(context.Background(), 1, []int64{42, 43, 42}); err != nil {
		t.Fatalf("expected manager scope update to succeed, got %v", err)
	}
	if !scope.replaceCalled {
		t.Fatal("expected scope replacement call")
	}
	if len(scope.replaceDeptIDs) != 3 {
		t.Fatalf("expected dept IDs to be forwarded, got %v", scope.replaceDeptIDs)
	}
}



func TestListMemberSubscriptions_ChecksScopeBeforeQuery(t *testing.T) {
	svc, _ := newTestManagerService(false)
	_, err := svc.ListMemberSubscriptions(context.Background(), 1, 2)
	if !errors.Is(err, ErrManagerScopeForbidden) {
		t.Fatalf("expected scope check to run first, got %v", err)
	}
}
