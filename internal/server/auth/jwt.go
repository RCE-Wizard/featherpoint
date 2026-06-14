package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Claims holds the JWT payload.
type Claims struct {
	Sub  string `json:"sub"`
	Role string `json:"role"`
	Exp  int64  `json:"exp"`
}

// IssueJWT creates a signed HS256 JWT for the given user.
func (s *Service) IssueJWT(userID, role string) (string, error) {
	header := base64url(mustJSON(map[string]string{"alg": "HS256", "typ": "JWT"}))
	payload := base64url(mustJSON(Claims{
		Sub:  userID,
		Role: role,
		Exp:  time.Now().Add(24 * time.Hour).Unix(),
	}))
	msg := header + "." + payload
	sig := hmacSHA256([]byte(msg), s.jwtSecret)
	return msg + "." + base64url(sig), nil
}

// VerifyJWT parses and verifies a JWT, returning the claims.
func (s *Service) VerifyJWT(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed token")
	}
	msg := parts[0] + "." + parts[1]
	sig := hmacSHA256([]byte(msg), s.jwtSecret)
	if base64url(sig) != parts[2] {
		return nil, fmt.Errorf("invalid signature")
	}
	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var c Claims
	if err := json.Unmarshal(rawPayload, &c); err != nil {
		return nil, err
	}
	if time.Now().Unix() > c.Exp {
		return nil, fmt.Errorf("token expired")
	}
	return &c, nil
}

func hmacSHA256(msg, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(msg)
	return mac.Sum(nil)
}

func base64url(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
