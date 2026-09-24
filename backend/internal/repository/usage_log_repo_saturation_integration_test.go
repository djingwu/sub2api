//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestUsageLog_SaturationReportQueries exercises the saturation report queries
// against a real PostgreSQL instance: quota holders come from active
// user_subscriptions joined to their (limited) group, current-window usage uses
// the stored counters, and cumulative usage plus the model/effort mix are read
// from usage_logs so window resets cannot erase history.
func TestUsageLog_SaturationReportQueries(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)

	user := mustCreateUser(t, client, &service.User{Email: "saturation@test.com"})
	other := mustCreateUser(t, client, &service.User{Email: "saturation-other@test.com"})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-saturation", Name: "saturation"})
	account := mustCreateAccount(t, client, &service.Account{Name: "acc-saturation"})
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "saturation-group",
		SubscriptionType: service.SubscriptionTypeSubscription,
		IsExclusive:      true,
	})
	_, err := client.Group.UpdateOneID(group.ID).SetMonthlyLimitUsd(600).Save(ctx)
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Second)
	windowStart := now.Add(-5 * 24 * time.Hour)
	sub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(group.ID).
		SetStartsAt(now.Add(-45 * 24 * time.Hour)).
		SetExpiresAt(now.Add(25 * 24 * time.Hour)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(now).
		SetNotes("").
		SetDailyWindowStart(now.Add(-2 * time.Hour)).
		SetWeeklyWindowStart(now.Add(-2 * 24 * time.Hour)).
		SetMonthlyWindowStart(windowStart).
		SetDailyUsageUsd(10).
		SetWeeklyUsageUsd(80).
		SetMonthlyUsageUsd(100).
		Save(ctx)
	require.NoError(t, err)

	createLog := func(model, effort string, cost float64, created time.Time) {
		t.Helper()
		_, err := repo.Create(ctx, &service.UsageLog{
			UserID:                   user.ID,
			APIKeyID:                 apiKey.ID,
			AccountID:                account.ID,
			Model:                    model,
			RequestedModel:           model,
			ReasoningEffort:          &effort,
			RequestedReasoningEffort: &effort,
			GroupID:                  &group.ID,
			SubscriptionID:           &sub.ID,
			InputTokens:              100,
			OutputTokens:             50,
			ActualCost:               cost,
			BillingType:              service.BillingTypeSubscription,
			CreatedAt:                created,
		})
		require.NoError(t, err)
	}
	createLog("gpt-5.6-luna", "max", 60, now.Add(-48*time.Hour))
	createLog("gpt-5.6-sol", "high", 40, now.Add(-24*time.Hour))
	// Previous window: counted in the cumulative totals only.
	createLog("gpt-5.6-luna", "max", 100, now.Add(-40*24*time.Hour))

	aggregates, err := repo.GetSaturationUserAggregates(ctx, []int64{user.ID})
	require.NoError(t, err)
	require.Len(t, aggregates, 1)
	row := aggregates[0]
	require.Equal(t, user.ID, row.UserID)
	require.InDelta(t, 600, row.QuotaUSD, 0.001)
	require.InDelta(t, 100, row.UsedUSD, 0.001)
	require.InDelta(t, 10, row.DailyUsedUSD, 0.001)
	require.InDelta(t, 80, row.WeeklyUsedUSD, 0.001)
	require.InDelta(t, 200, row.LifetimeUsedUSD, 0.001)
	require.Equal(t, int64(3), row.LifetimeRequests)
	require.NotNil(t, row.WindowStartedAt)
	require.NotNil(t, row.FirstUsedAt)
	require.NotNil(t, row.LastUsedAt)
	require.Contains(t, row.GroupNames, "saturation-group")

	all, err := repo.GetSaturationUserAggregates(ctx, nil)
	require.NoError(t, err)
	for _, candidate := range all {
		require.NotEqual(t, other.ID, candidate.UserID, "users without an active quota subscription must not appear")
	}

	combos, err := repo.GetSaturationUserCombos(ctx, []int64{user.ID})
	require.NoError(t, err)
	require.Len(t, combos, 2)
	tokensByCombo := make(map[string]int64, len(combos))
	var totalCost float64
	for _, combo := range combos {
		tokensByCombo[combo.Model+"|"+combo.Effort] += combo.Tokens
		totalCost += combo.CostUSD
		require.Equal(t, int64(1), combo.Requests)
	}
	require.Equal(t, int64(150), tokensByCombo["gpt-5.6-luna|max"])
	require.Equal(t, int64(150), tokensByCombo["gpt-5.6-sol|high"])
	require.InDelta(t, 100, totalCost, 0.001)

	trend, err := repo.GetSaturationDailyTrend(ctx, 30)
	require.NoError(t, err)
	require.Len(t, trend, 30)
	byDate := make(map[string]int64, len(trend))
	var totalTrendCost float64
	for _, point := range trend {
		byDate[point.Date] = point.Requests
		totalTrendCost += point.CostUSD
	}
	require.Equal(t, int64(1), byDate[now.Add(-48*time.Hour).UTC().Format("2006-01-02")])
	require.Equal(t, int64(1), byDate[now.Add(-24*time.Hour).UTC().Format("2006-01-02")])
	require.InDelta(t, 100, totalTrendCost, 0.001)
}
