package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/npezzotti/go-chatroom/internal/types"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultJwtExpiration = time.Hour * 24
	tokenCookieKey       = "token"
)

func UserId(ctx context.Context) (int, bool) {
	userId, ok := ctx.Value(userIdKey).(int)

	return userId, ok
}

const (
	userIdClaim = "user-id"
	expClaim    = "exp"
)

type contextKey string

const userIdKey contextKey = "user-id"

func (s *GoChatApp) extractUserIdFromToken(tokenString string) (int, error) {
	token, err := s.verifyToken(tokenString)
	if err != nil {
		return 0, fmt.Errorf("verify token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fmt.Errorf("invalid token claims")
	}

	userId, ok := claims[userIdClaim].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid user id claim")
	}

	return int(userId), nil
}

func createJwtCookie(tokenString string, exp time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     tokenCookieKey,
		Value:    tokenString,
		Path:     "/",
		Expires:  time.Now().Add(exp),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

func hashPassword(passwd string) (string, error) {
	passwdHash, err := bcrypt.GenerateFromPassword([]byte(passwd), bcrypt.DefaultCost)
	return string(passwdHash), err
}

func verifyPassword(passwdHash, passwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(passwdHash), []byte(passwd))
	return err == nil
}

func (s *GoChatApp) createJwtForSession(user types.User, exp time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		userIdClaim: user.Id,
		expClaim:    time.Now().Add(exp).Unix(),
	})

	return token.SignedString(s.signingKey)
}

func (s *GoChatApp) verifyToken(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return s.signingKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return token, nil
}
