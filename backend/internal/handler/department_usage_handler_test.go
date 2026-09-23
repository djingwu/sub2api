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

func departmentReasoningRouter(repo *userUsageRepoCapture) *gin.Engine {
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	usageHandler := NewUsageHandler(usageSvc, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage/department-usage/reasoning", usageHandler.DepartmentReasoningEffort)
	return router
}

func TestDepartmentReasoningEffortDefaultsToGPTAndEffective(t *testing.T) {
	repo := &userUsageRepoCapture{
		departmentReasoningGroupStats: []usagestats.ReasoningEffortStat{
			{GroupID: 2, GroupName: "Platform", Effort: "high", Requests: 2, TotalTokens: 20, AvgDurationMs: 1000, AvgFirstTokenMs: 100},
			{GroupID: 2, GroupName: "Platform", Effort: "high", Requests: 1, TotalTokens: 10, AvgDurationMs: 400, AvgFirstTokenMs: 40},
			{GroupID: 2, GroupName: "Platform", Effort: "low", Requests: 1, TotalTokens: 5, AvgDurationMs: 200, AvgFirstTokenMs: 20},
			{GroupID: 1, GroupName: "Application", Effort: "medium", Requests: 2, TotalTokens: 20, AvgDurationMs: 500, AvgFirstTokenMs: 50},
			{GroupID: 1, GroupName: "Application", Effort: "", Requests: 1, TotalTokens: 10},
		},
		departmentReasoningModelStats: []usagestats.ReasoningEffortStat{
			{Model: "gpt-5.5", Effort: "high", Requests: 2, TotalTokens: 20},
			{Model: "gpt-5.6-sol", Effort: "low", Requests: 5, TotalTokens: 50},
		},
		departmentReasoningTrendStats: []usagestats.ReasoningEffortStat{
			{Bucket: "2026-09-02", Effort: "high", Requests: 1, TotalTokens: 10},
			{Bucket: "2026-09-01", Effort: "low", Requests: 1, TotalTokens: 5},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/usage/department-usage/reasoning?start_date=2026-09-01&end_date=2026-09-07&user_id=99&api_key_id=3", nil)
	rec := httptest.NewRecorder()
	departmentReasoningRouter(repo).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "cost")
	require.Equal(t, "effective", repo.departmentReasoningGroupSource)
	require.Equal(t, "gpt", repo.departmentReasoningGroupModelScope)
	require.Equal(t, "day", repo.departmentReasoningTrendGranularity)
	require.Equal(t, int64(0), repo.departmentReasoningGroupFilters.UserID)
	require.Equal(t, int64(0), repo.departmentReasoningGroupFilters.APIKeyID)
	require.Equal(t, int64(0), repo.departmentReasoningGroupFilters.GroupID)
	require.Equal(t, "", repo.departmentReasoningGroupFilters.Model)

	var envelope struct {
		Data struct {
			EffortSource string   `json:"effort_source"`
			ModelScope   string   `json:"model_scope"`
			Granularity  string   `json:"granularity"`
			Efforts      []string `json:"efforts"`
			Departments  []struct {
				GroupID       int64 `json:"group_id"`
				TotalRequests int64 `json:"total_requests"`
				Efforts       []struct {
					Effort        string  `json:"effort"`
					Requests      int64   `json:"requests"`
					AvgDurationMs float64 `json:"avg_duration_ms"`
				} `json:"efforts"`
			} `json:"departments"`
			Models []struct {
				Model string `json:"model"`
			} `json:"models"`
			Trend []struct {
				Bucket string `json:"bucket"`
			} `json:"trend"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, "effective", envelope.Data.EffortSource)
	require.Equal(t, "gpt", envelope.Data.ModelScope)
	require.Equal(t, []string{"high", "medium", "low", "unspecified"}, envelope.Data.Efforts)

	require.Len(t, envelope.Data.Departments, 2)
	require.Equal(t, int64(2), envelope.Data.Departments[0].GroupID)
	require.Equal(t, int64(4), envelope.Data.Departments[0].TotalRequests)
	require.Len(t, envelope.Data.Departments[0].Efforts, 2)
	require.Equal(t, "high", envelope.Data.Departments[0].Efforts[0].Effort)
	require.Equal(t, 800.0, envelope.Data.Departments[0].Efforts[0].AvgDurationMs)
	require.Equal(t, int64(1), envelope.Data.Departments[1].GroupID)
	require.Equal(t, "unspecified", envelope.Data.Departments[1].Efforts[1].Effort)

	require.Len(t, envelope.Data.Models, 2)
	require.Equal(t, "gpt-5.6-sol", envelope.Data.Models[0].Model)
	require.Len(t, envelope.Data.Trend, 2)
	require.Equal(t, "2026-09-01", envelope.Data.Trend[0].Bucket)
	require.Equal(t, "2026-09-02", envelope.Data.Trend[1].Bucket)
}

func TestDepartmentReasoningEffortScopesToDepartmentAndModel(t *testing.T) {
	repo := &userUsageRepoCapture{}

	req := httptest.NewRequest(http.MethodGet, "/usage/department-usage/reasoning?start_date=2026-09-01&end_date=2026-09-07&group_id=5&model=gpt-5.6-sol&effort_source=requested&model_scope=all&granularity=week", nil)
	rec := httptest.NewRecorder()
	departmentReasoningRouter(repo).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(5), repo.departmentReasoningGroupFilters.GroupID)
	require.Equal(t, "gpt-5.6-sol", repo.departmentReasoningGroupFilters.Model)
	require.Equal(t, usagestats.ModelSourceRequested, repo.departmentReasoningGroupFilters.ModelFilterSource)
	require.Equal(t, "effective", repo.departmentReasoningGroupSource)
	require.Equal(t, "gpt", repo.departmentReasoningGroupModelScope)
	require.Equal(t, "week", repo.departmentReasoningTrendGranularity)
	require.Contains(t, rec.Body.String(), `"effort_source":"effective"`)
	require.Contains(t, rec.Body.String(), `"model_scope":"gpt"`)
	require.Contains(t, rec.Body.String(), `"model":"gpt-5.6-sol"`)
	require.Contains(t, rec.Body.String(), `"departments":[]`)
	require.Contains(t, rec.Body.String(), `"models":[]`)
	require.Contains(t, rec.Body.String(), `"trend":[]`)
}

func TestDepartmentReasoningEffortRejectsInvalidOptions(t *testing.T) {
	repo := &userUsageRepoCapture{}

	cases := []string{
		"effort_source=weird",
		"model_scope=weird",
		"group_id=not-a-number",
	}
	for _, query := range cases {
		req := httptest.NewRequest(http.MethodGet, "/usage/department-usage/reasoning?start_date=2026-09-01&end_date=2026-09-07&"+query, nil)
		rec := httptest.NewRecorder()
		departmentReasoningRouter(repo).ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, query)
	}
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

func TestDepartmentUsageIncludesSummaryWithoutCosts(t *testing.T) {
	repo := &userUsageRepoCapture{
		departmentBreakdown: []usagestats.GroupUsageBreakdown{
			{GroupID: 1, GroupName: "Application", Requests: 1, TotalTokens: 10},
		},
		departmentSummary: &usagestats.DepartmentUsageSummary{
			TotalRequests:     7,
			TotalTokens:       70,
			ActiveDepartments: 3,
			ActiveUsers:       5,
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

	req := httptest.NewRequest(http.MethodGet, "/usage/department-usage?start_date=2026-09-01&end_date=2026-09-07", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "cost")
	require.Equal(t, int64(0), repo.departmentSummaryFilters.UserID)
	require.Equal(t, int64(0), repo.departmentSummaryFilters.GroupID)

	var envelope struct {
		Data struct {
			Summary struct {
				TotalRequests     int64 `json:"total_requests"`
				TotalTokens       int64 `json:"total_tokens"`
				ActiveDepartments int64 `json:"active_departments"`
				ActiveUsers       int64 `json:"active_users"`
			} `json:"summary"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, int64(7), envelope.Data.Summary.TotalRequests)
	require.Equal(t, int64(70), envelope.Data.Summary.TotalTokens)
	require.Equal(t, int64(3), envelope.Data.Summary.ActiveDepartments)
	require.Equal(t, int64(5), envelope.Data.Summary.ActiveUsers)
}

func TestDepartmentClientSoftwareScopesToGroupAndOmitsCosts(t *testing.T) {
	repo := &userUsageRepoCapture{
		departmentClientSoftware: []usagestats.ClientSoftwareStat{
			{ClientSoftware: "claude-cli", Requests: 12, TotalTokens: 120, UserCount: 4, DepartmentCount: 2},
			{ClientSoftware: "codex_cli_rs", Requests: 5, TotalTokens: 50, UserCount: 2, DepartmentCount: 1},
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
	router.GET("/usage/department-usage/client-software", usageHandler.DepartmentClientSoftware)

	req := httptest.NewRequest(http.MethodGet, "/usage/department-usage/client-software?start_date=2026-09-01&end_date=2026-09-07&user_id=99&api_key_id=3&group_id=5&limit=3", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "cost")
	require.Equal(t, int64(0), repo.departmentClientSoftwareFilters.UserID)
	require.Equal(t, int64(0), repo.departmentClientSoftwareFilters.APIKeyID)
	require.Equal(t, int64(5), repo.departmentClientSoftwareFilters.GroupID)
	require.Equal(t, 3, repo.departmentClientSoftwareLimit)

	var envelope struct {
		Data struct {
			Clients []struct {
				ClientSoftware  string `json:"client_software"`
				Requests        int64  `json:"requests"`
				TotalTokens     int64  `json:"total_tokens"`
				UserCount       int64  `json:"user_count"`
				DepartmentCount int64  `json:"department_count"`
			} `json:"clients"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Clients, 2)
	require.Equal(t, "claude-cli", envelope.Data.Clients[0].ClientSoftware)
	require.Equal(t, int64(4), envelope.Data.Clients[0].UserCount)
	require.Equal(t, int64(2), envelope.Data.Clients[0].DepartmentCount)
}

func TestDepartmentClientSoftwareRejectsInvalidGroupID(t *testing.T) {
	repo := &userUsageRepoCapture{}
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	usageHandler := NewUsageHandler(usageSvc, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage/department-usage/client-software", usageHandler.DepartmentClientSoftware)

	for _, query := range []string{"group_id=not-a-number", "group_id=-1"} {
		req := httptest.NewRequest(http.MethodGet, "/usage/department-usage/client-software?start_date=2026-09-01&end_date=2026-09-07&"+query, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, query)
	}
}

func TestDepartmentUsageReportsUnusedDepartmentsWithoutCosts(t *testing.T) {
	repo := &userUsageRepoCapture{
		departmentBreakdown: []usagestats.GroupUsageBreakdown{
			{GroupID: 1, GroupName: "Application", Requests: 1, TotalTokens: 10},
		},
		departmentGroups: []usagestats.UnusedDepartment{
			{GroupID: 1, GroupName: "Application"},
			{GroupID: 2, GroupName: "Platform"},
			{GroupID: 3, GroupName: "Research"},
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

	req := httptest.NewRequest(http.MethodGet, "/usage/department-usage?start_date=2026-09-01&end_date=2026-09-07", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "cost")
	require.True(t, repo.departmentGroupsFetched)

	var envelope struct {
		Data struct {
			Summary struct {
				TotalDepartments int64 `json:"total_departments"`
			} `json:"summary"`
			UnusedDepartments []struct {
				GroupID   int64  `json:"group_id"`
				GroupName string `json:"group_name"`
			} `json:"unused_departments"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, int64(3), envelope.Data.Summary.TotalDepartments)
	require.Len(t, envelope.Data.UnusedDepartments, 2)
	require.Equal(t, int64(2), envelope.Data.UnusedDepartments[0].GroupID)
	require.Equal(t, "Platform", envelope.Data.UnusedDepartments[0].GroupName)
	require.Equal(t, "Research", envelope.Data.UnusedDepartments[1].GroupName)
}

func TestDepartmentModelStatsReturnsModelsWithoutCosts(t *testing.T) {
	repo := &userUsageRepoCapture{
		departmentModelStats: []usagestats.DepartmentModelStat{
			{Model: "claude-sonnet-4-5", Requests: 9, TotalTokens: 90, UserCount: 3},
			{Model: "gpt-5", Requests: 4, TotalTokens: 40, UserCount: 2},
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
	router.GET("/usage/department-usage/models", usageHandler.DepartmentModelStats)

	req := httptest.NewRequest(http.MethodGet, "/usage/department-usage/models?start_date=2026-09-01&end_date=2026-09-07&user_id=99&model=should-be-ignored&limit=2", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "cost")
	require.Equal(t, int64(0), repo.departmentModelStatsFilters.UserID)
	require.Equal(t, int64(0), repo.departmentModelStatsFilters.APIKeyID)
	require.Equal(t, int64(0), repo.departmentModelStatsFilters.GroupID)
	require.Equal(t, 2, repo.departmentModelStatsLimit)

	var envelope struct {
		Data struct {
			Models []struct {
				Model       string `json:"model"`
				Requests    int64  `json:"requests"`
				TotalTokens int64  `json:"total_tokens"`
				UserCount   int64  `json:"user_count"`
			} `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Models, 2)
	require.Equal(t, "claude-sonnet-4-5", envelope.Data.Models[0].Model)
	require.Equal(t, int64(90), envelope.Data.Models[0].TotalTokens)
	require.Equal(t, int64(3), envelope.Data.Models[0].UserCount)
}

func TestDepartmentUsageRejectsUnauthenticatedAccess(t *testing.T) {
	repo := &userUsageRepoCapture{}
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	usageHandler := NewUsageHandler(usageSvc, nil, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/usage/department-usage/client-software", usageHandler.DepartmentClientSoftware)
	router.GET("/usage/department-usage/models", usageHandler.DepartmentModelStats)

	for _, path := range []string{"/usage/department-usage/client-software", "/usage/department-usage/models"} {
		req := httptest.NewRequest(http.MethodGet, path+"?start_date=2026-09-01&end_date=2026-09-07", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code, path)
	}
}
