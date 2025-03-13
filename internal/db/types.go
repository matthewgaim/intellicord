package db

import "time"

type JoinedServersInfo struct {
	ID              int    `json:"id"`
	DiscordServerID string `json:"discord_server_id"`
	JoinedAt        string `json:"joined_at"`
	Name            string `json:"name"`
	MemberCount     int    `json:"member_count"`
	OnlineCount     int    `json:"online_count"`
	Icon            string `json:"icon"`
	PremiumTier     int    `json:"premium_tier"`
	Banner          string `json:"banner"`
}

type FileInformation struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Size         int64  `json:"size"`
	AnalyzedDate string `json:"analyzed_date"`
}

type UserInfo struct {
	PriceID              string    `json:"price_id"`
	Plan                 string    `json:"plan"`
	PlanMonthlyStartDate time.Time `json:"plan_monthly_start_date"`
	PlanRenewalDate      time.Time `json:"plan_renewal_date"`
	JoinedAt             time.Time `json:"joined_at"`
}

type PlanLimits struct {
	MaxFileUploads int
	MaxMessages    int
}
