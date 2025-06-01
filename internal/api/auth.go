package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/npezzotti/go-chatroom/internal/database"
	"golang.org/x/crypto/bcrypt"
)

var (
	SecretKey []byte

	defaultExp     = time.Hour * 24
	tokenCookieKey = "token"
)

func UserId(ctx context.Context) (int, bool) {
	userId, ok := ctx.Value(userIdKey).(int)

	return userId, ok
}

type jwtClaim string

const (
	userIdClaim = "user-id"
	expClaim    = "exp"
)

type contextKey string

const userIdKey contextKey = "user-id"

type User struct {
	Id           int       `json:"id"`
	Username     string    `json:"username"`
	EmailAddress string    `json:"email_address,omitempty"`
	Password     string    `json:"-"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func extractUserIdFromToken(r *http.Request) (int, error) {
	tokenString, err := r.Cookie(tokenCookieKey)
	if err != nil {
		return 0, fmt.Errorf("get cookie: %w", err)
	}

	token, err := verifyToken(tokenString.Value)
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

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId, err := extractUserIdFromToken(r)
		if err != nil {
			s.log.Println("failed to extract user id from token:", err)
			errResp := NewUnauthorizedError()
			writeJson(s.log, w, errResp.Code, errResp)
			return
		}

		ctx := context.WithValue(r.Context(), userIdKey, userId)
		w.Header().Add("Cache-Control", "no-store, no-cache, must-revalidate, private")

		next(w, r.WithContext(ctx))
	}
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errResp := NewBadRequestError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	pwdHash, err := hashPassword(req.Password)
	if err != nil {
		errResp := NewInternalServerError(err)
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	params := database.CreateAccountParams{
		Username:     r.Form.Get("username"),
		EmailAddress: r.Form.Get("email"),
		PasswordHash: pwdHash,
	}

	newUser, err := s.db.CreateAccount(params)
	if err != nil {
		errResp := NewInternalServerError(err)
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	writeJson(s.log, w, http.StatusCreated, User{
		Id:           newUser.Id,
		Username:     newUser.Username,
		EmailAddress: newUser.EmailAddress,
	})
}

func (s *Server) account(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		userId, ok := UserId(r.Context())
		if !ok {
			errResp := NewUnauthorizedError()
			writeJson(s.log, w, errResp.Code, errResp)
			return
		}

		user, err := s.db.GetAccount(userId)
		if err != nil {
			errResp := NewNotFoundError()
			writeJson(s.log, w, errResp.Code, errResp)
			return
		}

		u := User{
			Id:           user.Id,
			Username:     user.Username,
			EmailAddress: user.EmailAddress,
		}

		writeJson(s.log, w, http.StatusOK, u)
	} else if r.Method == http.MethodPut {
		userId, ok := UserId(r.Context())
		if !ok {
			errResp := NewUnauthorizedError()
			writeJson(s.log, w, errResp.Code, errResp)
			return
		}

		curUser, err := s.db.GetAccount(userId)
		if err != nil {
			errResp := NewNotFoundError()
			writeJson(s.log, w, errResp.Code, errResp)
			return
		}

		var u User
		err = json.NewDecoder(r.Body).Decode(&u)
		if err != nil {
			errResp := NewBadRequestError()
			writeJson(s.log, w, errResp.Code, errResp)
			return
		}

		pwdHash, err := hashPassword(u.Password)
		if err != nil {
			errResp := NewInternalServerError(err)
			writeJson(s.log, w, errResp.Code, errResp)
			return
		}

		params := database.UpdateAccountParams{
			UserId:       curUser.Id,
			Username:     u.Username,
			PasswordHash: pwdHash,
		}

		dbUser, err := s.db.UpdateAccount(params)
		if err != nil {
			errResp := NewInternalServerError(err)
			writeJson(s.log, w, errResp.Code, errResp)
			return
		}

		userResp := User{
			Id:           dbUser.Id,
			Username:     dbUser.Username,
			EmailAddress: dbUser.EmailAddress,
			CreatedAt:    dbUser.CreatedAt,
			UpdatedAt:    dbUser.UpdatedAt,
		}

		writeJson(s.log, w, http.StatusOK, userResp)
	} else {
		errResp := NewMethodNotAllowedError()
		writeJson(s.log, w, errResp.Code, errResp)
	}
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	userId, ok := UserId(r.Context())
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	user, err := s.db.GetAccount(userId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	u := User{
		Id:           user.Id,
		Username:     user.Username,
		EmailAddress: user.EmailAddress,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}

	writeJson(s.log, w, http.StatusOK, u)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var lr LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&lr); err != nil {
		errResp := NewBadRequestError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	dbUser, err := s.db.GetAccountByEmail(lr.Email)
	if err != nil {
		var errResp *ApiError
		if err == sql.ErrNoRows {
			errResp = NewNotFoundError()
		} else {
			errResp = NewInternalServerError(err)
		}
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	if !verifyPassword(dbUser.PasswordHash, lr.Password) {
		errResp := NewUnauthorizedError()
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	u := User{
		Id:           dbUser.Id,
		Username:     dbUser.Username,
		EmailAddress: dbUser.EmailAddress,
		CreatedAt:    dbUser.CreatedAt,
		UpdatedAt:    dbUser.UpdatedAt,
	}

	token, err := createJwtForSession(u, defaultExp)
	if err != nil {
		errResp := NewInternalServerError(err)
		writeJson(s.log, w, errResp.Code, errResp)
		return
	}

	http.SetCookie(w, createJwtCookie(token, defaultExp))

	writeJson(s.log, w, http.StatusOK, u)
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

func (s *Server) logout(w http.ResponseWriter, _ *http.Request) {
	// instruct browser to delete cookie by overwriting it with an expired token
	http.SetCookie(w, createJwtCookie("", time.Duration(time.Unix(0, 0).Unix())))
	w.WriteHeader(http.StatusNoContent)
}

func hashPassword(passwd string) (string, error) {
	passwdHash, err := bcrypt.GenerateFromPassword([]byte(passwd), bcrypt.DefaultCost)
	return string(passwdHash), err
}

func verifyPassword(passwdHash, passwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(passwdHash), []byte(passwd))
	return err == nil
}

func createJwtForSession(user User, exp time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		userIdClaim: user.Id,
		expClaim:    time.Now().Add(exp).Unix(),
	})

	return token.SignedString(SecretKey)
}

func verifyToken(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return SecretKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return token, nil
}
