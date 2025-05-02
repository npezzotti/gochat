package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

var (
	defaultExp     = time.Hour * 24
	tokenCookieKey = "token"
)

type jwtClaim string

const (
	userIdClaim = "user-id"
	expClaim    = "exp"
)

type contextKey string

const userIdKey contextKey = "user-id"

type User struct {
	Id           int    `json:"id"`
	Username     string `json:"username"`
	EmailAddress string `json:"email_address,omitempty"`
	Password     string `json:"password,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func authMiddleware(l *log.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := r.Cookie(tokenCookieKey)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		token, err := verifyToken(tokenString.Value)
		if err != nil {
			l.Println("token verify failed:", err)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		userId, ok := claims[userIdClaim].(float64)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		ctx := context.WithValue(r.Context(), userIdKey, int(userId))
		w.Header().Add("Cache-Control", "no-store, no-cache, must-revalidate, private")

		next(w, r.WithContext(ctx))
	}
}

func createAccount(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if err := render(w, "signup.html.tmpl"); err != nil {
			errResp := NewInternalServerError(err)
			writeJson(l, w, errResp.Code, errResp)
			return
		}
	} else if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			errResp := NewInternalServerError(err)
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		pwdHash, err := hashPassword(r.Form.Get("password"))
		if err != nil {
			errResp := NewInternalServerError(err)
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		params := CreateAccountParams{
			Username:     r.Form.Get("username"),
			EmailAddress: r.Form.Get("email"),
			PasswordHash: pwdHash,
		}

		_, err = CreateAccount(params)
		if err != nil {
			errResp := NewInternalServerError(err)
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		http.Redirect(w, r, "/login", http.StatusFound)
	} else {
		errResp := NewMethodNotAllowedError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}
}

func account(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		userId, ok := UserId(r.Context())
		if !ok {
			errResp := NewUnauthorizedError()
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		user, err := GetAccount(userId)
		if err != nil {
			errResp := NewNotFoundError()
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		u := User{
			Id:           user.Id,
			Username:     user.Username,
			EmailAddress: user.EmailAddress,
		}

		writeJson(l, w, http.StatusOK, u)
	} else if r.Method == http.MethodPut {
		userId, ok := UserId(r.Context())
		if !ok {
			errResp := NewUnauthorizedError()
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		curUser, err := GetAccount(userId)
		if err != nil {
			errResp := NewNotFoundError()
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		var u User
		err = json.NewDecoder(r.Body).Decode(&u)
		if err != nil {
			errResp := NewBadRequestError()
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		pwdHash, err := hashPassword(u.Password)
		if err != nil {
			errResp := NewInternalServerError(err)
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		params := UpdateAccountParams{
			UserId:       curUser.Id,
			Username:     u.Username,
			PasswordHash: pwdHash,
		}

		dbUser, err := UpdateAccount(params)
		if err != nil {
			errResp := NewInternalServerError(err)
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		userResp := User{
			Id:           dbUser.Id,
			Username:     dbUser.Username,
			EmailAddress: dbUser.EmailAddress,
		}

		writeJson(l, w, http.StatusOK, userResp)
	} else {
		errResp := NewMethodNotAllowedError()
		writeJson(l, w, errResp.Code, errResp)
	}
}

func session(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	userId, ok := UserId(r.Context())
	l.Println("userId:", userId)
	if !ok {
		errResp := NewUnauthorizedError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	user, err := GetAccount(userId)
	if err != nil {
		errResp := NewNotFoundError()
		writeJson(l, w, errResp.Code, errResp)
		return
	}

	u := User{
		Id:           user.Id,
		Username:     user.Username,
		EmailAddress: user.EmailAddress,
	}

	writeJson(l, w, http.StatusOK, u)
}

func login(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if err := render(w, "login.html.tmpl"); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodPost {
		var lr LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&lr); err != nil {
			errResp := NewBadRequestError()
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		dbUser, err := GetAccountByEmail(lr.Email)
		if err != nil {
			var errResp *ApiError
			if err == sql.ErrNoRows {
				errResp = NewNotFoundError()
			} else {
				errResp = NewInternalServerError(err)
			}
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		if !verifyPassword(dbUser.PasswordHash, lr.Password) {
			errResp := NewUnauthorizedError()
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		u := User{
			Id:           dbUser.Id,
			Username:     dbUser.Username,
			EmailAddress: dbUser.EmailAddress,
		}

		token, err := createJwtForSession(u, defaultExp)
		if err != nil {
			errResp := NewInternalServerError(err)
			writeJson(l, w, errResp.Code, errResp)
			return
		}

		http.SetCookie(w, createJwtCookie(token, defaultExp))

		writeJson(l, w, http.StatusOK, u)
	} else {
		errResp := NewMethodNotAllowedError()
		writeJson(l, w, errResp.Code, errResp)
	}
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

func logout(w http.ResponseWriter, _ *http.Request) {
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

	return token.SignedString(secretKey)
}

func verifyToken(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return token, nil
}

func decodeSigningSecret() ([]byte, error) {
	return base64.StdEncoding.DecodeString("wT0phFUusHZIrDhL9bUKPUhwaxKhpi/SaI6PtgB+MgU=")
}
