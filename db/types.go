package db

type JoinedServersInfo struct {
	ID              int    `json:"id"`
	DiscordServerID string `json:"discord_server_id"`
	JoinedAt        string `json:"joined_at"`
	Name            string `json:"name"`
	MemberCount     int    `json:"member_count"`
	Icon            string `json:"icon"`
}

type FileInformation struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Size         int64  `json:"size"`
	AnalyzedDate string `json:"analyzed_date"`
}
