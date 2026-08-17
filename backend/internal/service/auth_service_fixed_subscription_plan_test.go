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
// (in-memory sqlite) and a stub subscription assigner. The user repository is
// not exercised by the fixed-plan binding path, so nil is passed.
func newAuthServiceForFixedPlan(t *testing.T, client *dbent.Client, assigner *defaultSubscriptionAssignerStub) *AuthService {
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
		NewSettingService(&settingRepoStub{values: map[string]string{}}, cfg),
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

func TestBindFixedSubscriptionPlanToNewUser_BindsPlanFromDB(t *testing.T) {
	client := newEntClientForFixedPlan(t)
	ctx := context.Background()

	group, err := client.Group.Create().
		SetName("fixed-plan-group").
		SetSubscriptionType(SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)

	plan, err := client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("$200 固定套餐").
		SetPrice(200).
		SetCurrency("USD").
		SetValidityDays(30).
		SetProductName(FixedSubscriptionPlanProductName).
		Save(ctx)
	require.NoError(t, err)

	assigner := &defaultSubscriptionAssignerStub{}
	svc := newAuthServiceForFixedPlan(t, client, assigner)

	svc.assignSubscriptions(ctx, 42, nil, "signup notes")

	require.Len(t, assigner.calls, 1, "fixed plan must be auto-bound on signup")
	require.Equal(t, int64(42), assigner.calls[0].UserID)
	require.Equal(t, plan.GroupID, assigner.calls[0].GroupID)
	require.Equal(t, 30, assigner.calls[0].ValidityDays)
	require.Contains(t, assigner.calls[0].Notes, "fixed $200")
}

func TestBindFixedSubscriptionPlanToNewUser_MissingPlanSkips(t *testing.T) {
	client := newEntClientForFixedPlan(t)
	assigner := &defaultSubscriptionAssignerStub{}
	svc := newAuthServiceForFixedPlan(t, client, assigner)

	svc.assignSubscriptions(context.Background(), 42, nil, "signup notes")

	require.Empty(t, assigner.calls, "no binding when the fixed plan is absent")
}

func TestBindFixedSubscriptionPlanToNewUser_AssigneeErrorFailsOpen(t *testing.T) {
	client := newEntClientForFixedPlan(t)
	ctx := context.Background()

	group, err := client.Group.Create().
		SetName("fixed-plan-group-2").
		SetSubscriptionType(SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("$200 固定套餐").
		SetPrice(200).
		SetCurrency("USD").
		SetValidityDays(30).
		SetProductName(FixedSubscriptionPlanProductName).
		Save(ctx)
	require.NoError(t, err)

	assigner := &defaultSubscriptionAssignerStub{err: ErrGroupNotSubscriptionType}
	svc := newAuthServiceForFixedPlan(t, client, assigner)

	require.NotPanics(t, func() {
		svc.assignSubscriptions(ctx, 42, nil, "signup notes")
	})
}