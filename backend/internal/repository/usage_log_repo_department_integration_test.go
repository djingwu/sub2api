//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestUsageLog_DepartmentReportQueries exercises the team report queries
// against a real PostgreSQL instance. It guards the SQL that the capture-based
// handler tests cannot see, in particular the requested-model GROUP BY on
// GetDepartmentModelStatsWithFilters where an unqualified "model" alias used to
// collide with the usage_logs.model column, and the department-only group scope
// that keeps 免费组/cline/plan usage out of every aggregate.
func TestUsageLog_DepartmentReportQueries(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)

	user := mustCreateUser(t, client, &service.User{Email: "dept-report@test.com"})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-dept-report", Name: "dept-report"})
	account := mustCreateAccount(t, client, &service.Account{Name: "acc-dept-report"})
	group := mustCreateGroup(t, client, &service.Group{
		Name:             "dept-report-group",
		SubscriptionType: service.SubscriptionTypeSubscription,
		IsExclusive:      true,
	})
	emptyGroup := mustCreateGroup(t, client, &service.Group{
		Name:             "dept-report-empty",
		SubscriptionType: service.SubscriptionTypeSubscription,
		IsExclusive:      true,
	})
	// Neither a standard group (免费组/cline) nor a non-exclusive subscription
	// group (fixed plan group) is a department; their usage must never leak into
	// the team report.
	freeGroup := mustCreateGroup(t, client, &service.Group{Name: "dept-report-free"})
	planGroup := mustCreateGroup(t, client, &service.Group{
		Name:             "dept-report-plan",
		SubscriptionType: service.SubscriptionTypeSubscription,
	})

	now := time.Now().UTC().Truncate(time.Second)
	userAgent := "Claude-CLI/1.2.3 (extra)"
	for _, requestedModel := range []string{"gpt-5.6-luna", "gpt-5.6-luna", "claude-sonnet-4-5"} {
		_, err := repo.Create(ctx, &service.UsageLog{
			UserID:              user.ID,
			APIKeyID:            apiKey.ID,
			AccountID:           account.ID,
			Model:               "mapped-model",
			RequestedModel:      requestedModel,
			GroupID:             &group.ID,
			InputTokens:         10,
			OutputTokens:        5,
			CacheCreationTokens: 3,
			CacheReadTokens:     4,
			UserAgent:           &userAgent,
			CreatedAt:           now,
		})
		require.NoError(t, err)
	}
	for _, groupID := range []int64{freeGroup.ID, planGroup.ID} {
		_, err := repo.Create(ctx, &service.UsageLog{
			UserID:         user.ID,
			APIKeyID:       apiKey.ID,
			AccountID:      account.ID,
			Model:          "mapped-model",
			RequestedModel: "gpt-5.6-luna",
			GroupID:        &groupID,
			InputTokens:    1000,
			OutputTokens:   1000,
			UserAgent:      &userAgent,
			CreatedAt:      now,
		})
		require.NoError(t, err)
	}

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)

	models, err := repo.GetDepartmentModelStatsWithFilters(ctx, start, end, usagestats.UsageLogFilters{}, 5)
	require.NoError(t, err)
	require.Len(t, models, 2)
	require.Equal(t, "gpt-5.6-luna", models[0].Model)
	require.Equal(t, int64(2), models[0].Requests)
	require.Equal(t, int64(44), models[0].TotalTokens)
	require.Equal(t, "claude-sonnet-4-5", models[1].Model)
	require.Equal(t, int64(22), models[1].TotalTokens)

	breakdown, err := repo.GetGroupUsageBreakdownWithFilters(ctx, start, end, usagestats.UsageLogFilters{})
	require.NoError(t, err)
	require.Len(t, breakdown, 1)
	require.Equal(t, group.ID, breakdown[0].GroupID)
	require.Equal(t, int64(3), breakdown[0].Requests)
	require.Equal(t, int64(66), breakdown[0].TotalTokens)
	require.Equal(t, int64(2), breakdown[0].ModelCount)
	require.Equal(t, int64(1), breakdown[0].ActiveUserCount)

	summary, err := repo.GetDepartmentUsageSummaryWithFilters(ctx, start, end, usagestats.UsageLogFilters{})
	require.NoError(t, err)
	require.Equal(t, int64(3), summary.TotalRequests)
	require.Equal(t, int64(66), summary.TotalTokens)
	require.Equal(t, int64(1), summary.ActiveDepartments)
	require.Equal(t, int64(1), summary.ActiveUsers)

	clients, err := repo.GetClientSoftwareStatsWithFilters(ctx, start, end, usagestats.UsageLogFilters{}, 5)
	require.NoError(t, err)
	require.Len(t, clients, 1)
	require.Equal(t, "claude-cli", clients[0].ClientSoftware)
	require.Equal(t, int64(3), clients[0].Requests)
	require.Equal(t, int64(66), clients[0].TotalTokens)
	require.Equal(t, int64(1), clients[0].DepartmentCount)

	heatmap, err := repo.GetUsageHeatmapWithFilters(ctx, start, end, usagestats.UsageLogFilters{}, "UTC")
	require.NoError(t, err)
	var heatmapTokens int64
	for _, point := range heatmap {
		heatmapTokens += point.TotalTokens
	}
	require.Equal(t, int64(66), heatmapTokens)

	groups, err := repo.ListDepartmentGroups(ctx)
	require.NoError(t, err)
	found := map[int64]bool{}
	for _, candidate := range groups {
		found[candidate.GroupID] = true
	}
	require.True(t, found[group.ID], "active department with usage must be listed")
	require.True(t, found[emptyGroup.ID], "active department without usage must be listed")
	require.False(t, found[freeGroup.ID], "standard group is not a department")
	require.False(t, found[planGroup.ID], "non-exclusive subscription group is not a department")
}
