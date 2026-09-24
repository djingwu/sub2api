package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type saturationRepoCapture struct {
	service.UsageLogRepository
	aggregates []usagestats.SaturationUserAggregate
	combos     []usagestats.SaturationUserCombo
	trend      []usagestats.SaturationTrendPoint
}

func (r *saturationRepoCapture) GetSaturationUserAggregates(_ context.Context, _ []int64) ([]usagestats.SaturationUserAggregate, error) {
	return r.aggregates, nil
}

func (r *saturationRepoCapture) GetSaturationUserCombos(_ context.Context, _ []int64) ([]usagestats.SaturationUserCombo, error) {
	return r.combos, nil
}

func (r *saturationRepoCapture) GetSaturationDailyTrend(_ context.Context, _ int) ([]usagestats.SaturationTrendPoint, error) {
	return r.trend, nil
}

type saturationSettingCapture struct {
	service.SettingRepository
	value string
	set   bool
}

func (s *saturationSettingCapture) GetValue(_ context.Context, _ string) (string, error) {
	return s.value, nil
}

func (s *saturationSettingCapture) Set(_ context.Context, _, value string) error {
	s.value = value
	s.set = true
	return nil
}

func saturationTestRouter(handler *SaturationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/saturation", handler.Snapshot)
	router.GET("/admin/saturation/users", handler.Users)
	router.GET("/admin/saturation/trend", handler.Trend)
	router.PUT("/admin/saturation/config", handler.UpdateConfig)
	return router
}

func TestSaturationSnapshotReturnsBandsAndConfig(t *testing.T) {
	repo := &saturationRepoCapture{
		aggregates: []usagestats.SaturationUserAggregate{
			{UserID: 1, Email: "u1@example.com", QuotaUSD: 600, UsedUSD: 600, LifetimeUsedUSD: 600},
		},
		combos: []usagestats.SaturationUserCombo{
			{UserID: 1, Model: "gpt-5.6-luna", Effort: "max", Requests: 1, Tokens: 1_000_000, CostUSD: 600},
		},
	}
	svc := service.NewSaturationService(repo, &saturationSettingCapture{})
	handler := NewSaturationHandler(svc)

	rec := httptest.NewRecorder()
	saturationTestRouter(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/saturation", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	var envelope struct {
		Data struct {
			Config struct {
				ThresholdPercent float64 `json:"threshold_percent"`
				BaselineCombos   []struct {
					Model  string `json:"model"`
					Effort string `json:"effort"`
				} `json:"baseline_combos"`
			} `json:"config"`
			Summary struct {
				UserCount         int64   `json:"user_count"`
				HeavyUsers        int64   `json:"heavy_users"`
				CompliantUsers    int64   `json:"compliant_users"`
				BaselineCostShare float64 `json:"baseline_cost_share"`
			} `json:"summary"`
			CurrentBands []struct {
				Key   string `json:"key"`
				Users int64  `json:"users"`
			} `json:"current_bands"`
			Combos []struct {
				Model    string `json:"model"`
				Baseline bool   `json:"baseline"`
			} `json:"combos"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), "body: %s", rec.Body.String())
	require.Equal(t, int64(1), envelope.Data.Summary.UserCount)
	require.Equal(t, int64(1), envelope.Data.Summary.HeavyUsers)
	require.Equal(t, int64(1), envelope.Data.Summary.CompliantUsers)
	require.InDelta(t, 100, envelope.Data.Summary.BaselineCostShare, 0.001)
	require.Equal(t, 50.0, envelope.Data.Config.ThresholdPercent)
	require.Len(t, envelope.Data.Config.BaselineCombos, 2)
	require.Len(t, envelope.Data.CurrentBands, 4)
	require.Equal(t, "saturated", envelope.Data.CurrentBands[3].Key)
	require.Equal(t, int64(1), envelope.Data.CurrentBands[3].Users)
	require.Len(t, envelope.Data.Combos, 1)
	require.True(t, envelope.Data.Combos[0].Baseline)
}

func TestSaturationUsersFiltersByBand(t *testing.T) {
	repo := &saturationRepoCapture{
		aggregates: []usagestats.SaturationUserAggregate{
			{UserID: 7, Email: "u7@example.com", QuotaUSD: 600, UsedUSD: 60},
			{UserID: 8, Email: "u8@example.com", QuotaUSD: 600, UsedUSD: 540},
		},
	}
	svc := service.NewSaturationService(repo, &saturationSettingCapture{})
	handler := NewSaturationHandler(svc)

	rec := httptest.NewRecorder()
	saturationTestRouter(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/saturation/users?band=dormant", nil))

	require.Equal(t, http.StatusOK, rec.Code)

	var envelope struct {
		Data struct {
			Total int64 `json:"total"`
			Items []struct {
				UserID      int64   `json:"user_id"`
				UsedPercent float64 `json:"used_percent"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), "body: %s", rec.Body.String())
	require.Equal(t, int64(1), envelope.Data.Total)
	require.Equal(t, int64(7), envelope.Data.Items[0].UserID)
	require.InDelta(t, 10, envelope.Data.Items[0].UsedPercent, 0.001)
}

func TestSaturationTrendParsesDays(t *testing.T) {
	svc := service.NewSaturationService(&saturationRepoCapture{}, &saturationSettingCapture{})
	handler := NewSaturationHandler(svc)
	router := saturationTestRouter(handler)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/saturation/trend?days=abc", nil))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/saturation/trend?days=7", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope struct {
		Data []struct {
			Date     string  `json:"date"`
			Requests int64   `json:"requests"`
			Tokens   int64   `json:"tokens"`
			CostUSD  float64 `json:"cost_usd"`
			Users    int64   `json:"users"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), "body: %s", rec.Body.String())
	require.NotNil(t, envelope.Data)
}

func TestSaturationUpdateConfigValidatesAndPersists(t *testing.T) {
	settings := &saturationSettingCapture{}
	svc := service.NewSaturationService(&saturationRepoCapture{}, settings)
	handler := NewSaturationHandler(svc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/admin/saturation/config", handler.UpdateConfig)

	invalid := `{"baseline_combos":[],"threshold_percent":50}`
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/admin/saturation/config", strings.NewReader(invalid)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.False(t, settings.set)

	valid := `{"baseline_combos":[{"model":"gpt-5.6-luna","effort":"max"}],"threshold_percent":65}`
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/admin/saturation/config", strings.NewReader(valid)))
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, settings.set)
	require.Contains(t, settings.value, "gpt-5.6-luna")
	require.Contains(t, settings.value, "65")
}
