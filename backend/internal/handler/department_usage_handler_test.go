package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDepartmentUsageOmitsCostsAndUserScope(t *testing.T) {
	repo := &userUsageRepoCapture{
		departmentBreakdown: []usagestats.GroupUsageBreakdown{
			{GroupID: 2, GroupName: "Platform", Requests: 2, TotalTokens: 20, AvgDurationMs: 1200},
			{GroupID: 1, GroupName: "Application", Requests: 1, TotalTokens: 10, AvgDurationMs: 800},
		},
	}
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	usageHandler := NewUsageHandler(usageSvc, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage/department-usage", usageHandler.DepartmentUsage)

	req := httptest.NewRequest(http.MethodGet, "/usage/department-usage?start_date=2026-09-01&end_date=2026-09-07&api_key_id=not-a-number&group_id=not-a-number&model=should-be-ignored", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "cost")
	require.NotContains(t, rec.Body.String(), "actual_cost")
	require.NotContains(t, rec.Body.String(), "account_cost")
	require.Equal(t, int64(0), repo.departmentBreakdownFilters.UserID)
	require.Equal(t, int64(0), repo.departmentBreakdownFilters.APIKeyID)
	require.Equal(t, int64(0), repo.departmentBreakdownFilters.GroupID)
	require.Equal(t, []string{"Application", "Platform"}, departmentNames(t, rec.Body.Bytes()))
}

func TestDepartmentUsageIncludesTopModelsWithoutCosts(t *testing.T) {
	repo := &userUsageRepoCapture{
		departmentBreakdown: []usagestats.GroupUsageBreakdown{
			{GroupID: 2, GroupName: "Platform", Requests: 2, TotalTokens: 20, ModelCount: 4, ActiveUserCount: 3, StreamRequests: 2},
			{GroupID: 1, GroupName: "Application", Requests: 1, TotalTokens: 10, ModelCount: 2, ActiveUserCount: 1},
		},
		groupModelStats: []usagestats.GroupModelStat{
			{GroupID: 2, GroupName: "Platform", Model: "claude-sonnet-4-5", TotalTokens: 15},
			{GroupID: 2, GroupName: "Platform", Model: "gpt-5", TotalTokens: 5},
			{GroupID: 1, GroupName: "Application", Model: "claude-sonnet-4-5", TotalTokens: 10},
		},
	}
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	usageHandler := NewUsageHandler(usageSvc, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage/department-usage", usageHandler.DepartmentUsage)

	req := httptest.NewRequest(http.MethodGet, "/usage/department-usage?start_date=2026-09-01&end_date=2026-09-07&group_id=99&model=should-be-ignored", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "cost")
	require.Equal(t, departmentTopModelLimit, repo.departmentTopModelLimit)
	require.Equal(t, int64(0), repo.departmentTopModelFilters.UserID)
	require.Equal(t, int64(0), repo.departmentTopModelFilters.APIKeyID)
	require.Equal(t, int64(0), repo.departmentTopModelFilters.GroupID)

	var envelope struct {
		Data struct {
			Departments []struct {
				GroupName           string `json:"group_name"`
				Requests            int64  `json:"requests"`
				ModelCount          int64  `json:"model_count"`
				ActiveUserCount     int64  `json:"active_user_count"`
				StreamRequests      int64  `json:"stream_requests"`
				CacheCreationTokens int64  `json:"cache_creation_tokens"`
				TopModels           []struct {
					Model       string `json:"model"`
					TotalTokens int64  `json:"total_tokens"`
				} `json:"top_models"`
			} `json:"departments"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Departments, 2)
	require.Equal(t, "Application", envelope.Data.Departments[0].GroupName)
	require.Equal(t, int64(1), envelope.Data.Departments[0].Requests)
	require.Equal(t, int64(2), envelope.Data.Departments[0].ModelCount)
	require.Len(t, envelope.Data.Departments[0].TopModels, 1)
	require.Equal(t, int64(10), envelope.Data.Departments[0].TopModels[0].TotalTokens)
	require.Equal(t, "Platform", envelope.Data.Departments[1].GroupName)
	require.Equal(t, int64(3), envelope.Data.Departments[1].ActiveUserCount)
	require.Equal(t, int64(2), envelope.Data.Departments[1].StreamRequests)
	require.Len(t, envelope.Data.Departments[1].TopModels, 2)
	require.Equal(t, "claude-sonnet-4-5", envelope.Data.Departments[1].TopModels[0].Model)
	require.Equal(t, int64(15), envelope.Data.Departments[1].TopModels[0].TotalTokens)
	require.Equal(t, int64(5), envelope.Data.Departments[1].TopModels[1].TotalTokens)
}

func TestDepartmentUsageTrendReturnsTokenOnlySeries(t *testing.T) {
	repo := &userUsageRepoCapture{
		departmentTrend: []usagestats.DepartmentTrendPoint{
			{Bucket: "2026-09-01", Requests: 3, TotalTokens: 30},
			{Bucket: "2026-09-02", Requests: 4, TotalTokens: 40},
		},
		departmentModelTrend: []usagestats.ModelTrendPoint{
			{Bucket: "2026-09-01", Model: "claude-sonnet-4-5", TotalTokens: 20},
			{Bucket: "2026-09-02", Model: "claude-sonnet-4-5", TotalTokens: 30},
		},
	}
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	usageHandler := NewUsageHandler(usageSvc, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage/department-usage/trend", usageHandler.DepartmentUsageTrend)

	req := httptest.NewRequest(http.MethodGet, "/usage/department-usage/trend?start_date=2026-09-01&end_date=2026-09-07&granularity=week&user_id=99&group_id=77", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "cost")
	require.Equal(t, int64(0), repo.departmentTrendFilters.UserID)
	require.Equal(t, int64(0), repo.departmentTrendFilters.GroupID)
	require.Contains(t, rec.Body.String(), `"granularity":"week"`)
	require.Contains(t, rec.Body.String(), `"total_tokens":40`)
	require.Contains(t, rec.Body.String(), "claude-sonnet-4-5")
}

func TestDepartmentUsageHeatmapUsesRequesterTimezone(t *testing.T) {
	repo := &userUsageRepoCapture{
		departmentHeatmap: []usagestats.UsageHeatmapPoint{
			{Weekday: 1, Hour: 9, Requests: 5, TotalTokens: 50},
			{Weekday: 1, Hour: 10, Requests: 3, TotalTokens: 30},
		},
	}
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	usageHandler := NewUsageHandler(usageSvc, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage/department-usage/heatmap", usageHandler.DepartmentUsageHeatmap)

	req := httptest.NewRequest(http.MethodGet, "/usage/department-usage/heatmap?start_date=2026-09-01&end_date=2026-09-07&timezone=Asia/Shanghai&group_id=77&user_id=99", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "cost")
	require.Equal(t, "Asia/Shanghai", repo.departmentHeatmapTimezone)
	require.Equal(t, int64(0), repo.departmentHeatmapFilters.GroupID)
	require.Equal(t, int64(0), repo.departmentHeatmapFilters.UserID)
	require.Contains(t, rec.Body.String(), `"weekday":1`)
	require.Contains(t, rec.Body.String(), `"total_tokens":50`)
}

func TestDepartmentUsageHeatmapRejectsInvalidTimezone(t *testing.T) {
	repo := &userUsageRepoCapture{}
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	usageHandler := NewUsageHandler(usageSvc, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage/department-usage/heatmap", usageHandler.DepartmentUsageHeatmap)

	req := httptest.NewRequest(http.MethodGet, "/usage/department-usage/heatmap?start_date=2026-09-01&end_date=2026-09-07&timezone=Not/AZone", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, repo.departmentHeatmapTimezone)
}

func departmentNames(t *testing.T, body []byte) []string {
	t.Helper()
	var envelope struct {
		Data struct {
			Departments []struct {
				GroupName string `json:"group_name"`
			} `json:"departments"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))

	names := make([]string, 0, len(envelope.Data.Departments))
	for _, department := range envelope.Data.Departments {
		names = append(names, department.GroupName)
	}
	return names
}
