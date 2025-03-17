package api

type User struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

type UpdateAllowedChannelsRequest struct {
	ChannelIDs []string `json:"channel_ids"`
	ServerID   string   `json:"server_id"`
	UserID     string   `json:"user_id"`
}

type CheckoutSessionType struct {
	Status string `json:"status"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

type ClientSecret struct {
	ClientSecret string `json:"clientSecret"`
}
