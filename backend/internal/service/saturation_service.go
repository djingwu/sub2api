package service

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// SettingKeyUserSaturationConfig stores the admin-tunable saturation report
// configuration (baseline combinations + compliance threshold) as JSON.
const SettingKeyUserSaturationConfig = "user_saturation_config"

const (
	// DefaultSaturationThresholdPercent is the default share of window cost that
	// must flow through the baseline combinations for a user to count as
	// compliant.
	DefaultSaturationThresholdPercent = 50.0
	saturationCycleDays               = 30.0
	saturationDefaultPageSize         = 20
	saturationMaxPageSize             = 200
	// SaturationDefaultTrendDays is the default lookback for the daily trend.
	saturationDefaultTrendDays = 30
	// SaturationMaxTrendDays caps the daily trend lookback.
	saturationMaxTrendDays = 90
)

// ErrSaturationUnsupported is returned when the configured repository does not
// implement the saturation aggregates (e.g. a stub used in tests).
var ErrSaturationUnsupported = infraerrors.InternalServer("SATURATION_UNSUPPORTED", "saturation reporting is not supported by this repository")

// DefaultSaturationBaselineCombos returns the best-value combinations used
// until an admin saves a custom configuration.
func DefaultSaturationBaselineCombos() []usagestats.SaturationBaselineCombo {
	return []usagestats.SaturationBaselineCombo{
		{Model: "gpt-5.6-luna", Effort: "max"},
		{Model: "gpt-6-astra", Effort: "medium"},
	}
}

// SaturationUserQuery is the filter/sort/page input for the saturation user
// list. Zero values keep the defaults (current metric, used_percent desc).
type SaturationUserQuery struct {
	Metric     string
	Band       string
	Compliance string
	Search     string
	Sort       string
	Order      string
	Page       int
	PageSize   int
}

// SaturationService builds the user saturation report. The whole report is
// derived from two aggregates: quota holders with their subscription counters
// (current window) and their subscription-billed usage logs (cumulative +
// model/effort mix), so self-service window resets never erase history.
type SaturationService struct {
	usageRepo   UsageLogRepository
	settingRepo SettingRepository
}

// NewSaturationService creates the saturation report service.
func NewSaturationService(usageRepo UsageLogRepository, settingRepo SettingRepository) *SaturationService {
	return &SaturationService{usageRepo: usageRepo, settingRepo: settingRepo}
}

type saturationUserAggregatesRepo interface {
	GetSaturationUserAggregates(ctx context.Context, scopeUserIDs []int64) ([]usagestats.SaturationUserAggregate, error)
}

type saturationUserCombosRepo interface {
	GetSaturationUserCombos(ctx context.Context, scopeUserIDs []int64) ([]usagestats.SaturationUserCombo, error)
}

type saturationDailyTrendRepo interface {
	GetSaturationDailyTrend(ctx context.Context, days int) ([]usagestats.SaturationTrendPoint, error)
}

type saturationReport struct {
	snapshot *usagestats.SaturationSnapshot
	users    []usagestats.SaturationUser
}

// GetConfig returns the stored configuration, falling back to the defaults when
// nothing valid has been saved yet.
func (s *SaturationService) GetConfig(ctx context.Context) usagestats.SaturationConfig {
	if s.settingRepo == nil {
		return defaultSaturationConfig()
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyUserSaturationConfig)
	if err != nil || strings.TrimSpace(raw) == "" {
		return defaultSaturationConfig()
	}
	var cfg usagestats.SaturationConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return defaultSaturationConfig()
	}
	if !normalizeSaturationConfig(&cfg) {
		return defaultSaturationConfig()
	}
	return cfg
}

// UpdateConfig validates and persists a new configuration.
func (s *SaturationService) UpdateConfig(ctx context.Context, cfg usagestats.SaturationConfig) (usagestats.SaturationConfig, error) {
	if !normalizeSaturationConfig(&cfg) {
		return usagestats.SaturationConfig{}, infraerrors.BadRequest("SATURATION_INVALID_CONFIG", "baseline combinations must not be empty and threshold_percent must be within (0, 100]")
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return usagestats.SaturationConfig{}, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyUserSaturationConfig, string(raw)); err != nil {
		return usagestats.SaturationConfig{}, err
	}
	return cfg, nil
}

func defaultSaturationConfig() usagestats.SaturationConfig {
	return usagestats.SaturationConfig{
		BaselineCombos:   DefaultSaturationBaselineCombos(),
		ThresholdPercent: DefaultSaturationThresholdPercent,
	}
}

// normalizeSaturationConfig trims, dedupes and validates the config in place.
// It returns false when the config must be rejected.
func normalizeSaturationConfig(cfg *usagestats.SaturationConfig) bool {
	if cfg == nil || len(cfg.BaselineCombos) == 0 {
		return false
	}
	if math.IsNaN(cfg.ThresholdPercent) || cfg.ThresholdPercent <= 0 || cfg.ThresholdPercent > 100 {
		return false
	}
	seen := make(map[string]struct{}, len(cfg.BaselineCombos))
	combos := make([]usagestats.SaturationBaselineCombo, 0, len(cfg.BaselineCombos))
	for _, combo := range cfg.BaselineCombos {
		model := strings.TrimSpace(combo.Model)
		effort := strings.TrimSpace(combo.Effort)
		if model == "" {
			return false
		}
		key := saturationComboKey(model, effort)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		combos = append(combos, usagestats.SaturationBaselineCombo{Model: model, Effort: effort})
	}
	cfg.BaselineCombos = combos
	return len(combos) > 0
}

func saturationComboKey(model, effort string) string {
	return strings.ToLower(strings.TrimSpace(model)) + "\x00" + strings.ToLower(strings.TrimSpace(effort))
}

func saturationBand(value float64) string {
	switch {
	case value < 20:
		return usagestats.SaturationBandDormant
	case value < 50:
		return usagestats.SaturationBandLight
	case value < 80:
		return usagestats.SaturationBandActive
	default:
		return usagestats.SaturationBandSaturated
	}
}

func saturationBandLabel(key string) (min float64, max *float64) {
	floatPtr := func(v float64) *float64 { return &v }
	switch key {
	case usagestats.SaturationBandLight:
		return 20, floatPtr(50)
	case usagestats.SaturationBandActive:
		return 50, floatPtr(80)
	case usagestats.SaturationBandSaturated:
		return 80, nil
	default:
		return 0, floatPtr(20)
	}
}

// GetSnapshot returns the dashboard snapshot over every quota holder.
func (s *SaturationService) GetSnapshot(ctx context.Context) (*usagestats.SaturationSnapshot, error) {
	report, err := s.buildReport(ctx)
	if err != nil {
		return nil, err
	}
	return report.snapshot, nil
}

// ListUsers returns the filtered, sorted and paginated user rows of the same
// report.
func (s *SaturationService) ListUsers(ctx context.Context, query SaturationUserQuery) (*usagestats.SaturationUserList, error) {
	report, err := s.buildReport(ctx)
	if err != nil {
		return nil, err
	}
	users := filterSaturationUsers(report.users, query)
	sortSaturationUsers(users, query)

	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = saturationDefaultPageSize
	}
	if pageSize > saturationMaxPageSize {
		pageSize = saturationMaxPageSize
	}
	start := (page - 1) * pageSize
	if start > len(users) {
		start = len(users)
	}
	end := start + pageSize
	if end > len(users) {
		end = len(users)
	}
	items := users[start:end]
	if items == nil {
		items = []usagestats.SaturationUser{}
	}
	return &usagestats.SaturationUserList{Items: items, Total: int64(len(users)), Page: page, PageSize: pageSize}, nil
}

// GetDailyTrend returns the last days days (default 30, capped at 90) of
// subscription-billed usage per day, backfilled with zeros.
func (s *SaturationService) GetDailyTrend(ctx context.Context, days int) ([]usagestats.SaturationTrendPoint, error) {
	if days <= 0 {
		days = saturationDefaultTrendDays
	}
	if days > saturationMaxTrendDays {
		days = saturationMaxTrendDays
	}
	repo, ok := s.usageRepo.(saturationDailyTrendRepo)
	if !ok {
		return nil, ErrSaturationUnsupported
	}
	points, err := repo.GetSaturationDailyTrend(ctx, days)
	if err != nil {
		return nil, err
	}
	if points == nil {
		points = []usagestats.SaturationTrendPoint{}
	}
	return points, nil
}

func (s *SaturationService) buildReport(ctx context.Context) (*saturationReport, error) {
	config := s.GetConfig(ctx)

	aggregatesRepo, ok := s.usageRepo.(saturationUserAggregatesRepo)
	if !ok {
		return nil, ErrSaturationUnsupported
	}
	combosRepo, ok := s.usageRepo.(saturationUserCombosRepo)
	if !ok {
		return nil, ErrSaturationUnsupported
	}

	rows, err := aggregatesRepo.GetSaturationUserAggregates(ctx, nil)
	if err != nil {
		return nil, err
	}
	combos, err := combosRepo.GetSaturationUserCombos(ctx, nil)
	if err != nil {
		return nil, err
	}

	report := buildSaturationReport(config, rows, combos)
	return report, nil
}

type saturationComboAgg struct {
	model    string
	effort   string
	requests int64
	tokens   int64
	costUSD  float64
	users    map[int64]struct{}
}

func buildSaturationReport(config usagestats.SaturationConfig, rows []usagestats.SaturationUserAggregate, combos []usagestats.SaturationUserCombo) *saturationReport {
	now := time.Now()
	baseline := make(map[string]struct{}, len(config.BaselineCombos))
	for _, combo := range config.BaselineCombos {
		baseline[saturationComboKey(combo.Model, combo.Effort)] = struct{}{}
	}

	knownUsers := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		knownUsers[row.UserID] = struct{}{}
	}

	combosByUser := make(map[int64][]usagestats.SaturationUserCombo, len(rows))
	comboAggs := make(map[string]*saturationComboAgg)
	var totalWindowCost, totalWindowTokens float64
	var baselineCost, baselineTokens float64

	for _, combo := range combos {
		if _, ok := knownUsers[combo.UserID]; !ok {
			continue
		}
		combosByUser[combo.UserID] = append(combosByUser[combo.UserID], combo)

		key := saturationComboKey(combo.Model, combo.Effort)
		agg, ok := comboAggs[key]
		if !ok {
			agg = &saturationComboAgg{
				model:  combo.Model,
				effort: combo.Effort,
				users:  make(map[int64]struct{}),
			}
			comboAggs[key] = agg
		}
		agg.requests += combo.Requests
		agg.tokens += combo.Tokens
		agg.costUSD += combo.CostUSD
		agg.users[combo.UserID] = struct{}{}

		totalWindowCost += combo.CostUSD
		totalWindowTokens += float64(combo.Tokens)
		if _, isBaseline := baseline[key]; isBaseline {
			baselineCost += combo.CostUSD
			baselineTokens += float64(combo.Tokens)
		}
	}

	users := make([]usagestats.SaturationUser, 0, len(rows))
	summary := usagestats.SaturationSummary{
		UserCount: int64(len(rows)),
	}

	var sumCurrentPct, sumLifetimePct float64
	var usersWithUsage int64
	currentPcts := make([]float64, 0, len(rows))
	lifetimePcts := make([]float64, 0, len(rows))
	currentUsed := make([]float64, 0, len(rows))
	lifetimeUsed := make([]float64, 0, len(rows))

	for _, row := range rows {
		user := usagestats.SaturationUser{
			UserID:           row.UserID,
			Email:            row.Email,
			Username:         row.Username,
			Status:           row.Status,
			PrimaryDeptID:    row.PrimaryDeptID,
			DeptName:         row.DeptName,
			GroupNames:       row.GroupNames,
			QuotaUSD:         row.QuotaUSD,
			UsedUSD:          row.UsedUSD,
			DailyUsedUSD:     row.DailyUsedUSD,
			WeeklyUsedUSD:    row.WeeklyUsedUSD,
			LifetimeUsedUSD:  row.LifetimeUsedUSD,
			WindowStartedAt:  row.WindowStartedAt,
			ExpiresAt:        row.ExpiresAt,
			LifetimeRequests: row.LifetimeRequests,
			FirstUsedAt:      row.FirstUsedAt,
			LastUsedAt:       row.LastUsedAt,
		}
		if user.QuotaUSD > 0 {
			user.UsedPercent = user.UsedUSD / user.QuotaUSD * 100
		}
		user.Cycles = saturationCycles(now, row.FirstUsedAt)
		if user.QuotaUSD > 0 {
			user.LifetimePercent = row.LifetimeUsedUSD / (user.QuotaUSD * float64(user.Cycles)) * 100
		}

		var userWindowCost, userWindowTokens float64
		var userBaselineCost, userBaselineTokens float64
		var topCombo *usagestats.SaturationUserCombo
		for i := range combosByUser[row.UserID] {
			combo := &combosByUser[row.UserID][i]
			userWindowCost += combo.CostUSD
			userWindowTokens += float64(combo.Tokens)
			user.WindowRequests += combo.Requests
			if _, ok := baseline[saturationComboKey(combo.Model, combo.Effort)]; ok {
				userBaselineCost += combo.CostUSD
				userBaselineTokens += float64(combo.Tokens)
			}
			if topCombo == nil || combo.CostUSD > topCombo.CostUSD ||
				(combo.CostUSD == topCombo.CostUSD && combo.Tokens > topCombo.Tokens) {
				topCombo = combo
			}
		}
		user.WindowCostUSD = userWindowCost
		user.BaselineCostUSD = userBaselineCost
		user.WindowTokens = int64(userWindowTokens)
		if topCombo != nil {
			user.TopModel = topCombo.Model
			user.TopEffort = topCombo.Effort
			user.TopComboTokens = topCombo.Tokens
		}
		if userWindowCost > 0 {
			user.BaselineCostShare = userBaselineCost / userWindowCost * 100
			user.Compliant = user.BaselineCostShare >= config.ThresholdPercent
			usersWithUsage++
		}
		if userWindowTokens > 0 {
			user.BaselineTokenShare = userBaselineTokens / userWindowTokens * 100
		}

		summary.QuotaUSD += row.QuotaUSD
		summary.UsedUSD += row.UsedUSD
		summary.LifetimeUsedUSD += row.LifetimeUsedUSD
		if user.UsedPercent < 20 {
			summary.DormantUsers++
		}
		if user.UsedPercent >= 80 {
			summary.HeavyUsers++
		}
		if row.UsedUSD == 0 && row.LifetimeUsedUSD > 0 {
			summary.ResetUsers++
		}
		if row.LifetimeUsedUSD <= 0 {
			summary.NeverUsedUsers++
		}
		if user.Compliant {
			summary.CompliantUsers++
		}
		if row.WindowStartedAt != nil && (summary.EarliestWindowStart == nil || row.WindowStartedAt.Before(*summary.EarliestWindowStart)) {
			start := *row.WindowStartedAt
			summary.EarliestWindowStart = &start
		}
		if row.ExpiresAt != nil && (summary.LatestWindowEnd == nil || row.ExpiresAt.After(*summary.LatestWindowEnd)) {
			end := *row.ExpiresAt
			summary.LatestWindowEnd = &end
		}
		sumCurrentPct += user.UsedPercent
		sumLifetimePct += user.LifetimePercent
		currentPcts = append(currentPcts, user.UsedPercent)
		lifetimePcts = append(lifetimePcts, user.LifetimePercent)
		currentUsed = append(currentUsed, row.UsedUSD)
		lifetimeUsed = append(lifetimeUsed, row.LifetimeUsedUSD)
		users = append(users, user)
	}

	if len(rows) > 0 {
		summary.AverageSaturation = sumCurrentPct / float64(len(rows))
		summary.AverageLifetimeSaturation = sumLifetimePct / float64(len(rows))
	}
	if usersWithUsage > 0 {
		summary.CompliantRate = float64(summary.CompliantUsers) / float64(usersWithUsage) * 100
	}
	summary.BaselineCostUSD = baselineCost
	if totalWindowCost > 0 {
		summary.BaselineCostShare = baselineCost / totalWindowCost * 100
	}
	if totalWindowTokens > 0 {
		summary.BaselineTokenShare = baselineTokens / totalWindowTokens * 100
	}

	comboStats := make([]usagestats.SaturationComboStat, 0, len(comboAggs))
	for key, agg := range comboAggs {
		stat := usagestats.SaturationComboStat{
			Model:    agg.model,
			Effort:   agg.effort,
			Requests: agg.requests,
			Tokens:   agg.tokens,
			CostUSD:  agg.costUSD,
			Users:    int64(len(agg.users)),
		}
		if agg.tokens > 0 {
			stat.USDPerMillion = agg.costUSD / float64(agg.tokens) * 1_000_000
		}
		if totalWindowCost > 0 {
			stat.CostShare = agg.costUSD / totalWindowCost * 100
		}
		if totalWindowTokens > 0 {
			stat.TokenShare = float64(agg.tokens) / totalWindowTokens * 100
		}
		if _, ok := baseline[key]; ok {
			stat.Baseline = true
		}
		comboStats = append(comboStats, stat)
	}
	sort.Slice(comboStats, func(i, j int) bool {
		if comboStats[i].CostUSD != comboStats[j].CostUSD {
			return comboStats[i].CostUSD > comboStats[j].CostUSD
		}
		return comboStats[i].Tokens > comboStats[j].Tokens
	})

	snapshot := &usagestats.SaturationSnapshot{
		GeneratedAt:   now,
		Config:        config,
		Summary:       summary,
		CurrentBands:  saturationBands(currentPcts, currentUsed),
		LifetimeBands: saturationBands(lifetimePcts, lifetimeUsed),
		Combos:        comboStats,
	}
	return &saturationReport{snapshot: snapshot, users: users}
}

// saturationCycles counts how many 30-day quota cycles a user has been active
// for, starting at their first subscription-billed request. It is the
// denominator used for the cumulative saturation percentage.
func saturationCycles(now time.Time, firstUsedAt *time.Time) int64 {
	if firstUsedAt == nil {
		return 1
	}
	days := now.Sub(*firstUsedAt).Hours() / 24
	cycles := int64(math.Ceil(days / saturationCycleDays))
	if cycles < 1 {
		return 1
	}
	return cycles
}

func saturationBands(percents, used []float64) []usagestats.SaturationBandStat {
	keys := []string{
		usagestats.SaturationBandDormant,
		usagestats.SaturationBandLight,
		usagestats.SaturationBandActive,
		usagestats.SaturationBandSaturated,
	}
	stats := make([]usagestats.SaturationBandStat, 0, len(keys))
	index := make(map[string]int, len(keys))
	for _, key := range keys {
		min, max := saturationBandLabel(key)
		index[key] = len(stats)
		stats = append(stats, usagestats.SaturationBandStat{Key: key, MinPct: min, MaxPct: max})
	}
	for i, percent := range percents {
		key := saturationBand(percent)
		pos, ok := index[key]
		if !ok {
			continue
		}
		stats[pos].Users++
		if i < len(used) {
			stats[pos].UsedUSD += used[i]
		}
	}
	return stats
}

func filterSaturationUsers(users []usagestats.SaturationUser, query SaturationUserQuery) []usagestats.SaturationUser {
	metricLifetime := strings.EqualFold(strings.TrimSpace(query.Metric), usagestats.SaturationMetricLifetime)
	search := strings.ToLower(strings.TrimSpace(query.Search))
	out := make([]usagestats.SaturationUser, 0, len(users))
	for _, user := range users {
		percent := user.UsedPercent
		if metricLifetime {
			percent = user.LifetimePercent
		}
		if query.Band != "" && saturationBand(percent) != query.Band {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(query.Compliance)) {
		case usagestats.SaturationComplianceCompliant:
			if !user.Compliant {
				continue
			}
		case usagestats.SaturationComplianceNonCompliant:
			if user.Compliant {
				continue
			}
		}
		if search != "" {
			haystack := strings.ToLower(user.Email + " " + user.Username + " " + user.DeptName + " " + user.GroupNames)
			if !strings.Contains(haystack, search) {
				continue
			}
		}
		out = append(out, user)
	}
	return out
}

func sortSaturationUsers(users []usagestats.SaturationUser, query SaturationUserQuery) {
	field := strings.TrimSpace(query.Sort)
	if field == "" {
		field = "used_percent"
	}
	ascending := strings.EqualFold(strings.TrimSpace(query.Order), "asc")
	less := func(i, j int) bool { return false }
	switch field {
	case "used_usd":
		less = func(i, j int) bool { return users[i].UsedUSD < users[j].UsedUSD }
	case "lifetime_used_usd":
		less = func(i, j int) bool { return users[i].LifetimeUsedUSD < users[j].LifetimeUsedUSD }
	case "lifetime_percent":
		less = func(i, j int) bool { return users[i].LifetimePercent < users[j].LifetimePercent }
	case "lifetime_requests":
		less = func(i, j int) bool { return users[i].LifetimeRequests < users[j].LifetimeRequests }
	case "baseline_cost_share":
		less = func(i, j int) bool { return users[i].BaselineCostShare < users[j].BaselineCostShare }
	case "last_used_at":
		less = func(i, j int) bool {
			return timePtrBefore(users[i].LastUsedAt, users[j].LastUsedAt)
		}
	case "email":
		less = func(i, j int) bool { return users[i].Email < users[j].Email }
	default:
		less = func(i, j int) bool { return users[i].UsedPercent < users[j].UsedPercent }
	}
	sort.SliceStable(users, func(i, j int) bool {
		if ascending {
			return less(i, j)
		}
		return less(j, i)
	})
}

func timePtrBefore(a, b *time.Time) bool {
	if a == nil {
		return b != nil
	}
	if b == nil {
		return false
	}
	return a.Before(*b)
}
