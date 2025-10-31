package api

type MiddlewareDiscordUser struct {
	ID string `json:"id"`
}

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

type DiscordTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type DiscordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	GlobalName    string `json:"global_name"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
}

type DiscordMeResponse struct {
	User DiscordUser `json:"user"`
}
