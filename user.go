package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	Id           int
	Username     string
	EmailAddress string
	PasswordHash string
}

type UserResponse struct {
	Id           int    `json:"id"`
	Username     string `json:"username"`
	EmailAddress string `json:"email_address"`
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
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			l.Println("parse form:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		pwdHash, err := hashPassword(r.Form.Get("password"))
		if err != nil {
			l.Println("hash passwd:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		params := CreateAccountParams{
			Username:     r.Form.Get("username"),
			EmailAddress: r.Form.Get("email"),
			PasswordHash: pwdHash,
		}

		_, err = CreateAccount(params)
		if err != nil {
			l.Println("created account:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/login", http.StatusFound)
	} else {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func editAccount(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if err := render(w, "edit-account.html.tmpl"); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodPost {
		userId, ok := r.Context().Value(userIdKey).(int)
		if !ok {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		user, err := GetAccount(userId)
		if err != nil {
			l.Println("get account:", err)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		if err := r.ParseForm(); err != nil {
			l.Println("parse form:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		pwdHash, err := hashPassword(r.Form.Get("password"))
		if err != nil {
			l.Println("hash passwd:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		params := UpdateAccountParams{
			User:         user,
			Username:     r.Form.Get("username"),
			PasswordHash: pwdHash,
		}

		_, err = UpdateAccount(params)
		if err != nil {
			l.Println("update account:", err)
		}

		http.Redirect(w, r, "/account/edit", http.StatusFound)
	} else {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func login(l *log.Logger, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if err := render(w, "login.html.tmpl"); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			l.Println(err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var lr LoginRequest
		if err := json.Unmarshal(body, &lr); err != nil {
			l.Println(err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		user, err := GetAccountByEmail(lr.Email)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			} else {
				l.Println("get account by email:", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			return
		}

		if !verifyPassword(user.PasswordHash, lr.Password) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		token, err := createJwtForSession(user, defaultExp)
		if err != nil {
			l.Println("create jwt:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, createJwtCookie(token, defaultExp))

		userResp := UserResponse{
			Id:           user.Id,
			Username:     user.Username,
			EmailAddress: user.EmailAddress,
		}

		userRespJSON, err := json.Marshal(userResp)
		if err != nil {
			l.Println("marshal user:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Write(userRespJSON)
	} else {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
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
