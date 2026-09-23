package handler

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type userUsageFilters struct {
	Filters   usagestats.UsageLogFilters
	StartTime time.Time
	EndTime   time.Time
}

type userModelStat struct {
	Model               string  `json:"model"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	Cost                float64 `json:"cost"`
	ActualCost          float64 `json:"actual_cost"`
}

type userGroupStat struct {
	GroupID     int64   `json:"group_id"`
	GroupName   string  `json:"group_name"`
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"total_tokens"`
	Cost        float64 `json:"cost"`
	ActualCost  float64 `json:"actual_cost"`
}

// departmentUsageStat is the deliberately narrow public team view. Keep cost
// fields out of this DTO so they cannot leak through the API response.
type departmentUsageStat struct {
	GroupID             int64             `json:"group_id"`
	GroupName           string            `json:"group_name"`
	Requests            int64             `json:"requests"`
	TotalTokens         int64             `json:"total_tokens"`
	InputTokens         int64             `json:"input_tokens"`
	OutputTokens        int64             `json:"output_tokens"`
	CacheCreationTokens int64             `json:"cache_creation_tokens"`
	CacheReadTokens     int64             `json:"cache_read_tokens"`
	ModelCount          int64             `json:"model_count"`
	ActiveUserCount     int64             `json:"active_user_count"`
	ImageCount          int64             `json:"image_count"`
	VideoCount          int64             `json:"video_count"`
	StreamRequests      int64             `json:"stream_requests"`
	AvgDurationMs       float64           `json:"avg_duration_ms"`
	AvgFirstTokenMs     float64           `json:"avg_first_token_ms"`
	TopModels           []departmentModel `json:"top_models"`
}

// departmentModel mirrors GroupModelStat without any cost information.
type departmentModel struct {
	Model       string `json:"model"`
	TotalTokens int64  `json:"total_tokens"`
}

// departmentTopModelLimit caps how many models each department row carries. The
// page shows them on demand, so a small fixed window keeps the payload flat.
const departmentTopModelLimit = 5

// departmentReasoningEffortBucket is one reasoning-effort tier inside a
// department, model, or time bucket. Cost fields stay out of this DTO as well.
type departmentReasoningEffortBucket struct {
	Effort              string  `json:"effort"`
	Requests            int64   `json:"requests"`
	TotalTokens         int64   `json:"total_tokens"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	AvgDurationMs       float64 `json:"avg_duration_ms"`
	AvgFirstTokenMs     float64 `json:"avg_first_token_ms"`
}

// departmentReasoningEffortRow holds the effort breakdown of one department,
// one model, or one time bucket.
type departmentReasoningEffortRow struct {
	GroupID       int64                             `json:"group_id"`
	GroupName     string                            `json:"group_name"`
	Model         string                            `json:"model"`
	Bucket        string                            `json:"bucket"`
	TotalRequests int64                             `json:"total_requests"`
	TotalTokens   int64                             `json:"total_tokens"`
	Efforts       []departmentReasoningEffortBucket `json:"efforts"`
}

// UsageHandler handles usage-related requests
type UsageHandler struct {
	usageService   *service.UsageService
	apiKeyService  *service.APIKeyService
	opsService     *service.OpsService
	settingService *service.SettingService
}

// NewUsageHandler creates a new UsageHandler
func NewUsageHandler(
	usageService *service.UsageService,
	apiKeyService *service.APIKeyService,
	opsService *service.OpsService,
	settingService *service.SettingService,
) *UsageHandler {
	return &UsageHandler{
		usageService:   usageService,
		apiKeyService:  apiKeyService,
		opsService:     opsService,
		settingService: settingService,
	}
}

func (h *UsageHandler) parseUserUsageFilters(c *gin.Context, requireRange bool) (*userUsageFilters, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return nil, false
	}

	parsed, ok := h.parseUserUsageDateRange(c, requireRange)
	if !ok {
		return nil, false
	}

	var apiKeyID int64
	if apiKeyIDStr := strings.TrimSpace(c.Query("api_key_id")); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return nil, false
		}
		if h.apiKeyService == nil {
			response.InternalError(c, "API key service not available")
			return nil, false
		}
		apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), id)
		if err != nil {
			response.ErrorFrom(c, err)
			return nil, false
		}
		if apiKey.UserID != subject.UserID {
			response.Forbidden(c, "Not authorized to access this API key's usage records")
			return nil, false
		}
		apiKeyID = id
	}

	var groupID int64
	if groupIDStr := strings.TrimSpace(c.Query("group_id")); groupIDStr != "" {
		id, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid group_id")
			return nil, false
		}
		groupID = id
	}

	var requestType *int16
	var stream *bool
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return nil, false
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := strings.TrimSpace(c.Query("stream")); streamStr != "" {
		val, err := strconv.ParseBool(streamStr)
		if err != nil {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return nil, false
		}
		stream = &val
	}

	var nativeCompactionV2 *bool
	if raw := strings.TrimSpace(c.Query("native_compaction_v2")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			response.BadRequest(c, "Invalid native_compaction_v2 value, use true or false")
			return nil, false
		}
		nativeCompactionV2 = &value
	}

	var billingType *int8
	if billingTypeStr := strings.TrimSpace(c.Query("billing_type")); billingTypeStr != "" {
		val, err := strconv.ParseInt(billingTypeStr, 10, 8)
		if err != nil {
			response.BadRequest(c, "Invalid billing_type")
			return nil, false
		}
		bt := int8(val)
		billingType = &bt
	}

	billingMode := strings.TrimSpace(c.Query("billing_mode"))
	if billingMode != "" && !service.BillingMode(billingMode).IsValidUsageFilter() {
		response.BadRequest(c, "Invalid billing_mode")
		return nil, false
	}

	parsed.Filters = usagestats.UsageLogFilters{
		UserID:             subject.UserID,
		APIKeyID:           apiKeyID,
		GroupID:            groupID,
		Model:              strings.TrimSpace(c.Query("model")),
		ModelFilterSource:  usagestats.ModelSourceRequested,
		RequestType:        requestType,
		Stream:             stream,
		NativeCompactionV2: nativeCompactionV2,
		BillingType:        billingType,
		BillingMode:        billingMode,
		StartTime:          parsed.Filters.StartTime,
		EndTime:            parsed.Filters.EndTime,
	}
	return parsed, true
}

func (h *UsageHandler) parseUserUsageDateRange(c *gin.Context, requireRange bool) (*userUsageFilters, bool) {
	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	var startTime, endTime time.Time
	var startPtr, endPtr *time.Time
	startDateStr := strings.TrimSpace(c.Query("start_date"))
	endDateStr := strings.TrimSpace(c.Query("end_date"))

	if startDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return nil, false
		}
		startTime = t
		startPtr = &startTime
	}
	if endDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return nil, false
		}
		endTime = t.AddDate(0, 0, 1)
		endPtr = &endTime
	}

	if requireRange {
		if startPtr == nil {
			switch c.DefaultQuery("period", "") {
			case "today":
				startTime = timezone.StartOfDayInUserLocation(now, userTZ)
			case "week":
				startTime = now.AddDate(0, 0, -7)
			case "month":
				startTime = now.AddDate(0, -1, 0)
			default:
				startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
			}
			startPtr = &startTime
		}
		if endPtr == nil {
			if strings.TrimSpace(c.Query("period")) != "" {
				endTime = now
			} else {
				endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
			}
			endPtr = &endTime
		}
	}

	return &userUsageFilters{
		Filters: usagestats.UsageLogFilters{
			StartTime: startPtr,
			EndTime:   endPtr,
		},
		StartTime: derefTime(startPtr),
		EndTime:   derefTime(endPtr),
	}, true
}

func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

// List handles listing usage records with pagination
// GET /api/v1/usage
func (h *UsageHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	parsed, ok := h.parseUserUsageFilters(c, false)
	if !ok {
		return
	}

	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}

	records, result, err := h.usageService.ListWithFilters(c.Request.Context(), params, parsed.Filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.UsageLog, 0, len(records))
	for i := range records {
		out = append(out, *dto.UsageLogFromService(&records[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// ListErrors handles listing the current user's failed requests (redacted).
// GET /api/v1/usage/errors
func (h *UsageHandler) ListErrors(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Visibility switch (fail-closed). Defense-in-depth: frontend also hides the tab.
	if h.settingService == nil || !h.settingService.IsUserErrorViewAllowed(c.Request.Context()) {
		response.Forbidden(c, "Error requests view is disabled")
		return
	}
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}

	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}

	filter := &service.OpsErrorLogFilter{Page: page, PageSize: pageSize}

	// Date range (half-open [start, end)), reuse usage-list semantics.
	userTZ := c.Query("timezone")
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return
		}
		filter.StartTime = &t
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return
		}
		t = t.AddDate(0, 0, 1)
		filter.EndTime = &t
	}

	filter.Model = strings.TrimSpace(c.Query("model"))

	if k := strings.TrimSpace(c.Query("api_key_id")); k != "" {
		n, err := strconv.ParseInt(k, 10, 64)
		if err != nil || n < 0 {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}
		if n > 0 {
			filter.APIKeyID = &n
		}
	}

	if sc := strings.TrimSpace(c.Query("status_code")); sc != "" {
		n, err := strconv.Atoi(sc)
		if err != nil || n < 0 {
			response.BadRequest(c, "Invalid status_code")
			return
		}
		filter.StatusCodes = []int{n}
	}

	if cat := strings.TrimSpace(c.Query("category")); cat != "" {
		phases, types := service.CategoryToFilter(cat)
		filter.ErrorPhasesAny = phases
		filter.ErrorTypesAny = types
	}

	// 排序对齐用量明细:列白名单与方向归一在 repo 层,非法值回退 created_at DESC。
	filter.SetSort(c.Query("sort_by"), c.Query("sort_order"))

	result, err := h.opsService.ListUserErrorRequests(c.Request.Context(), subject.UserID, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, result.Items, int64(result.Total), result.Page, result.PageSize)
}

// GetErrorDetail handles fetching one of the current user's failed-request details (redacted).
// GET /api/v1/usage/errors/:id
func (h *UsageHandler) GetErrorDetail(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.settingService == nil || !h.settingService.IsUserErrorViewAllowed(c.Request.Context()) {
		response.Forbidden(c, "Error requests view is disabled")
		return
	}
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return
	}
	detail, err := h.opsService.GetUserErrorRequestDetail(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, detail)
}

// GetByID handles getting a single usage record
// GET /api/v1/usage/:id
func (h *UsageHandler) GetByID(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	usageID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid usage ID")
		return
	}

	record, err := h.usageService.GetByID(c.Request.Context(), usageID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 验证所有权
	if record.UserID != subject.UserID {
		response.Forbidden(c, "Not authorized to access this record")
		return
	}

	response.Success(c, dto.UsageLogFromService(record))
}

// Stats handles getting usage statistics
// GET /api/v1/usage/stats
func (h *UsageHandler) Stats(c *gin.Context) {
	parsed, ok := h.parseUserUsageFilters(c, true)
	if !ok {
		return
	}

	stats, err := h.usageService.GetStatsWithFilters(c.Request.Context(), parsed.Filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	stats.TotalAccountCost = nil
	stats.UpstreamEndpoints = nil
	stats.EndpointPaths = nil

	response.Success(c, stats)
}

const (
	defaultAPIKeyDailyUsageDays = 30
	maxAPIKeyDailyUsageDays     = 90
)

func parseAPIKeyDailyUsageDays(raw string) (int, bool) {
	if strings.TrimSpace(raw) == "" {
		return defaultAPIKeyDailyUsageDays, true
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 || days > maxAPIKeyDailyUsageDays {
		return 0, false
	}
	return days, true
}

func apiKeyDailyUsageRange(days int, userTZ string) (time.Time, time.Time) {
	now := timezone.NowInUserLocation(userTZ)
	startTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -(days-1)), userTZ)
	endTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
	return startTime, endTime
}

// DashboardStats handles getting user dashboard statistics
// GET /api/v1/usage/dashboard/stats
func (h *UsageHandler) DashboardStats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	stats, err := h.usageService.GetUserDashboardStats(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}

// DashboardTrend handles getting user usage trend data
// GET /api/v1/usage/dashboard/trend
func (h *UsageHandler) DashboardTrend(c *gin.Context) {
	parsed, ok := h.parseUserUsageFilters(c, true)
	if !ok {
		return
	}
	granularity := c.DefaultQuery("granularity", "day")

	trend, err := h.usageService.GetUsageTrendWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, granularity, parsed.Filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"trend":       trend,
		"start_date":  parsed.StartTime.Format("2006-01-02"),
		"end_date":    parsed.EndTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity": granularity,
	})
}

// DashboardModels handles getting user model usage statistics
// GET /api/v1/usage/dashboard/models
func (h *UsageHandler) DashboardModels(c *gin.Context) {
	parsed, ok := h.parseUserUsageFilters(c, true)
	if !ok {
		return
	}

	modelSource := strings.TrimSpace(c.Query("model_source"))
	if modelSource != "" && modelSource != usagestats.ModelSourceRequested {
		response.BadRequest(c, "Invalid model_source, user usage only supports requested")
		return
	}

	stats, err := h.usageService.GetModelStatsWithFiltersBySource(c.Request.Context(), parsed.StartTime, parsed.EndTime, parsed.Filters, usagestats.ModelSourceRequested)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"models":     userModelStatsFromUsageStats(stats),
		"start_date": parsed.StartTime.Format("2006-01-02"),
		"end_date":   parsed.EndTime.Add(-24 * time.Hour).Format("2006-01-02"),
	})
}

// DashboardSnapshotV2 returns usage-page chart data scoped to the current user.
// GET /api/v1/usage/dashboard/snapshot-v2
func (h *UsageHandler) DashboardSnapshotV2(c *gin.Context) {
	parsed, ok := h.parseUserUsageFilters(c, true)
	if !ok {
		return
	}

	granularity := strings.TrimSpace(c.DefaultQuery("granularity", "day"))
	if granularity != "hour" {
		granularity = "day"
	}
	includeTrend, ok := parseBoolQueryWithDefault(c, "include_trend", true)
	if !ok {
		return
	}
	includeModels, ok := parseBoolQueryWithDefault(c, "include_model_stats", true)
	if !ok {
		return
	}
	includeGroups, ok := parseBoolQueryWithDefault(c, "include_group_stats", false)
	if !ok {
		return
	}

	resp := gin.H{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"start_date":   parsed.StartTime.Format("2006-01-02"),
		"end_date":     parsed.EndTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity":  granularity,
	}

	if includeTrend {
		trend, err := h.usageService.GetUsageTrendWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, granularity, parsed.Filters)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		resp["trend"] = trend
	}
	if includeModels {
		models, err := h.usageService.GetModelStatsWithFiltersBySource(c.Request.Context(), parsed.StartTime, parsed.EndTime, parsed.Filters, usagestats.ModelSourceRequested)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		resp["models"] = userModelStatsFromUsageStats(models)
	}
	if includeGroups {
		groups, err := h.usageService.GetGroupStatsWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, parsed.Filters)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		resp["groups"] = userGroupStatsFromUsageStats(groups)
	}

	response.Success(c, resp)
}

// DepartmentUsage returns aggregate token usage for every department visible
// to authenticated team members. It intentionally does not expose costs,
// request details, users, or API keys.
// GET /api/v1/usage/department-usage
func (h *UsageHandler) DepartmentUsage(c *gin.Context) {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	parsed, ok := h.parseUserUsageDateRange(c, true)
	if !ok {
		return
	}

	breakdown, err := h.usageService.GetDepartmentUsageBreakdownWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, parsed.Filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	topModels, err := h.usageService.GetDepartmentGroupModelStatsWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, parsed.Filters, departmentTopModelLimit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	modelsByGroup := make(map[int64][]departmentModel, len(breakdown))
	for _, model := range topModels {
		modelsByGroup[model.GroupID] = append(modelsByGroup[model.GroupID], departmentModel{
			Model:       model.Model,
			TotalTokens: model.TotalTokens,
		})
	}

	departments := make([]departmentUsageStat, 0, len(breakdown))
	for _, row := range breakdown {
		models := modelsByGroup[row.GroupID]
		if models == nil {
			models = []departmentModel{}
		}
		departments = append(departments, departmentUsageStat{
			GroupID:             row.GroupID,
			GroupName:           row.GroupName,
			Requests:            row.Requests,
			TotalTokens:         row.TotalTokens,
			InputTokens:         row.InputTokens,
			OutputTokens:        row.OutputTokens,
			CacheCreationTokens: row.CacheCreationTokens,
			CacheReadTokens:     row.CacheReadTokens,
			ModelCount:          row.ModelCount,
			ActiveUserCount:     row.ActiveUserCount,
			ImageCount:          row.ImageCount,
			VideoCount:          row.VideoCount,
			StreamRequests:      row.StreamRequests,
			AvgDurationMs:       row.AvgDurationMs,
			AvgFirstTokenMs:     row.AvgFirstTokenMs,
			TopModels:           models,
		})
	}
	// This is a directory-style summary, not a ranking. Keep a stable
	// alphabetical order so the page does not imply competition between teams.
	sort.SliceStable(departments, func(i, j int) bool {
		return strings.ToLower(departments[i].GroupName) < strings.ToLower(departments[j].GroupName)
	})

	summary, err := h.usageService.GetDepartmentUsageSummary(c.Request.Context(), parsed.StartTime, parsed.EndTime, parsed.Filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Coverage is derived from the group directory: active departments over all
	// adoption-candidate groups. Groups with no usage in the range are listed
	// separately so the table stays focused on actual usage.
	groups, err := h.usageService.ListDepartmentGroups(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	usedGroupIDs := make(map[int64]struct{}, len(breakdown))
	for _, row := range breakdown {
		usedGroupIDs[row.GroupID] = struct{}{}
	}
	unusedDepartments := make([]usagestats.UnusedDepartment, 0, len(groups))
	for _, group := range groups {
		if _, used := usedGroupIDs[group.GroupID]; !used {
			unusedDepartments = append(unusedDepartments, group)
		}
	}
	summary.TotalDepartments = int64(len(groups))

	response.Success(c, gin.H{
		"departments":        departments,
		"summary":            summary,
		"unused_departments": unusedDepartments,
	})
}

// DepartmentUsageTrend returns the token trend for team-wide reports: one
// series aggregated across all departments plus a per-model breakdown. Costs are
// never included.
// GET /api/v1/usage/department-usage/trend
func (h *UsageHandler) DepartmentUsageTrend(c *gin.Context) {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	parsed, ok := h.parseUserUsageDateRange(c, true)
	if !ok {
		return
	}
	granularity := c.DefaultQuery("granularity", "day")

	totalTrend, err := h.usageService.GetDepartmentUsageTrendWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, granularity, parsed.Filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	modelTrend, err := h.usageService.GetDepartmentModelTrendWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, granularity, parsed.Filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"granularity": granularity,
		"total_trend": totalTrend,
		"model_trend": modelTrend,
	})
}

// DepartmentUsageHeatmap returns weekday-by-hour request activity for the team
// usage report, bucketed in the requester's timezone. Costs are never included.
// GET /api/v1/usage/department-usage/heatmap
func (h *UsageHandler) DepartmentUsageHeatmap(c *gin.Context) {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	parsed, ok := h.parseUserUsageDateRange(c, true)
	if !ok {
		return
	}

	tz := strings.TrimSpace(c.Query("timezone"))
	if tz == "" || tz == "Local" {
		tz = timezone.Name()
	}
	if tz == "Local" {
		// Postgres has no "Local" zone; fall back to UTC when the server
		// timezone was never configured with an IANA name.
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		response.BadRequest(c, "Invalid timezone")
		return
	}

	points, err := h.usageService.GetDepartmentUsageHeatmapWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, parsed.Filters, tz)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"timezone": tz,
		"points":   points,
	})
}

// DepartmentClientSoftware returns the top client software products for the
// team (or a single department when group_id is set). The client software is
// extracted from the user_agent field. Costs are never included.
// GET /api/v1/usage/department-usage/client-software
func (h *UsageHandler) DepartmentClientSoftware(c *gin.Context) {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	parsed, ok := h.parseUserUsageDateRange(c, true)
	if !ok {
		return
	}

	var groupID int64
	if groupIDStr := strings.TrimSpace(c.Query("group_id")); groupIDStr != "" {
		id, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil || id < 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		groupID = id
	}
	// Team-facing report: only the department scope is accepted. User, API key,
	// and personal model filters are deliberately ignored so a member cannot
	// drill into another person's usage.
	filters := parsed.Filters
	filters.GroupID = groupID

	limit := 10
	if limitStr := strings.TrimSpace(c.DefaultQuery("limit", "10")); limitStr != "" {
		if limitVal, err := strconv.Atoi(limitStr); err == nil && limitVal > 0 && limitVal <= 50 {
			limit = limitVal
		}
	}

	stats, err := h.usageService.GetDepartmentClientSoftwareStats(c.Request.Context(), parsed.StartTime, parsed.EndTime, filters, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"clients": stats,
	})
}

// DepartmentModelStats returns the top models across the team for the team
// usage report. Costs are never included.
// GET /api/v1/usage/department-usage/models
func (h *UsageHandler) DepartmentModelStats(c *gin.Context) {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	parsed, ok := h.parseUserUsageDateRange(c, true)
	if !ok {
		return
	}

	limit := 10
	if limitStr := strings.TrimSpace(c.DefaultQuery("limit", "10")); limitStr != "" {
		if limitVal, err := strconv.Atoi(limitStr); err == nil && limitVal > 0 && limitVal <= 50 {
			limit = limitVal
		}
	}

	stats, err := h.usageService.GetDepartmentModelStats(c.Request.Context(), parsed.StartTime, parsed.EndTime, parsed.Filters, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"models": stats,
	})
}

// DepartmentReasoningEffort returns the GPT reasoning-effort mix for the team
// usage report: per department, per model, and over time. Costs are never
// included.
// GET /api/v1/usage/department-usage/reasoning
func (h *UsageHandler) DepartmentReasoningEffort(c *gin.Context) {
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	parsed, ok := h.parseUserUsageDateRange(c, true)
	if !ok {
		return
	}

	effortSource := strings.TrimSpace(c.DefaultQuery("effort_source", "effective"))
	if effortSource != "effective" && effortSource != "requested" {
		response.BadRequest(c, "Invalid effort_source, use effective or requested")
		return
	}
	modelScope := strings.TrimSpace(c.DefaultQuery("model_scope", "gpt"))
	if modelScope != "gpt" && modelScope != "all" {
		response.BadRequest(c, "Invalid model_scope, use gpt or all")
		return
	}

	var groupID int64
	if groupIDStr := strings.TrimSpace(c.Query("group_id")); groupIDStr != "" {
		id, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil || id < 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		groupID = id
	}
	// Team-facing report: only the department scope is accepted. User, API key,
	// and personal model filters are deliberately ignored so a member cannot
	// drill into another team's usage.
	filters := usagestats.UsageLogFilters{GroupID: groupID}
	granularity := c.DefaultQuery("granularity", "day")

	groupStats, err := h.usageService.GetDepartmentReasoningEffortGroupStatsWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, filters, effortSource, modelScope)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	modelFamily := c.DefaultQuery("model_family", "false") == "true"
	modelStats, err := h.usageService.GetDepartmentReasoningEffortModelStatsWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, filters, effortSource, modelScope, modelFamily)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	trendStats, err := h.usageService.GetDepartmentReasoningEffortTrendWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, granularity, filters, effortSource, modelScope)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"effort_source": effortSource,
		"model_scope":   modelScope,
		"model_family":  modelFamily,
		"granularity":   granularity,
		"efforts":       departmentReasoningEffortTiers(groupStats),
		"departments":   buildDepartmentReasoningEffortRows(groupStats, departmentReasoningEffortDimensionGroup),
		"models":        buildDepartmentReasoningEffortRows(modelStats, departmentReasoningEffortDimensionModel),
		"trend":         buildDepartmentReasoningEffortRows(trendStats, departmentReasoningEffortDimensionBucket),
	})
}

const (
	departmentReasoningEffortDimensionGroup  = "group"
	departmentReasoningEffortDimensionModel  = "model"
	departmentReasoningEffortDimensionBucket = "bucket"
)

// departmentReasoningEffortTiers lists the effort labels seen in the result,
// most used first, so every chart can share one legend order.
func departmentReasoningEffortTiers(stats []usagestats.ReasoningEffortStat) []string {
	totals := make(map[string]int64, len(stats))
	for _, stat := range stats {
		totals[normalizeDepartmentReasoningEffort(stat.Effort)] += stat.Requests
	}
	tiers := make([]string, 0, len(totals))
	for effort := range totals {
		tiers = append(tiers, effort)
	}
	sort.SliceStable(tiers, func(i, j int) bool {
		if totals[tiers[i]] != totals[tiers[j]] {
			return totals[tiers[i]] > totals[tiers[j]]
		}
		return tiers[i] < tiers[j]
	})
	return tiers
}

func normalizeDepartmentReasoningEffort(effort string) string {
	if trimmed := strings.TrimSpace(effort); trimmed != "" {
		return trimmed
	}
	return usagestats.UnspecifiedReasoningEffort
}

// buildDepartmentReasoningEffortRows folds the flat query result into one row
// per dimension value with its effort tiers. Combined averages are weighted by
// request count because each input row is already an average.
func buildDepartmentReasoningEffortRows(stats []usagestats.ReasoningEffortStat, dimension string) []departmentReasoningEffortRow {
	type entry struct {
		row      departmentReasoningEffortRow
		byEffort map[string]*departmentReasoningEffortBucket
	}

	entries := make(map[string]*entry, len(stats))
	order := make([]string, 0, len(stats))

	for _, stat := range stats {
		var key string
		row := departmentReasoningEffortRow{}
		switch dimension {
		case departmentReasoningEffortDimensionGroup:
			key = strconv.FormatInt(stat.GroupID, 10)
			row.GroupID = stat.GroupID
			row.GroupName = stat.GroupName
		case departmentReasoningEffortDimensionModel:
			key = stat.Model
			row.Model = stat.Model
		default:
			key = stat.Bucket
			row.Bucket = stat.Bucket
		}

		item, ok := entries[key]
		if !ok {
			item = &entry{row: row, byEffort: make(map[string]*departmentReasoningEffortBucket)}
			entries[key] = item
			order = append(order, key)
		}

		effort := normalizeDepartmentReasoningEffort(stat.Effort)
		bucket, ok := item.byEffort[effort]
		if !ok {
			bucket = &departmentReasoningEffortBucket{Effort: effort}
			item.byEffort[effort] = bucket
		}

		bucket.Requests += stat.Requests
		bucket.TotalTokens += stat.TotalTokens
		bucket.InputTokens += stat.InputTokens
		bucket.OutputTokens += stat.OutputTokens
		bucket.CacheCreationTokens += stat.CacheCreationTokens
		bucket.CacheReadTokens += stat.CacheReadTokens
		bucket.AvgDurationMs += stat.AvgDurationMs * float64(stat.Requests)
		bucket.AvgFirstTokenMs += stat.AvgFirstTokenMs * float64(stat.Requests)
		item.row.TotalRequests += stat.Requests
		item.row.TotalTokens += stat.TotalTokens
	}

	rows := make([]departmentReasoningEffortRow, 0, len(order))
	for _, key := range order {
		item := entries[key]
		efforts := make([]departmentReasoningEffortBucket, 0, len(item.byEffort))
		for _, bucket := range item.byEffort {
			if bucket.Requests > 0 {
				bucket.AvgDurationMs /= float64(bucket.Requests)
				bucket.AvgFirstTokenMs /= float64(bucket.Requests)
			}
			efforts = append(efforts, *bucket)
		}
		sort.SliceStable(efforts, func(i, j int) bool {
			if efforts[i].Requests != efforts[j].Requests {
				return efforts[i].Requests > efforts[j].Requests
			}
			return efforts[i].Effort < efforts[j].Effort
		})
		item.row.Efforts = efforts
		rows = append(rows, item.row)
	}

	if dimension == departmentReasoningEffortDimensionBucket {
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Bucket < rows[j].Bucket })
		return rows
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].TotalRequests != rows[j].TotalRequests {
			return rows[i].TotalRequests > rows[j].TotalRequests
		}
		return strings.ToLower(rows[i].GroupName+rows[i].Model) < strings.ToLower(rows[j].GroupName+rows[j].Model)
	})
	return rows
}

func userModelStatsFromUsageStats(stats []usagestats.ModelStat) []userModelStat {
	out := make([]userModelStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, userModelStat{
			Model:               stat.Model,
			Requests:            stat.Requests,
			InputTokens:         stat.InputTokens,
			OutputTokens:        stat.OutputTokens,
			CacheCreationTokens: stat.CacheCreationTokens,
			CacheReadTokens:     stat.CacheReadTokens,
			TotalTokens:         stat.TotalTokens,
			Cost:                stat.Cost,
			ActualCost:          stat.ActualCost,
		})
	}
	return out
}

func userGroupStatsFromUsageStats(stats []usagestats.GroupStat) []userGroupStat {
	out := make([]userGroupStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, userGroupStat{
			GroupID:     stat.GroupID,
			GroupName:   stat.GroupName,
			Requests:    stat.Requests,
			TotalTokens: stat.TotalTokens,
			Cost:        stat.Cost,
			ActualCost:  stat.ActualCost,
		})
	}
	return out
}

func parseBoolQueryWithDefault(c *gin.Context, key string, fallback bool) (bool, bool) {
	raw := c.Query(key)
	if strings.TrimSpace(raw) == "" {
		return fallback, true
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		response.BadRequest(c, "Invalid "+key+" value, use true or false")
		return false, false
	}
	return parsed, true
}

// BatchAPIKeysUsageRequest represents the request for batch API keys usage
type BatchAPIKeysUsageRequest struct {
	APIKeyIDs []int64 `json:"api_key_ids" binding:"required"`
}

// DashboardAPIKeysUsage handles getting usage stats for user's own API keys
// POST /api/v1/usage/dashboard/api-keys-usage
func (h *UsageHandler) DashboardAPIKeysUsage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req BatchAPIKeysUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if len(req.APIKeyIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	// Limit the number of API key IDs to prevent SQL parameter overflow
	if len(req.APIKeyIDs) > 100 {
		response.BadRequest(c, "Too many API key IDs (maximum 100 allowed)")
		return
	}

	validAPIKeyIDs, err := h.apiKeyService.VerifyOwnership(c.Request.Context(), subject.UserID, req.APIKeyIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if len(validAPIKeyIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	stats, err := h.usageService.GetBatchAPIKeyUsageStats(c.Request.Context(), validAPIKeyIDs, time.Time{}, time.Time{})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"stats": stats})
}

// GetMyAPIKeyDailyUsage handles getting daily usage details for the current user's API key.
// GET /api/v1/user/api-keys/:id/usage/daily?days=30
func (h *UsageHandler) GetMyAPIKeyDailyUsage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	apiKeyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	days, ok := parseAPIKeyDailyUsageDays(c.DefaultQuery("days", ""))
	if !ok {
		response.BadRequest(c, "Invalid days, allowed range is 1-90")
		return
	}

	if h.apiKeyService == nil {
		response.InternalError(c, "API key service is not configured")
		return
	}

	apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), apiKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if apiKey.UserID != subject.UserID {
		response.Forbidden(c, "Not authorized to access this API key's usage")
		return
	}

	userTZ := c.Query("timezone")
	startTime, endTime := apiKeyDailyUsageRange(days, userTZ)
	items, err := h.usageService.GetAPIKeyDailyUsage(c.Request.Context(), subject.UserID, apiKeyID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"items":      items,
		"days":       days,
		"start_date": startTime.Format("2006-01-02"),
		"end_date":   endTime.AddDate(0, 0, -1).Format("2006-01-02"),
	})
}
