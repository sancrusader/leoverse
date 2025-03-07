package leonardo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type sessionData struct {
	User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Sub   string `json:"sub"`
	} `json:"user"`
	Expires             string `json:"expires"`
	AccessToken         string `json:"accessToken"`
	AccessTokenIssuedAt int    `json:"accessTokenIssuedAt"`
	AccessTokenExpiry   int    `json:"accessTokenExpiry"`
	ServerTimestamp     int    `json:"serverTimestamp"`
}

type memCookieStore struct {
	cookie string
}

func NewMemCookieStore(cookie string) CookieStore {
	// If cookie is a JSON string, extract the access token
	if strings.HasPrefix(cookie, "{") {
		var session sessionData
		if err := json.Unmarshal([]byte(cookie), &session); err == nil {
			cookie = session.AccessToken
		}
	}

	// Format cookie if it's just a token
	if !strings.Contains(cookie, "=") {
		cookie = fmt.Sprintf("__Secure-next-auth.session-token=%s", cookie)
	}

	return &memCookieStore{cookie: cookie}
}

func (s *memCookieStore) GetCookie(ctx context.Context) (string, error) {
	if s.cookie == "" {
		return "", fmt.Errorf("cookie is not set")
	}
	return s.cookie, nil
}

func (s *memCookieStore) SetCookie(ctx context.Context, cookie string) error {
	s.cookie = cookie
	return nil
}
