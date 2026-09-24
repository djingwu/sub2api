package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type saturationUsageRepoStub struct {
	UsageLogRepository
	aggregates     []usagestats.SaturationUserAggregate
	combos         []usagestats.SaturationUserCombo
	aggregateCalls int
	comboCalls     int
	trendDays      int
	trend          []usagestats.SaturationTrendPoint
}

func (s *saturationUsageRepoStub) GetSaturationUserAggregates(_ context.Context, _ []int64) ([]usagestats.SaturationUserAggregate, error) {
	s.aggregateCalls++
	return s.aggregates, nil
}

func (s *saturationUsageRepoStub) GetSaturationUserCombos(_ context.Context, _ []int64) ([]usagestats.SaturationUserCombo, error) {
	s.comboCalls++
	return s.combos, nil
}

func (s *saturationUsageRepoStub) GetSaturationDailyTrend(_ context.Context, days int) ([]usagestats.SaturationTrendPoint, error) {
	s.trendDays = days
	return s.trend, nil
}

type saturationSettingRepoStub struct {
	SettingRepository
	value    string
	valueErr error
	setValue string
	setErr   error
}

func (s *saturationSettingRepoStub) GetValue(_ context.Context, _ string) (string, error) {
	if s.valueErr != nil {
		return "", s.valueErr
	}
	return s.value, nil
}

func (s *saturationSettingRepoStub) Set(_ context.Context, _, value string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.setValue = value
	return nil
}

func timePtr(t time.Time) *time.Time { return &t }

func saturationFixture(t *testing.T) (*saturationUsageRepoStub, usagestats.SaturationConfig) {
	t.Helper()
	now := time.Now()
	dept := int64(10)
	repo := &saturationUsageRepoStub{
		aggregates: []usagestats.SaturationUserAggregate{
			{
				UserID:           1,
				Email:            "u1@example.com",
				Username:         "u1",
				Status:           "active",
				PrimaryDeptID:    &dept,
				DeptName:         "研发部",
				GroupNames:       "研发部组",
				QuotaUSD:         600,
				UsedUSD:          300,
				DailyUsedUSD:     10,
				WeeklyUsedUSD:    80,
				LifetimeUsedUSD:  900,
				LifetimeRequests: 17,
				FirstUsedAt:      timePtr(now.Add(-45 * 24 * time.Hour)),
				LastUsedAt:       timePtr(now.Add(-2 * time.Hour)),
				WindowStartedAt:  timePtr(now.Add(-10 * 24 * time.Hour)),
				ExpiresAt:        timePtr(now.Add(20 * 24 * time.Hour)),
			},
			{
				UserID:           2,
				Email:            "u2@example.com",
				Username:         "u2",
				Status:           "active",
				QuotaUSD:         600,
				UsedUSD:          0,
				LifetimeUsedUSD:  120,
				LifetimeRequests: 4,
				FirstUsedAt:      timePtr(now.Add(-40 * 24 * time.Hour)),
				LastUsedAt:       timePtr(now.Add(-35 * 24 * time.Hour)),
				WindowStartedAt:  timePtr(now.Add(-5 * 24 * time.Hour)),
			},
			{
				UserID:           3,
				Email:            "u3@example.com",
				Username:         "u3",
				Status:           "active",
				QuotaUSD:         600,
				UsedUSD:          480,
				LifetimeUsedUSD:  480,
				LifetimeRequests: 20,
			},
		},
		combos: []usagestats.SaturationUserCombo{
			{UserID: 1, Model: "gpt-5.6-luna", Effort: "max", Requests: 10, Tokens: 4_000_000, CostUSD: 200},
			{UserID: 1, Model: "gpt-6-astra", Effort: "medium", Requests: 5, Tokens: 500_000, CostUSD: 50},
			{UserID: 1, Model: "gpt-5.6-sol", Effort: "high", Requests: 2, Tokens: 100_000, CostUSD: 50},
			{UserID: 2, Model: "gpt-5.6-sol", Effort: "high", Requests: 3, Tokens: 1_000_000, CostUSD: 60},
			{UserID: 99, Model: "gpt-5.6-luna", Effort: "max", Requests: 1, Tokens: 1_000_000, CostUSD: 999},
		},
	}
	return repo, defaultSaturationConfig()
}

func TestSaturationSnapshotAggregatesUsers(t *testing.T) {
	repo, _ := saturationFixture(t)
	svc := NewSaturationService(repo, &saturationSettingRepoStub{})

	snapshot, err := svc.GetSnapshot(context.Background())
	require.NoError(t, err)

	require.Equal(t, int64(3), snapshot.Summary.UserCount)
	require.InDelta(t, 1800, snapshot.Summary.QuotaUSD, 0.001)
	require.InDelta(t, 780, snapshot.Summary.UsedUSD, 0.001)
	require.InDelta(t, 1500, snapshot.Summary.LifetimeUsedUSD, 0.001)
	require.Equal(t, int64(1), snapshot.Summary.DormantUsers)
	require.Equal(t, int64(1), snapshot.Summary.HeavyUsers)
	require.Equal(t, int64(1), snapshot.Summary.ResetUsers)
	require.Equal(t, int64(0), snapshot.Summary.NeverUsedUsers)
	require.Equal(t, int64(1), snapshot.Summary.CompliantUsers)
	require.InDelta(t, 50, snapshot.Summary.CompliantRate, 0.001)
	require.InDelta(t, 250, snapshot.Summary.BaselineCostUSD, 0.001)
	require.InDelta(t, 250.0/360.0*100, snapshot.Summary.BaselineCostShare, 0.001)
	require.InDelta(t, 4_500_000.0/5_600_000.0*100, snapshot.Summary.BaselineTokenShare, 0.001)
	require.InDelta(t, (50.0+0+80.0)/3, snapshot.Summary.AverageSaturation, 0.001)
	require.InDelta(t, (75.0+10.0+80.0)/3, snapshot.Summary.AverageLifetimeSaturation, 0.001)

	require.Len(t, snapshot.Combos, 3)
	require.Equal(t, "gpt-5.6-luna", snapshot.Combos[0].Model)
	require.True(t, snapshot.Combos[0].Baseline)
	require.InDelta(t, 200, snapshot.Combos[0].CostUSD, 0.001)
	require.Equal(t, "gpt-5.6-sol", snapshot.Combos[1].Model)
	require.Equal(t, int64(2), snapshot.Combos[1].Users)
	require.InDelta(t, 110, snapshot.Combos[1].CostUSD, 0.001)
	require.False(t, snapshot.Combos[1].Baseline)

	require.Equal(t, usagestats.SaturationBandDormant, snapshot.CurrentBands[0].Key)
	require.Equal(t, int64(1), snapshot.CurrentBands[0].Users)
	require.Equal(t, int64(0), snapshot.CurrentBands[1].Users)
	require.Equal(t, int64(1), snapshot.CurrentBands[2].Users)
	require.Equal(t, int64(1), snapshot.CurrentBands[3].Users)

	require.Equal(t, int64(1), snapshot.LifetimeBands[0].Users)
	require.Equal(t, int64(1), snapshot.LifetimeBands[2].Users)
	require.Equal(t, int64(1), snapshot.LifetimeBands[3].Users)
}

func TestSaturationListUsersFiltersAndSorts(t *testing.T) {
	repo, _ := saturationFixture(t)
	svc := NewSaturationService(repo, &saturationSettingRepoStub{})
	ctx := context.Background()

	list, err := svc.ListUsers(ctx, SaturationUserQuery{Metric: usagestats.SaturationMetricCurrent, Band: usagestats.SaturationBandDormant})
	require.NoError(t, err)
	require.Equal(t, int64(1), list.Total)
	require.Equal(t, int64(2), list.Items[0].UserID)

	list, err = svc.ListUsers(ctx, SaturationUserQuery{Compliance: usagestats.SaturationComplianceCompliant})
	require.NoError(t, err)
	require.Equal(t, int64(1), list.Total)
	require.Equal(t, int64(1), list.Items[0].UserID)
	require.True(t, list.Items[0].Compliant)
	require.InDelta(t, 250, list.Items[0].BaselineCostUSD, 0.001)
	require.InDelta(t, 250.0/300.0*100, list.Items[0].BaselineCostShare, 0.001)
	require.Equal(t, "gpt-5.6-luna", list.Items[0].TopModel)
	require.Equal(t, int64(17), list.Items[0].WindowRequests)
	require.Equal(t, int64(4_600_000), list.Items[0].WindowTokens)

	list, err = svc.ListUsers(ctx, SaturationUserQuery{Compliance: usagestats.SaturationComplianceNonCompliant})
	require.NoError(t, err)
	require.Equal(t, int64(2), list.Total)

	list, err = svc.ListUsers(ctx, SaturationUserQuery{Search: "研发"})
	require.NoError(t, err)
	require.Equal(t, int64(1), list.Total)
	require.Equal(t, int64(1), list.Items[0].UserID)

	list, err = svc.ListUsers(ctx, SaturationUserQuery{Sort: "lifetime_used_usd", Order: "desc"})
	require.NoError(t, err)
	require.Equal(t, []int64{1, 3, 2}, []int64{list.Items[0].UserID, list.Items[1].UserID, list.Items[2].UserID})

	list, err = svc.ListUsers(ctx, SaturationUserQuery{Page: 2, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, int64(3), list.Total)
	require.Len(t, list.Items, 1)
	require.Equal(t, int64(2), list.Items[0].UserID)
	require.Equal(t, 2, list.Page)
	require.Equal(t, 2, list.PageSize)
	require.Equal(t, int64(2), list.Items[0].Cycles)
}

func TestSaturationConfigDefaultsAndUpdate(t *testing.T) {
	repo, _ := saturationFixture(t)
	settings := &saturationSettingRepoStub{}
	svc := NewSaturationService(repo, settings)
	ctx := context.Background()

	config := svc.GetConfig(ctx)
	require.Equal(t, DefaultSaturationThresholdPercent, config.ThresholdPercent)
	require.Equal(t, DefaultSaturationBaselineCombos(), config.BaselineCombos)

	_, err := svc.UpdateConfig(ctx, usagestats.SaturationConfig{ThresholdPercent: 50})
	require.Error(t, err)

	_, err = svc.UpdateConfig(ctx, usagestats.SaturationConfig{
		BaselineCombos:   []usagestats.SaturationBaselineCombo{{Model: "gpt-5.6-luna", Effort: "max"}},
		ThresholdPercent: 0,
	})
	require.Error(t, err)

	updated, err := svc.UpdateConfig(ctx, usagestats.SaturationConfig{
		BaselineCombos: []usagestats.SaturationBaselineCombo{
			{Model: " gpt-5.6-luna ", Effort: "max"},
			{Model: "gpt-5.6-luna", Effort: "MAX"},
			{Model: "gpt-6-astra", Effort: "medium"},
		},
		ThresholdPercent: 60,
	})
	require.NoError(t, err)
	require.Len(t, updated.BaselineCombos, 2)
	require.Equal(t, "gpt-5.6-luna", updated.BaselineCombos[0].Model)
	require.NotEmpty(t, settings.setValue)

	settings.value = settings.setValue
	reloaded := svc.GetConfig(ctx)
	require.Equal(t, 60.0, reloaded.ThresholdPercent)
	require.Len(t, reloaded.BaselineCombos, 2)

	settings.valueErr = errors.New("db down")
	fallback := svc.GetConfig(ctx)
	require.Equal(t, DefaultSaturationThresholdPercent, fallback.ThresholdPercent)
}

func TestSaturationUsesAdminScope(t *testing.T) {
	repo, _ := saturationFixture(t)
	svc := NewSaturationService(repo, &saturationSettingRepoStub{})

	snapshot, err := svc.GetSnapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(3), snapshot.Summary.UserCount)
	require.Equal(t, 1, repo.aggregateCalls)
	require.Equal(t, 1, repo.comboCalls)
}

func TestSaturationDailyTrendDefaultsAndCaps(t *testing.T) {
	repo, _ := saturationFixture(t)
	repo.trend = []usagestats.SaturationTrendPoint{{Date: "2026-09-23", Requests: 2, Tokens: 3, CostUSD: 4, Users: 1}}
	svc := NewSaturationService(repo, &saturationSettingRepoStub{})
	ctx := context.Background()

	points, err := svc.GetDailyTrend(ctx, 0)
	require.NoError(t, err)
	require.Len(t, points, 1)
	require.Equal(t, int64(2), points[0].Requests)
	require.Equal(t, 30, repo.trendDays)

	_, err = svc.GetDailyTrend(ctx, 365)
	require.NoError(t, err)
	require.Equal(t, 90, repo.trendDays)

	_, err = svc.GetDailyTrend(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, 7, repo.trendDays)
}
