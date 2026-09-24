package usagestats

import "time"

// SaturationBandDormant/... are the saturation bands shared by the snapshot
// distribution and the ?band= list filter. Bands are keyed by saturation
// percent (used / quota * 100).
const (
	SaturationBandDormant   = "dormant"
	SaturationBandLight     = "light"
	SaturationBandActive    = "active"
	SaturationBandSaturated = "saturated"
)

// SaturationMetricCurrent and SaturationMetricLifetime select which saturation
// value the user list is filtered/sorted by.
const (
	SaturationMetricCurrent  = "current"
	SaturationMetricLifetime = "lifetime"
)

// SaturationComplianceCompliant / NonCompliant are the ?compliance= list filter
// values: compliant means the baseline-combo share of window cost reaches the
// configured threshold.
const (
	SaturationComplianceCompliant    = "compliant"
	SaturationComplianceNonCompliant = "non_compliant"
)

// SaturationBaselineCombo is one model+effort pair that the saturation report
// treats as a best-value baseline combination.
type SaturationBaselineCombo struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

// SaturationConfig is the admin-tunable configuration for the saturation
// report: which combinations count as best-value and how much of a user's
// usage must flow through them to count as compliant.
type SaturationConfig struct {
	BaselineCombos   []SaturationBaselineCombo `json:"baseline_combos"`
	ThresholdPercent float64                   `json:"threshold_percent"`
}

// SaturationUserAggregate is one quota-holder row of the base aggregate. Quota
// and current-window usage come from user_subscriptions; cumulative usage comes
// from usage_logs so window resets cannot erase history.
type SaturationUserAggregate struct {
	UserID            int64
	Email             string
	Username          string
	Status            string
	PrimaryDeptID     *int64
	DeptName          string
	GroupNames        string
	SubscriptionCount int64
	QuotaUSD          float64
	UsedUSD           float64
	DailyUsedUSD      float64
	WeeklyUsedUSD     float64
	WindowStartedAt   *time.Time
	ExpiresAt         *time.Time
	LifetimeUsedUSD   float64
	LifetimeRequests  int64
	FirstUsedAt       *time.Time
	LastUsedAt        *time.Time
}

// SaturationUserCombo is one user x model x effort bucket for the current
// quota window. Only subscription-billed usage with actual cost > 0 is
// included, matching the quota counters.
type SaturationUserCombo struct {
	UserID   int64
	Model    string
	Effort   string
	Requests int64
	Tokens   int64
	CostUSD  float64
}

// SaturationSummary carries the headline numbers for one snapshot.
type SaturationSummary struct {
	UserCount                 int64      `json:"user_count"`
	QuotaUSD                  float64    `json:"quota_usd"`
	UsedUSD                   float64    `json:"used_usd"`
	LifetimeUsedUSD           float64    `json:"lifetime_used_usd"`
	AverageSaturation         float64    `json:"average_saturation"`
	AverageLifetimeSaturation float64    `json:"average_lifetime_saturation"`
	DormantUsers              int64      `json:"dormant_users"`
	HeavyUsers                int64      `json:"heavy_users"`
	ResetUsers                int64      `json:"reset_users"`
	NeverUsedUsers            int64      `json:"never_used_users"`
	BaselineCostUSD           float64    `json:"baseline_cost_usd"`
	BaselineCostShare         float64    `json:"baseline_cost_share"`
	BaselineTokenShare        float64    `json:"baseline_token_share"`
	CompliantUsers            int64      `json:"compliant_users"`
	CompliantRate             float64    `json:"compliant_rate"`
	EarliestWindowStart       *time.Time `json:"earliest_window_start,omitempty"`
	LatestWindowEnd           *time.Time `json:"latest_window_end,omitempty"`
}

// SaturationBandStat is one row of the saturation distribution. MaxPct is nil
// for the open-ended top band (>=80%).
type SaturationBandStat struct {
	Key     string   `json:"key"`
	MinPct  float64  `json:"min_pct"`
	MaxPct  *float64 `json:"max_pct,omitempty"`
	Users   int64    `json:"users"`
	UsedUSD float64  `json:"used_usd"`
}

// SaturationComboStat aggregates one model+effort combination across the
// snapshot scope for the current window.
type SaturationComboStat struct {
	Model         string  `json:"model"`
	Effort        string  `json:"effort"`
	Requests      int64   `json:"requests"`
	Tokens        int64   `json:"tokens"`
	CostUSD       float64 `json:"cost_usd"`
	Users         int64   `json:"users"`
	USDPerMillion float64 `json:"usd_per_million_tokens"`
	Baseline      bool    `json:"baseline"`
	CostShare     float64 `json:"cost_share"`
	TokenShare    float64 `json:"token_share"`
}

// SaturationSnapshot is the full saturation report for one scope.
type SaturationSnapshot struct {
	GeneratedAt   time.Time             `json:"generated_at"`
	Config        SaturationConfig      `json:"config"`
	Summary       SaturationSummary     `json:"summary"`
	CurrentBands  []SaturationBandStat  `json:"current_bands"`
	LifetimeBands []SaturationBandStat  `json:"lifetime_bands"`
	Combos        []SaturationComboStat `json:"combos"`
}

// SaturationUser is one user row of the saturation list.
type SaturationUser struct {
	UserID             int64      `json:"user_id"`
	Email              string     `json:"email"`
	Username           string     `json:"username"`
	Status             string     `json:"status"`
	PrimaryDeptID      *int64     `json:"primary_dept_id,omitempty"`
	DeptName           string     `json:"dept_name,omitempty"`
	GroupNames         string     `json:"group_names,omitempty"`
	QuotaUSD           float64    `json:"quota_usd"`
	UsedUSD            float64    `json:"used_usd"`
	UsedPercent        float64    `json:"used_percent"`
	DailyUsedUSD       float64    `json:"daily_used_usd"`
	WeeklyUsedUSD      float64    `json:"weekly_used_usd"`
	LifetimeUsedUSD    float64    `json:"lifetime_used_usd"`
	LifetimePercent    float64    `json:"lifetime_percent"`
	Cycles             int64      `json:"cycles"`
	WindowStartedAt    *time.Time `json:"window_started_at,omitempty"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	LifetimeRequests   int64      `json:"lifetime_requests"`
	WindowRequests     int64      `json:"window_requests"`
	WindowTokens       int64      `json:"window_tokens"`
	WindowCostUSD      float64    `json:"window_cost_usd"`
	FirstUsedAt        *time.Time `json:"first_used_at,omitempty"`
	LastUsedAt         *time.Time `json:"last_used_at,omitempty"`
	BaselineCostUSD    float64    `json:"baseline_cost_usd"`
	BaselineCostShare  float64    `json:"baseline_cost_share"`
	BaselineTokenShare float64    `json:"baseline_token_share"`
	Compliant          bool       `json:"compliant"`
	TopModel           string     `json:"top_model,omitempty"`
	TopEffort          string     `json:"top_effort,omitempty"`
	TopComboTokens     int64      `json:"top_combo_tokens"`
}

// SaturationUserList is the paginated response of the saturation user list.
type SaturationUserList struct {
	Items    []SaturationUser `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// SaturationTrendPoint is one day of subscription-billed usage. Days without
// usage are returned with zeros so the trend chart has no gaps.
type SaturationTrendPoint struct {
	Date     string  `json:"date"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
	Users    int64   `json:"users"`
}
