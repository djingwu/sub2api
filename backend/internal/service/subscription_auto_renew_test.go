package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

// autoRenewRepoStub 内存订阅仓储：覆盖自动续期所需的全部方法。
type autoRenewRepoStub struct {
	userSubRepoNoop

	current    UserSubscription
	dueSubs    []UserSubscription
	duePages   int
	updateFlag bool
}

func (r *autoRenewRepoStub) GetByID(context.Context, int64) (*UserSubscription, error) {
	cp := r.current
	return &cp, nil
}

func (r *autoRenewRepoStub) GetByIDForUpdate(context.Context, int64) (*UserSubscription, error) {
	cp := r.current
	return &cp, nil
}

func (r *autoRenewRepoStub) ExtendExpiry(_ context.Context, _ int64, expiresAt time.Time) error {
	r.current.ExpiresAt = expiresAt
	return nil
}

func (r *autoRenewRepoStub) UpdateStatus(_ context.Context, _ int64, status string) error {
	r.current.Status = status
	return nil
}

func (r *autoRenewRepoStub) UpdateNotes(_ context.Context, _ int64, notes string) error {
	r.current.Notes = notes
	return nil
}

func (r *autoRenewRepoStub) Update(_ context.Context, sub *UserSubscription) error {
	r.current = *sub
	return nil
}

func (r *autoRenewRepoStub) UpdateAutoRenew(_ context.Context, _ int64, enabled bool) (*UserSubscription, error) {
	r.current.AutoRenew = enabled
	r.updateFlag = true
	cp := r.current
	return &cp, nil
}

func (r *autoRenewRepoStub) ListDueAutoRenew(_ context.Context, _ time.Time, params pagination.PaginationParams) ([]UserSubscription, *pagination.PaginationResult, error) {
	if r.duePages <= 1 {
		return r.dueSubs, &pagination.PaginationResult{Page: params.Page, Pages: 1, Total: int64(len(r.dueSubs))}, nil
	}
	// 仅在单页场景使用；多页场景由测试显式构造 secondPage。
	if params.Page >= r.duePages {
		return nil, &pagination.PaginationResult{Page: params.Page, Pages: r.duePages, Total: 0}, nil
	}
	return nil, &pagination.PaginationResult{Page: params.Page, Pages: r.duePages, Total: 0}, nil
}

func TestFreeRenewSubscriptionExtendsUnexpired(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	repo := &autoRenewRepoStub{current: UserSubscription{
		ID: 1, UserID: 10, GroupID: 20, Status: SubscriptionStatusActive,
		StartsAt: now.AddDate(0, 0, -5), ExpiresAt: now.AddDate(0, 0, 5),
	}}
	svc := NewSubscriptionService(nil, repo, nil, nil, nil)
	svc.now = func() time.Time { return now }

	sub, err := svc.FreeRenewSubscription(context.Background(), 1, 30, subscriptionAutoRenewNote)
	require.NoError(t, err)
	require.Equal(t, now.AddDate(0, 0, 35), sub.ExpiresAt)
	require.Equal(t, SubscriptionStatusActive, sub.Status)
	require.Equal(t, subscriptionAutoRenewNote, sub.Notes)
}

func TestFreeRenewSubscriptionReactivatesExpiredAndResetsWindows(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	oldStart := now.Add(-48 * time.Hour)
	repo := &autoRenewRepoStub{current: UserSubscription{
		ID: 2, UserID: 10, GroupID: 20, Status: SubscriptionStatusExpired,
		StartsAt: now.AddDate(0, 0, -5), ExpiresAt: now.Add(-time.Hour),
		DailyWindowStart:  &oldStart, WeeklyWindowStart: &oldStart, MonthlyWindowStart: &oldStart,
		DailyUsageUSD: 9, WeeklyUsageUSD: 9, MonthlyUsageUSD: 9,
	}}
	svc := NewSubscriptionService(nil, repo, nil, nil, nil)
	svc.now = func() time.Time { return now }

	sub, err := svc.FreeRenewSubscription(context.Background(), 2, 7, subscriptionAutoRenewNote)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusActive, sub.Status)
	require.Equal(t, now.AddDate(0, 0, 7), sub.ExpiresAt)
	// 过期续期会重置用量窗口并清零历史用量。
	require.Zero(t, sub.DailyUsageUSD)
	require.Zero(t, sub.WeeklyUsageUSD)
	require.Zero(t, sub.MonthlyUsageUSD)
	require.NotNil(t, sub.DailyWindowStart)
	require.NotNil(t, sub.WeeklyWindowStart)
	require.NotNil(t, sub.MonthlyWindowStart)
	require.Contains(t, sub.Notes, subscriptionAutoRenewNote)
}

func TestAutoRenewValidityDaysFallsBackToDefault(t *testing.T) {
	svc := NewSubscriptionService(nil, nil, nil, nil, nil)
	require.Equal(t, defaultAutoRenewValidityDays, svc.AutoRenewValidityDays(context.Background(), 99))
}

func TestSetAutoRenewTogglesAndExtends(t *testing.T) {
	repo := &autoRenewRepoStub{current: UserSubscription{ID: 3, UserID: 10, GroupID: 20, AutoRenew: true}}
	svc := NewSubscriptionService(nil, repo, nil, nil, nil)

	sub, err := svc.SetAutoRenew(context.Background(), 3, false)
	require.NoError(t, err)
	require.True(t, repo.updateFlag)
	require.False(t, sub.AutoRenew)

	// 幂等：同值直接返回不落库。
	repo.updateFlag = false
	_, err = svc.SetAutoRenew(context.Background(), 3, false)
	require.NoError(t, err)
	require.False(t, repo.updateFlag)
}

func TestUserIsDingTalkBySignupSource(t *testing.T) {
	svc := NewSubscriptionService(nil, nil, nil, nil, nil)
	require.True(t, svc.userIsDingTalk(&User{SignupSource: "dingtalk"}))
	require.True(t, svc.userIsDingTalk(&User{SignupSource: "DingTalk"}))
	require.False(t, svc.userIsDingTalk(&User{SignupSource: "email"}))
	require.False(t, svc.userIsDingTalk(&User{SignupSource: "wechat"}))
	require.False(t, svc.userIsDingTalk(nil))
}

// autoRenewUserRepoStub 钉钉身份查询桩。
type autoRenewUserRepoStub struct {
	dingtalk bool
}

func (r *autoRenewUserRepoStub) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	if !r.dingtalk {
		return nil, nil
	}
	return []UserAuthIdentityRecord{{ProviderType: "dingtalk"}}, nil
}

type autoRenewGroupRepoStub struct {
	subscriptionGroupRepoStub
}

func TestSubscriptionAutoRenewServiceRenewsEligibleOnly(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	repo := &autoRenewRepoStub{dueSubs: []UserSubscription{
		{ID: 1, UserID: 10, GroupID: 20, AutoRenew: true, Status: SubscriptionStatusActive, ExpiresAt: now.Add(time.Hour), User: &User{SignupSource: "dingtalk"}},
		{ID: 2, UserID: 11, GroupID: 20, AutoRenew: true, Status: SubscriptionStatusActive, ExpiresAt: now.Add(time.Hour), User: &User{SignupSource: "email"}},
		{ID: 3, UserID: 12, GroupID: 20, AutoRenew: false, Status: SubscriptionStatusActive, ExpiresAt: now.Add(time.Hour), User: &User{SignupSource: "dingtalk"}},
		{ID: 4, UserID: 10, GroupID: 21, AutoRenew: true, Status: SubscriptionStatusExpired, ExpiresAt: now.Add(-time.Hour), User: &User{SignupSource: "email"}},
	}}

	// 服务仅校验 dingtalk 身份 + auto_renew；续期动作本身由桩忽略。
	svc := NewSubscriptionAutoRenewService(repo, &autoRenewUserRepoStub{}, NewSubscriptionService(&subscriptionGroupRepoStub{group: &Group{ID: 20, SubscriptionType: SubscriptionTypeSubscription}}, repo, nil, nil, nil), time.Minute)

	require.True(t, svc.isEligibleForAutoRenew(context.Background(), &repo.dueSubs[0]))
	require.False(t, svc.isEligibleForAutoRenew(context.Background(), &repo.dueSubs[1]))
	require.False(t, svc.isEligibleForAutoRenew(context.Background(), &repo.dueSubs[2]))
}

func TestSubscriptionAutoRenewServiceIdentityFallback(t *testing.T) {
	svc := NewSubscriptionAutoRenewService(nil, &autoRenewUserRepoStub{dingtalk: true}, NewSubscriptionService(nil, nil, nil, nil, nil), time.Minute)
	require.True(t, svc.isEligibleForAutoRenew(context.Background(), &UserSubscription{UserID: 1, AutoRenew: true, User: &User{SignupSource: "email"}}))

	svc = NewSubscriptionAutoRenewService(nil, &autoRenewUserRepoStub{dingtalk: false}, NewSubscriptionService(nil, nil, nil, nil, nil), time.Minute)
	require.False(t, svc.isEligibleForAutoRenew(context.Background(), &UserSubscription{UserID: 1, AutoRenew: true, User: &User{SignupSource: "email"}}))
}

func TestSubscriptionAutoRenewServiceIdentityLookupErrorFailsClosed(t *testing.T) {
	svc := NewSubscriptionAutoRenewService(nil, &errIdentityRepoStub{}, NewSubscriptionService(nil, nil, nil, nil, nil), time.Minute)
	require.False(t, svc.isEligibleForAutoRenew(context.Background(), &UserSubscription{UserID: 1, AutoRenew: true, User: &User{SignupSource: "email"}}))
}

type errIdentityRepoStub struct{}

func (r *errIdentityRepoStub) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	return nil, errors.New("boom")
}