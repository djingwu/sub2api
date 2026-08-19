//go:build unit

package service

import (
	"context"
	"database/sql"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

// newAuthServiceForFixedPlan builds an AuthService with a real ent client
// (in-memory sqlite), a stub setting repo (carrying dingtalk_dept_group_map)
// and a stub subscription assigner. The user repository is not exercised by
// the dept-group binding path, so nil is passed.
func newAuthServiceForFixedPlan(t *testing.T, client *dbent.Client, settings map[string]string, assigner *defaultSubscriptionAssignerStub) *AuthService {
	t.Helper()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			ExpireHour: 1,
		},
		Default: config.DefaultConfig{
			UserBalance:     3.5,
			UserConcurrency: 2,
		},
	}

	return NewAuthService(
		client,
		nil, // userRepo (not exercised by this path)
		nil, // redeemRepo
		nil, // refreshTokenCache
		cfg,
		NewSettingService(&settingRepoStub{values: settings}, cfg),
		nil, // emailService
		nil, // turnstileService
		nil, // emailQueueService
		nil, // promoService
		assigner,
		nil, // affiliateService
		nil, // userPlatformQuotaRepo
	)
}

// newEntClientForFixedPlan opens an in-memory sqlite ent client with all schemas migrated.
func newEntClientForFixedPlan(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", "file:fixed_plan_ent?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// createDeptGroupForBinding creates a dept exclusive subscription group with
// the default validity days (30) that the login sync uses for the subscription.
func createDeptGroupForBinding(t *testing.T, ctx context.Context, client *dbent.Client, name string, validityDays int) *dbent.Group {
	t.Helper()
	g, err := client.Group.Create().
		SetName(name).
		SetIsExclusive(true).
		SetSubscriptionType(SubscriptionTypeSubscription).
		SetDefaultValidityDays(validityDays).
		Save(ctx)
	require.NoError(t, err)
	return g
}

const deptMapSetting = `{"7": "移动应用部", "88": "算法部"}`

func TestBindUserToDingTalkDeptGroup_MatchedDeptBinds(t *testing.T) {
	client := newEntClientForFixedPlan(t)
	ctx := context.Background()

	deptGroup := createDeptGroupForBinding(t, ctx, client, "移动应用部", 30)

	assigner := &defaultSubscriptionAssignerStub{}
	svc := newAuthServiceForFixedPlan(t, client, map[string]string{SettingKeyDingTalkDeptGroupMap: deptMapSetting}, assigner)

	require.NoError(t, svc.BindUserToDingTalkDeptGroup(ctx, 42, 7))

	require.Len(t, assigner.calls, 1, "matched dept must bind a subscription")
	require.Equal(t, int64(42), assigner.calls[0].UserID)
	require.Equal(t, deptGroup.ID, assigner.calls[0].GroupID)
	require.Equal(t, 30, assigner.calls[0].ValidityDays, "validity comes from the group default_validity_days")
	require.Equal(t, int64(42), assigner.calls[0].AssignedBy)
	require.Contains(t, assigner.calls[0].Notes, "dept group")
}

func TestBindUserToDingTalkDeptGroup_ZeroValidityFallsBack(t *testing.T) {
	client := newEntClientForFixedPlan(t)
	ctx := context.Background()

	createDeptGroupForBinding(t, ctx, client, "移动应用部", 0)

	assigner := &defaultSubscriptionAssignerStub{}
	svc := newAuthServiceForFixedPlan(t, client, map[string]string{SettingKeyDingTalkDeptGroupMap: deptMapSetting}, assigner)

	require.NoError(t, svc.BindUserToDingTalkDeptGroup(ctx, 42, 7))

	require.Len(t, assigner.calls, 1)
	require.Equal(t, 30, assigner.calls[0].ValidityDays, "falls back to 30 when group validity is unset")
}

func TestBindUserToDingTalkDeptGroup_UnmatchedDeptNoop(t *testing.T) {
	client := newEntClientForFixedPlan(t)
	ctx := context.Background()
	createDeptGroupForBinding(t, ctx, client, "移动应用部", 30)

	assigner := &defaultSubscriptionAssignerStub{}
	svc := newAuthServiceForFixedPlan(t, client, map[string]string{SettingKeyDingTalkDeptGroupMap: deptMapSetting}, assigner)

	// 未匹配部门：不分配任何分组/订阅
	require.NoError(t, svc.BindUserToDingTalkDeptGroup(ctx, 42, 999))
	// 空映射 / 未配置设置项
	svcEmpty := newAuthServiceForFixedPlan(t, client, map[string]string{}, assigner)
	require.NoError(t, svcEmpty.BindUserToDingTalkDeptGroup(ctx, 42, 7))
	// 根部门 / 非法 id
	require.NoError(t, svc.BindUserToDingTalkDeptGroup(ctx, 42, 1))
	require.NoError(t, svc.BindUserToDingTalkDeptGroup(ctx, 42, 0))

	require.Empty(t, assigner.calls, "no binding when dept is not matched")
}

func TestBindUserToDingTalkDeptGroup_MissingGroupFailsOpen(t *testing.T) {
	client := newEntClientForFixedPlan(t)
	ctx := context.Background()

	assigner := &defaultSubscriptionAssignerStub{}
	svc := newAuthServiceForFixedPlan(t, client, map[string]string{SettingKeyDingTalkDeptGroupMap: `{"7": "不存在的部门组"}`}, assigner)

	err := svc.BindUserToDingTalkDeptGroup(ctx, 42, 7)
	require.Error(t, err, "mapped group missing is surfaced for operator logging")
	require.Empty(t, assigner.calls)
}

func TestBindUserToDingTalkDeptGroup_InvalidMapFailsOpen(t *testing.T) {
	client := newEntClientForFixedPlan(t)
	ctx := context.Background()

	assigner := &defaultSubscriptionAssignerStub{}
	svc := newAuthServiceForFixedPlan(t, client, map[string]string{SettingKeyDingTalkDeptGroupMap: `not-json`}, assigner)

	err := svc.BindUserToDingTalkDeptGroup(ctx, 42, 7)
	require.Error(t, err)
	require.Empty(t, assigner.calls)
}

func TestBindUserToDingTalkDeptGroup_AssigneeErrorFailsOpen(t *testing.T) {
	client := newEntClientForFixedPlan(t)
	ctx := context.Background()
	createDeptGroupForBinding(t, ctx, client, "移动应用部", 30)

	assigner := &defaultSubscriptionAssignerStub{err: ErrGroupNotSubscriptionType}
	svc := newAuthServiceForFixedPlan(t, client, map[string]string{SettingKeyDingTalkDeptGroupMap: deptMapSetting}, assigner)

	err := svc.BindUserToDingTalkDeptGroup(ctx, 42, 7)
	require.Error(t, err)
}

func TestGetDingTalkDeptGroupMap_ParsesAndFilters(t *testing.T) {
	cfg := &config.Config{Default: config.DefaultConfig{UserBalance: 1, UserConcurrency: 1}}
	svc := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyDingTalkDeptGroupMap: `{"7": "移动应用部", "1": "公司", "abc": "bad", "88": " 算法部 "}`,
	}}, cfg)

	m, err := svc.GetDingTalkDeptGroupMap(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[int64]string{7: "移动应用部", 88: "算法部"}, m)
}

func TestGetDingTalkDeptGroupMap_EmptyOrInvalid(t *testing.T) {
	cfg := &config.Config{Default: config.DefaultConfig{UserBalance: 1, UserConcurrency: 1}}

	svcEmpty := NewSettingService(&settingRepoStub{values: map[string]string{}}, cfg)
	m, err := svcEmpty.GetDingTalkDeptGroupMap(context.Background())
	require.NoError(t, err)
	require.Empty(t, m)

	svcBad := NewSettingService(&settingRepoStub{values: map[string]string{SettingKeyDingTalkDeptGroupMap: `{broken`}}, cfg)
	_, err = svcBad.GetDingTalkDeptGroupMap(context.Background())
	require.Error(t, err)

	var nilSvc *SettingService
	m, err = nilSvc.GetDingTalkDeptGroupMap(context.Background())
	require.NoError(t, err)
	require.Empty(t, m)
}