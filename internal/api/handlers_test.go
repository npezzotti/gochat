package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/npezzotti/go-chatroom/internal/config"
	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/server"
	"github.com/npezzotti/go-chatroom/internal/stats"
	"github.com/npezzotti/go-chatroom/internal/testutil"
	"github.com/npezzotti/go-chatroom/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// findCookie is a helper function to find a cookie by name in the response recorder.
// It returns the cookie if found, or nil if not found.
func findCookie(rr *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func Test_healthCheck(t *testing.T) {
	mockRepo := &database.MockGoChatRepository{}
	defer mockRepo.AssertExpectations(t)

	tcases := []struct {
		name    string
		mockErr error
	}{
		{
			name:    "successful health check",
			mockErr: nil,
		},
		{
			name:    "failed health check",
			mockErr: errors.New("db error"),
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo.On("Ping").Return(tc.mockErr).Once()
			app := NewGoChatApp(http.NewServeMux(), nil, nil, mockRepo, nil, &config.Config{})
			rr := httptest.NewRecorder()
			buf := &bytes.Buffer{}
			req := httptest.NewRequest(http.MethodGet, "/healthz", buf)
			app.healthCheck(rr, req)

			if tc.mockErr != nil {
				assert.Equal(t, http.StatusInternalServerError, rr.Code, "expected status code to be 500")
			} else {
				assert.Equal(t, http.StatusOK, rr.Code, "expected status code to be 200")
				assert.Equal(t, "OK", rr.Body.String(), "expected response body to be 'OK'")
			}
		})
	}
}

func TestCreateAccountHandler(t *testing.T) {
	expectedUser := database.User{
		Id:           1,
		Username:     "newuser",
		EmailAddress: "newuser@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	tcases := []struct {
		name        string
		body        any
		success     bool
		mockUser    database.User
		mockErr     error
		expectedErr *ApiError
	}{
		{
			name: "successfully creates a new account",
			body: RegisterRequest{
				Username: expectedUser.Username,
				Email:    expectedUser.EmailAddress,
				Password: "password",
			},
			success:     true,
			mockUser:    expectedUser,
			mockErr:     nil,
			expectedErr: nil,
		},
		{
			name:        "failed with invalid json body",
			body:        "invalid json",
			success:     false,
			mockUser:    database.User{},
			mockErr:     nil,
			expectedErr: NewBadRequestError(),
		},
		{
			name: "fails with missing username",
			body: RegisterRequest{
				Email:    expectedUser.EmailAddress,
				Password: "password",
			},
			success:     false,
			mockUser:    database.User{},
			mockErr:     nil,
			expectedErr: NewBadRequestError(),
		},
		{
			name: "fails with missing email",
			body: RegisterRequest{
				Username: expectedUser.Username,
				Password: "password",
			},
			success:     false,
			mockUser:    database.User{},
			mockErr:     nil,
			expectedErr: NewBadRequestError(),
		},
		{
			name: "fails with missing password",
			body: RegisterRequest{
				Username: expectedUser.Username,
				Email:    expectedUser.EmailAddress,
			},
			success:     false,
			mockUser:    database.User{},
			mockErr:     nil,
			expectedErr: NewBadRequestError(),
		},
		{
			name: "fails with db error",
			body: RegisterRequest{
				Username: expectedUser.Username,
				Email:    expectedUser.EmailAddress,
				Password: "password",
			},
			success:     false,
			mockUser:    database.User{},
			mockErr:     errors.New("db error"),
			expectedErr: NewInternalServerError(nil),
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &database.MockGoChatRepository{}
			defer mockRepo.AssertExpectations(t)

			if tc.mockUser != (database.User{}) || tc.mockErr != nil {
				// Only set up the mock if a user is provided or an error is expected
				if regReq, ok := tc.body.(RegisterRequest); ok {
					params := database.CreateAccountParams{
						Username:     regReq.Username,
						EmailAddress: regReq.Email,
					}
					mockRepo.On("CreateAccount", mock.MatchedBy(func(req database.CreateAccountParams) bool {
						return req.Username == params.Username &&
							req.EmailAddress == params.EmailAddress &&
							verifyPassword(req.PasswordHash, regReq.Password)
					})).Return(tc.mockUser, tc.mockErr).Once()
				} else {
					t.Fatalf("unsupported request body type: %T", tc.body)
				}
			}

			app := NewGoChatApp(http.NewServeMux(), testutil.TestLogger(t), nil, mockRepo, nil, &config.Config{})

			var req *http.Request
			switch v := tc.body.(type) {
			case string:
				req = httptest.NewRequest(http.MethodPost, "/api/account", strings.NewReader(v))
			case RegisterRequest:
				body, err := json.Marshal(v)
				assert.NoError(t, err, "failed to marshal request body")
				req = httptest.NewRequest(http.MethodPost, "/api/account", bytes.NewBuffer(body))
			default:
				t.Fatalf("unsupported request body type: %T", v)
			}

			rr := httptest.NewRecorder()
			app.createAccount(rr, req)

			if tc.success {
				assert.Equal(t, http.StatusCreated, rr.Code)

				var user types.User
				err := json.NewDecoder(rr.Body).Decode(&user)
				assert.NoError(t, err, "failed to decode response")
				assert.Equal(t, expectedUser.Id, user.Id)
				assert.Equal(t, expectedUser.Username, user.Username)
				assert.Equal(t, expectedUser.EmailAddress, user.EmailAddress)
				assert.Equal(t, expectedUser.CreatedAt, user.CreatedAt)
				assert.Equal(t, expectedUser.UpdatedAt, user.UpdatedAt)
			} else {
				var apiErr ApiError
				err := json.NewDecoder(rr.Body).Decode(&apiErr)
				assert.NoError(t, err, "failed to decode error response")
				assert.Equal(t, tc.expectedErr.StatusCode, rr.Code, "expected status code to match")
				assert.Equal(t, *tc.expectedErr, apiErr, "expected ApiError response")
			}
		})
	}
}

func TestAccountHandler_Get(t *testing.T) {
	user := database.User{
		Id:           1,
		Username:     "test",
		EmailAddress: "",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	tcases := []struct {
		name        string
		userId      int
		mockUser    database.User
		mockErr     error
		expectedErr *ApiError
	}{
		{
			name:        "successfully retrieves account information",
			userId:      1,
			mockUser:    user,
			mockErr:     nil,
			expectedErr: nil,
		},
		{
			name:        "fails with unauthorized access",
			userId:      0,
			mockUser:    database.User{},
			mockErr:     nil,
			expectedErr: NewUnauthorizedError(),
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &database.MockGoChatRepository{}
			defer mockRepo.AssertExpectations(t)

			if tc.mockUser != (database.User{}) || tc.mockErr != nil {
				mockRepo.On("GetAccountById", 1).Return(tc.mockUser, tc.mockErr).Once()
			}

			app := NewGoChatApp(http.NewServeMux(), nil, nil, mockRepo, nil, &config.Config{})
			req := httptest.NewRequest(http.MethodGet, "/api/account", nil)

			if tc.userId > 0 {
				// Set user ID in context to simulate an authenticated user
				ctx := WithUserId(req.Context(), tc.userId)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			app.account(rr, req)

			if tc.expectedErr != nil {
				var apiErr ApiError
				err := json.NewDecoder(rr.Body).Decode(&apiErr)
				assert.NoError(t, err, "failed to decode error response")
				assert.Equal(t, rr.Code, tc.expectedErr.StatusCode, "expected status code to match")
				assert.Equal(t, *tc.expectedErr, apiErr, "expected ApiError response")
			} else {
				assert.Equal(t, http.StatusOK, rr.Code)

				var user types.User
				err := json.NewDecoder(rr.Body).Decode(&user)
				assert.NoError(t, err, "failed to decode response")
				assert.Equal(t, user.Id, tc.mockUser.Id)
				assert.Equal(t, user.Username, tc.mockUser.Username)
				assert.Equal(t, user.EmailAddress, tc.mockUser.EmailAddress)
				assert.Equal(t, user.CreatedAt, tc.mockUser.CreatedAt)
				assert.Equal(t, user.UpdatedAt, tc.mockUser.UpdatedAt)
			}
		})
	}
}

func TestAccountHandler_Put(t *testing.T) {
	mockCurUser := database.User{
		Id:           1,
		Username:     "test",
		EmailAddress: "test@example.com",
		PasswordHash: "testhash",
		CreatedAt:    time.Now().UTC().Add(-5 * time.Minute),
		UpdatedAt:    time.Now().UTC().Add(-5 * time.Minute),
	}

	tcases := []struct {
		name                  string
		userId                int
		body                  any
		mockCurUser           database.User
		mockGetAccountByIdErr error
		mockExpectedUser      database.User
		mockUpdateAccountErr  error
		expectedErr           *ApiError
	}{
		{
			name:   "successfully updates account information",
			userId: 1,
			body: UpdateAccountRequest{
				Username: "testupdated",
				Password: "passwordupdated",
			},
			mockCurUser:           mockCurUser,
			mockGetAccountByIdErr: nil,
			mockExpectedUser: database.User{
				Id:           1,
				Username:     "testupdated",
				EmailAddress: "test@example.com",
				PasswordHash: "hashedpasswordupdated",
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			},
			mockUpdateAccountErr: nil,
			expectedErr:          nil,
		},
		{
			name:   "fails with unauthorized access",
			userId: 0,
			body: UpdateAccountRequest{
				Username: "testupdated",
				Password: "passwordupdated",
			},
			mockCurUser:           database.User{},
			mockGetAccountByIdErr: nil,
			mockExpectedUser:      database.User{},
			mockUpdateAccountErr:  nil,
			expectedErr:           NewUnauthorizedError(),
		},
		{
			name:   "fails with user not found",
			userId: 1,
			body: UpdateAccountRequest{
				Username: "testupdated",
				Password: "passwordupdated",
			},
			mockCurUser:           database.User{},
			mockGetAccountByIdErr: sql.ErrNoRows,
			mockExpectedUser:      database.User{},
			mockUpdateAccountErr:  nil,
			expectedErr:           NewNotFoundError(),
		},
		{
			name:   "fails with db error on get account",
			userId: 1,
			body: UpdateAccountRequest{
				Username: "testupdated",
				Password: "passwordupdated",
			},
			mockCurUser:           mockCurUser,
			mockGetAccountByIdErr: nil,
			mockExpectedUser:      database.User{},
			mockUpdateAccountErr:  errors.New("db error"),
			expectedErr:           NewInternalServerError(nil),
		},
		{
			name:                  "fails with invalid json body",
			userId:                1,
			body:                  "invalid json",
			mockCurUser:           mockCurUser,
			mockGetAccountByIdErr: nil,
			mockExpectedUser:      database.User{},
			mockUpdateAccountErr:  nil,
			expectedErr:           NewBadRequestError(),
		},
		{
			name:   "fails with missing username",
			userId: 1,
			body: UpdateAccountRequest{
				Password: "passwordupdated",
			},
			mockCurUser:           mockCurUser,
			mockGetAccountByIdErr: nil,
			mockExpectedUser:      database.User{},
			mockUpdateAccountErr:  nil,
			expectedErr:           NewBadRequestError(),
		},
		{
			name:   "fails with missing password",
			userId: 1,
			body: UpdateAccountRequest{
				Username: "testupdated",
			},
			mockCurUser:           mockCurUser,
			mockGetAccountByIdErr: nil,
			mockExpectedUser:      database.User{},
			mockUpdateAccountErr:  nil,
			expectedErr:           NewBadRequestError(),
		},
		{
			name:   "fails with db error on update account",
			userId: 1,
			body: UpdateAccountRequest{
				Username: "testupdated",
				Password: "passwordupdated",
			},
			mockCurUser:           mockCurUser,
			mockGetAccountByIdErr: nil,
			mockExpectedUser:      database.User{},
			mockUpdateAccountErr:  errors.New("db error"),
			expectedErr:           NewInternalServerError(nil),
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &database.MockGoChatRepository{}
			defer mockRepo.AssertExpectations(t)

			if tc.userId > 0 && (tc.mockCurUser != (database.User{}) || tc.mockGetAccountByIdErr != nil) {
				mockRepo.On("GetAccountById", tc.userId).Return(tc.mockCurUser, tc.mockGetAccountByIdErr).Once()
			}

			if tc.mockExpectedUser != (database.User{}) || tc.mockUpdateAccountErr != nil {
				updateReq, ok := tc.body.(UpdateAccountRequest)
				assert.Truef(t, ok, "expected body to be of type UpdateAccountRequest, got %T", tc.body)
				mockRepo.On("UpdateAccount", mock.MatchedBy(func(params database.UpdateAccountParams) bool {
					return params.UserId == tc.userId &&
						params.Username == updateReq.Username &&
						verifyPassword(params.PasswordHash, updateReq.Password)
				})).Return(tc.mockExpectedUser, tc.mockUpdateAccountErr).Once()
			}

			app := NewGoChatApp(http.NewServeMux(), nil, nil, mockRepo, nil, &config.Config{})
			rr := httptest.NewRecorder()

			var req *http.Request
			switch v := tc.body.(type) {
			case string:
				req = httptest.NewRequest(http.MethodPut, "/api/account", strings.NewReader(v))
			case UpdateAccountRequest:
				body, err := json.Marshal(v)
				assert.NoErrorf(t, err, "failed to marshal request body: %v", err)
				req = httptest.NewRequest(http.MethodPut, "/api/account", bytes.NewBuffer(body))
			default:
				t.Fatalf("unsupported request body type: %T", v)
			}

			if tc.userId > 0 {
				// Set user ID in context to simulate an authenticated user
				ctx := WithUserId(req.Context(), tc.userId)
				req = req.WithContext(ctx)
			}

			app.account(rr, req)

			if tc.expectedErr != nil {
				var apiErr ApiError
				err := json.NewDecoder(rr.Body).Decode(&apiErr)
				assert.NoErrorf(t, err, "failed to decode error response: %v", err)
				assert.Equal(t, rr.Code, tc.expectedErr.StatusCode, "expected status code to match")
				assert.Equal(t, *tc.expectedErr, apiErr, "expected ApiError response")
			} else {
				assert.Equal(t, http.StatusOK, rr.Code)

				var user types.User
				err := json.NewDecoder(rr.Body).Decode(&user)
				assert.NoErrorf(t, err, "failed to decode response: %v", err)
				assert.Equal(t, user.Id, tc.mockExpectedUser.Id)
				assert.Equal(t, user.Username, tc.mockExpectedUser.Username)
				assert.Equal(t, user.EmailAddress, tc.mockExpectedUser.EmailAddress)
				assert.WithinDuration(t, user.UpdatedAt, tc.mockExpectedUser.UpdatedAt, time.Second, "expected updated at to match")
			}
		})
	}
}

func Test_session(t *testing.T) {
	mockUser := database.User{
		Id:           1,
		Username:     "testuser",
		EmailAddress: "testuser@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	tcases := []struct {
		name        string
		success     bool
		userId      int
		expectedErr *ApiError
		mockUser    database.User
		mockErr     error
	}{
		{
			name:        "successfully retrieves session",
			success:     true,
			userId:      1,
			expectedErr: nil,
			mockUser:    mockUser,
			mockErr:     nil,
		},
		{
			name:        "fails with unauthorized access",
			success:     false,
			userId:      0,
			expectedErr: NewUnauthorizedError(),
			mockUser:    database.User{},
			mockErr:     nil,
		},
		{
			name:        "fails with user not found",
			success:     false,
			userId:      1,
			expectedErr: NewNotFoundError(),
			mockUser:    database.User{},
			mockErr:     sql.ErrNoRows,
		},
		{
			name:        "fails with db error",
			success:     false,
			userId:      1,
			expectedErr: NewInternalServerError(nil),
			mockUser:    database.User{},
			mockErr:     errors.New("db error"),
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &database.MockGoChatRepository{}
			defer mockRepo.AssertExpectations(t)

			// Only set up the mock if a user ID is provided
			// and there is either a valid mock user or an error expected
			if tc.userId > 0 && (tc.mockUser != (database.User{}) || tc.mockErr != nil) {
				mockRepo.On("GetAccountById", tc.userId).Return(tc.mockUser, tc.mockErr).Once()
			}

			app := NewGoChatApp(http.NewServeMux(), nil, nil, mockRepo, nil, &config.Config{})

			req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
			if tc.userId > 0 {
				ctx := WithUserId(req.Context(), tc.userId)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			app.session(rr, req)

			if tc.success {
				var user types.User
				err := json.NewDecoder(rr.Body).Decode(&user)
				assert.NoErrorf(t, err, "failed to decode response: %v", err)
				assert.Equal(t, http.StatusOK, rr.Code)
				assert.Equal(t, tc.mockUser.Id, user.Id, "expected user ID to match")
				assert.Equal(t, tc.mockUser.Username, user.Username, "expected username to match")
				assert.Equal(t, tc.mockUser.EmailAddress, user.EmailAddress, "expected email address to match")
				assert.WithinDuration(t, tc.mockUser.CreatedAt, user.CreatedAt, time.Second, "expected created at to match")
				assert.WithinDuration(t, tc.mockUser.UpdatedAt, user.UpdatedAt, time.Second, "expected updated at to match")
			} else {
				var apiErr ApiError
				err := json.NewDecoder(rr.Body).Decode(&apiErr)
				assert.NoErrorf(t, err, "failed to decode error response: %v", err)
				assert.Equal(t, apiErr.StatusCode, rr.Code, "expected status code to match")
				assert.Equal(t, *tc.expectedErr, apiErr, "expected ApiError response")
			}
		})
	}
}

func Test_login(t *testing.T) {
	mockUser := database.User{
		Id:           1,
		Username:     "testuser",
		EmailAddress: "testuser@example.com",
		PasswordHash: "$2a$10$dP8ByMfAiDG54vZg/SwEkuJN0ttMSaUFbA3KzcxeriGN31lIXuCu2", // hash for "password123"
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	testCases := []struct {
		name        string
		body        any
		mockUser    database.User
		mockErr     error
		success     bool
		expectError *ApiError
	}{
		{
			name: "successful login",
			body: LoginRequest{
				Email:    "testuser@example.com",
				Password: "password123",
			},
			mockUser:    mockUser,
			mockErr:     nil,
			success:     true,
			expectError: nil,
		},
		{
			name:        "fails with invalid json body",
			body:        "invalid json",
			mockUser:    database.User{},
			mockErr:     nil,
			success:     false,
			expectError: NewBadRequestError(),
		},
		{
			name: "fails with missing email",
			body: LoginRequest{
				Password: "password123",
			},
			mockUser:    database.User{},
			mockErr:     nil,
			success:     false,
			expectError: NewBadRequestError(),
		},
		{
			name: "fails with missing password",
			body: LoginRequest{
				Email: "testuser@example.com",
			},
			mockUser:    database.User{},
			mockErr:     nil,
			success:     false,
			expectError: NewBadRequestError(),
		},
		{
			name: "fails with db error",
			body: LoginRequest{
				Email:    "testuser@example.com",
				Password: "password123",
			},
			mockUser:    database.User{},
			mockErr:     errors.New("db error"),
			success:     false,
			expectError: NewInternalServerError(nil),
		},
		{
			name: "fails with incorrect password",
			body: LoginRequest{
				Email:    "testuser@example.com",
				Password: "wrong-password",
			},
			mockUser:    mockUser,
			mockErr:     nil,
			success:     false,
			expectError: NewUnauthorizedError(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &database.MockGoChatRepository{}
			defer mockRepo.AssertExpectations(t)

			// Only set up the mock if an email is provided in the body
			if tc.mockUser != (database.User{}) || tc.mockErr != nil {
				req, ok := tc.body.(LoginRequest)
				assert.Truef(t, ok, "expected body to be of type LoginRequest, got %T", tc.body)
				// Mock the GetAccountByEmail method to return the mock user or error
				mockRepo.On("GetAccountByEmail", req.Email).Return(tc.mockUser, tc.mockErr)
			}

			app := NewGoChatApp(http.NewServeMux(), nil, nil, mockRepo, nil, &config.Config{
				SigningKey: []byte("test-signing-key"),
			})

			var req *http.Request
			switch v := tc.body.(type) {
			case string:
				req = httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(v))
			default:
				body, err := json.Marshal(tc.body)
				assert.NoErrorf(t, err, "failed to marshal login request: %v", err)
				req = httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBuffer(body))
			}

			rr := httptest.NewRecorder()
			app.login(rr, req)

			if tc.success {
				token := findCookie(rr, tokenCookieKey)
				assert.NotNil(t, token, "expected token cookie to be set")
				assert.NotEmpty(t, token.Value, "expected token value to be set")
				assert.WithinDuration(t, token.Expires, time.Now().Add(defaultJwtExpiration), time.Second, "expected token expiration to be set correctly")
				var u types.User
				err := json.NewDecoder(rr.Body).Decode(&u)
				assert.NoErrorf(t, err, "failed to decode response: %v", err)

				expectedUserResp := types.User{
					Id:           tc.mockUser.Id,
					Username:     tc.mockUser.Username,
					EmailAddress: tc.mockUser.EmailAddress,
					CreatedAt:    tc.mockUser.CreatedAt,
					UpdatedAt:    tc.mockUser.UpdatedAt,
				}
				assert.Equal(t, http.StatusOK, rr.Code)
				assert.Equal(t, expectedUserResp, u, "expected user response to match")
			} else {
				var e ApiError
				err := json.NewDecoder(rr.Body).Decode(&e)
				assert.NoErrorf(t, err, "failed to decode response: %v", err)
				assert.Equal(t, e.StatusCode, rr.Code, "expected status code to match")
				assert.Equal(t, *tc.expectError, e, "expected ApiError response")
			}
		})
	}
}

func Test_logout(t *testing.T) {
	app := NewGoChatApp(http.NewServeMux(), log.Default(), nil, &database.MockGoChatRepository{}, nil, &config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	req.AddCookie(createJwtCookie("testtoken", defaultJwtExpiration))
	rr := httptest.NewRecorder()
	app.logout(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Check if the token cookie is set to expire
	token := findCookie(rr, tokenCookieKey)
	assert.NotNil(t, token, "expected token cookie to be set")
	assert.WithinDuration(t, token.Expires, time.Now(), time.Duration(time.Second), "expected tokent to be expired")
	assert.Equal(t, "", token.Value, "expected token value to be empty")
}

func Test_createRoom(t *testing.T) {
	mockRoom := database.Room{
		Id:          1,
		Name:        "Test Room",
		ExternalId:  "EoGKUXPHgz", // Example shortid, typically under 9 characters
		Description: "This is a test room",
		OwnerId:     1,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	tcases := []struct {
		name        string
		body        any
		userId      int
		mockRoom    database.Room
		mockErr     error
		shortIdErr  error // Added to handle short ID generation errors
		expectedErr *ApiError
	}{
		{
			name: "successfully creates a room",
			body: CreateRoomRequest{
				Name:        "Test Room",
				Description: "This is a test room",
			},
			userId:      1,
			mockRoom:    mockRoom,
			mockErr:     nil,
			expectedErr: nil,
		},
		{
			name:        "fails with invalid json body",
			body:        "invalid json",
			userId:      1,
			mockRoom:    database.Room{},
			mockErr:     nil,
			expectedErr: NewBadRequestError(),
		},
		{
			name:        "missing room name",
			body:        CreateRoomRequest{Description: "This is a test room"},
			userId:      1,
			mockRoom:    database.Room{},
			mockErr:     nil,
			expectedErr: NewBadRequestError(),
		},
		{
			name:        "missing room description",
			body:        CreateRoomRequest{Name: "Test Room"},
			userId:      1,
			mockRoom:    database.Room{},
			mockErr:     nil,
			expectedErr: NewBadRequestError(),
		},
		{
			name: "fails with no user id in context",
			body: CreateRoomRequest{
				Name:        "Test Room",
				Description: "This is a test room",
			},
			userId:      0,
			mockRoom:    database.Room{},
			mockErr:     nil,
			expectedErr: NewUnauthorizedError(),
		},
		{
			name: "fails to generate short id",
			body: CreateRoomRequest{
				Name:        "Test Room",
				Description: "This is a test room",
			},
			userId:      1,
			mockRoom:    database.Room{},
			mockErr:     nil,
			shortIdErr:  errors.New("failed to generate short id"),
			expectedErr: NewInternalServerError(nil),
		},
		{
			name: "fails with db error",
			body: CreateRoomRequest{
				Name:        "Test Room",
				Description: "This is a test room",
			},
			userId:      1,
			mockRoom:    mockRoom,
			mockErr:     errors.New("db error"),
			shortIdErr:  nil,
			expectedErr: NewInternalServerError(nil),
		},
	}
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &database.MockGoChatRepository{}
			defer mockRepo.AssertExpectations(t)

			// Check if mockRoom was provided in the test case by comparing to the Id field
			if tc.mockRoom.Id != 0 || tc.mockErr != nil {
				createRoomReq, ok := tc.body.(CreateRoomRequest)
				if !ok {
					t.Fatalf("expected body to be of type CreateRoomRequest, got %T", tc.body)
				}
				mockRepo.On("CreateRoom", mock.MatchedBy(func(params database.CreateRoomParams) bool {
					return params.Name == createRoomReq.Name &&
						params.Description == createRoomReq.Description &&
						params.OwnerId == tc.userId &&
						params.ExternalId == tc.mockRoom.ExternalId // shortid is typically up to 9 characters
				})).Return(tc.mockRoom, tc.mockErr).Once()
			}

			app := NewGoChatApp(http.NewServeMux(), log.Default(), nil, mockRepo, nil, &config.Config{})

			// Only override generateShortId if a shortIdErr is expected or a mockRoom is provided
			app.generateShortId = func() (string, error) {
				if tc.shortIdErr != nil {
					return "", tc.shortIdErr
				}
				return mockRoom.ExternalId, nil // Return the mock room's ExternalId
			}

			body, err := json.Marshal(tc.body)
			assert.NoErrorf(t, err, "failed to marshal request body: %v", err)
			req := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewBuffer(body))

			if tc.userId > 0 {
				ctx := WithUserId(req.Context(), tc.userId)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()

			app.createRoom(rr, req)

			if tc.expectedErr != nil {
				var apiErr ApiError
				err := json.NewDecoder(rr.Body).Decode(&apiErr)
				assert.NoErrorf(t, err, "failed to decode error response: %v", err)
				assert.Equal(t, rr.Code, tc.expectedErr.StatusCode, "expected status code to match")
				assert.Equal(t, *tc.expectedErr, apiErr, "expected ApiError response")
			} else {
				assert.Equal(t, http.StatusCreated, rr.Code)

				var room types.Room
				err := json.NewDecoder(rr.Body).Decode(&room)
				assert.NoErrorf(t, err, "failed to decode response: %v", err)
				assert.Equal(t, tc.mockRoom.Id, room.Id, "expected room id to match")
				assert.Equal(t, tc.mockRoom.Name, room.Name, "expected room name to match")
				assert.Equal(t, tc.mockRoom.ExternalId, room.ExternalId, "expected room external id to match")
				assert.Equal(t, tc.mockRoom.Description, room.Description, "expected room description to match")
				assert.Equal(t, tc.mockRoom.OwnerId, room.OwnerId, "expected room owner id to match requester ID")
				assert.WithinDuration(t, time.Now().UTC(), room.CreatedAt, time.Second, "expected room created at to be close to now")
				assert.WithinDuration(t, time.Now().UTC(), room.UpdatedAt, time.Second, "expected room updated at to be close to now")
			}
		})
	}
}
func Test_deleteRoom(t *testing.T) {
	mockRoom := database.Room{
		Id:          1,
		Name:        "Test Room",
		ExternalId:  "EoGKUXPHgz", // Example shortid, typically under 9 characters
		Description: "This is a test room",
		OwnerId:     1,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	tcases := []struct {
		name                       string
		userId                     int
		roomId                     string
		mockRoom                   database.Room
		mockGetRoomByExternalIdErr error
		mockDeleteRoomErr          error
		expectedErr                *ApiError
	}{
		{
			name:                       "successfully deletes a room",
			userId:                     1,
			roomId:                     mockRoom.ExternalId,
			mockRoom:                   mockRoom,
			mockGetRoomByExternalIdErr: nil,
			mockDeleteRoomErr:          nil,
			expectedErr:                nil,
		},
		{
			name:                       "fails with no query parameter",
			userId:                     1,
			roomId:                     "",
			mockRoom:                   database.Room{},
			mockGetRoomByExternalIdErr: nil,
			mockDeleteRoomErr:          nil,
			expectedErr:                NewBadRequestError(),
		},
		{
			name:                       "fails with room not found",
			userId:                     1,
			roomId:                     "not-found",
			mockRoom:                   database.Room{},
			mockGetRoomByExternalIdErr: sql.ErrNoRows,
			mockDeleteRoomErr:          nil,
			expectedErr:                NewNotFoundError(),
		},
		{
			name:                       "fails with db error on get room",
			userId:                     1,
			roomId:                     mockRoom.ExternalId,
			mockRoom:                   database.Room{},
			mockGetRoomByExternalIdErr: errors.New("db error"),
			mockDeleteRoomErr:          nil,
			expectedErr:                NewInternalServerError(nil),
		},
		{
			name:                       "fails with forbidden access",
			userId:                     2, // Different user ID than the room owner
			roomId:                     mockRoom.ExternalId,
			mockRoom:                   mockRoom,
			mockGetRoomByExternalIdErr: nil,
			mockDeleteRoomErr:          nil,
			expectedErr:                NewForbiddenError(),
		},
		{
			name:                       "fails with db error on delete room",
			userId:                     1, // Different user ID than the room owner
			roomId:                     mockRoom.ExternalId,
			mockRoom:                   mockRoom,
			mockGetRoomByExternalIdErr: nil,
			mockDeleteRoomErr:          errors.New("db error"),
			expectedErr:                NewInternalServerError(nil),
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &database.MockGoChatRepository{}
			defer mockRepo.AssertExpectations(t)

			if tc.roomId != "" || tc.mockGetRoomByExternalIdErr != nil {
				mockRepo.On("GetRoomByExternalId", tc.roomId).Return(tc.mockRoom, tc.mockGetRoomByExternalIdErr).Once()
			}

			if tc.mockRoom.Id != 0 || tc.mockDeleteRoomErr != nil {
				if tc.expectedErr == nil || *tc.expectedErr != *NewForbiddenError() { // "fails with forbidden access" case does not call DeleteRoom
					mockRepo.On("DeleteRoom", tc.mockRoom.Id).Return(tc.mockDeleteRoomErr).Once()
				}
			}

			su := &stats.MockStatsUpdater{}
			defer su.AssertExpectations(t)
			su.On("RegisterMetric", mock.Anything).Return(nil).Times(4)

			cs, err := server.NewChatServer(log.Default(), mockRepo, su)
			if err != nil {
				t.Fatalf("failed to create chat server: %v", err)
			}

			app := NewGoChatApp(http.NewServeMux(), log.Default(), cs, mockRepo, nil, &config.Config{})

			var queryString string
			if tc.roomId != "" {
				queryString = "?id=" + tc.roomId
			}
			req := httptest.NewRequest(http.MethodDelete, "/api/rooms/"+queryString, nil)

			if tc.userId > 0 {
				ctx := WithUserId(req.Context(), tc.userId)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			// Call deleteRoom and ensure it does not block on chat server interface.
			app.deleteRoom(rr, req)

			if tc.expectedErr != nil {
				var apiErr ApiError
				err := json.NewDecoder(rr.Body).Decode(&apiErr)
				assert.NoErrorf(t, err, "failed to decode error response: %v", err)
				assert.Equal(t, rr.Code, tc.expectedErr.StatusCode, "expected status code to match")
				assert.Equal(t, *tc.expectedErr, apiErr, "expected ApiError response")
			} else {
				assert.Equal(t, http.StatusNoContent, rr.Code)
			}
		})
	}
}

func Test_getUsersSubscriptions(t *testing.T) {
	mockSubs := []database.Subscription{
		{
			Id:            1,
			LastReadSeqId: 0,
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
			Room: database.Room{
				Id:          1,
				ExternalId:  "EoGKUXPHgz",
				Name:        "Test Room",
				Description: "This is a test room",
				SeqId:       0,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			},
		},
	}

	tcases := []struct {
		name        string
		userId      int
		mockSubs    []database.Subscription
		mockErr     error
		expected    []types.Subscription
		expectedErr *ApiError
	}{
		{
			name:     "successfully retrieves user subscriptions",
			userId:   1,
			mockSubs: mockSubs,
			mockErr:  nil,
			expected: func() []types.Subscription {
				subs := make([]types.Subscription, len(mockSubs))
				for i, sub := range mockSubs {
					subs[i] = types.Subscription{
						Id:            sub.Id,
						LastReadSeqId: sub.LastReadSeqId,
						CreatedAt:     sub.CreatedAt,
						UpdatedAt:     sub.UpdatedAt,
						Room: types.Room{
							Id:          sub.Room.Id,
							Name:        sub.Room.Name,
							ExternalId:  sub.Room.ExternalId,
							Description: sub.Room.Description,
							CreatedAt:   sub.Room.CreatedAt,
							UpdatedAt:   sub.Room.UpdatedAt,
						},
					}
				}
				return subs
			}(),
			expectedErr: nil,
		},
		{
			name:        "fails with unauthorized access",
			userId:      0,
			mockSubs:    nil,
			mockErr:     nil,
			expected:    nil,
			expectedErr: NewUnauthorizedError(),
		},
		{
			name:        "fails with db error",
			userId:      1,
			mockSubs:    nil,
			mockErr:     errors.New("db error"),
			expected:    nil,
			expectedErr: NewInternalServerError(nil),
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &database.MockGoChatRepository{}
			defer mockRepo.AssertExpectations(t)

			if tc.mockSubs != nil || tc.mockErr != nil {
				mockRepo.On("ListSubscriptions", tc.userId).Return(tc.mockSubs, tc.mockErr).Once()
			}

			app := NewGoChatApp(http.NewServeMux(), log.Default(), nil, mockRepo, nil, &config.Config{})

			req := httptest.NewRequest(http.MethodGet, "/api/subscriptions", nil)
			if tc.userId > 0 {
				ctx := WithUserId(req.Context(), tc.userId)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			app.getUsersSubscriptions(rr, req)

			if tc.expectedErr != nil {
				var apiErr ApiError
				err := json.NewDecoder(rr.Body).Decode(&apiErr)
				assert.NoError(t, err, "expected to decode ApiError successfully")
				assert.Equal(t, rr.Code, tc.expectedErr.StatusCode, "expected status code to match")
				assert.Equal(t, *tc.expectedErr, apiErr, "expected ApiError response")
				return
			}

			assert.Equal(t, http.StatusOK, rr.Code)

			var subs []types.Subscription
			err := json.NewDecoder(rr.Body).Decode(&subs)
			assert.NoError(t, err, "expected to decode subscriptions successfully")

			assert.Equal(t, len(tc.expected), len(subs), "expected number of subscriptions to match")
			for i := range subs {
				assert.Equal(t, tc.expected[i].Id, subs[i].Id)
				assert.Equal(t, tc.expected[i].LastReadSeqId, subs[i].LastReadSeqId)
				assert.WithinDuration(t, tc.expected[i].CreatedAt, subs[i].CreatedAt, time.Second)
				assert.WithinDuration(t, tc.expected[i].UpdatedAt, subs[i].UpdatedAt, time.Second)
				assert.Equal(t, tc.expected[i].Room.Id, subs[i].Room.Id)
				assert.Equal(t, tc.expected[i].Room.Name, subs[i].Room.Name)
				assert.Equal(t, tc.expected[i].Room.ExternalId, subs[i].Room.ExternalId)
				assert.Equal(t, tc.expected[i].Room.Description, subs[i].Room.Description)
				assert.WithinDuration(t, tc.expected[i].Room.CreatedAt, subs[i].CreatedAt, time.Second)
				assert.WithinDuration(t, tc.expected[i].Room.UpdatedAt, subs[i].UpdatedAt, time.Second)
			}
		})
	}
}

func Test_getMessages(t *testing.T) {
	fixedTime := time.Date(2025, time.June, 28, 11, 17, 54, 692262000, time.Local)
	mockMessages := []database.Message{
		{
			Id:        3,
			RoomId:    1,
			UserId:    1,
			Content:   "Hello!",
			SeqId:     3,
			CreatedAt: fixedTime,
		},
		{
			Id:        2,
			RoomId:    1,
			UserId:    2,
			Content:   "Hi there!",
			SeqId:     2,
			CreatedAt: fixedTime.Add(-10 * time.Minute),
		},
		{
			Id:        1,
			RoomId:    1,
			UserId:    3,
			Content:   "Hey!",
			SeqId:     1,
			CreatedAt: fixedTime.Add(-20 * time.Minute),
		},
	}

	tcases := []struct {
		name                       string
		roomId                     string
		userId                     int
		mockRoom                   database.Room
		mockGetRoomByExternalIdErr error
		mockMessages               []database.Message
		mockGetMessagesErr         error
		limit                      string
		before                     string
		after                      string
		expected                   []types.Message
		expectedErr                *ApiError
	}{
		{
			name:                       "successfully retrieves messages with no query parameters",
			roomId:                     "EoGKUXPHgz",
			userId:                     1,
			mockRoom:                   database.Room{Id: 1, ExternalId: "EoGKUXPHgz"},
			mockGetRoomByExternalIdErr: nil,
			mockMessages:               mockMessages,
			mockGetMessagesErr:         nil,
			expected: func() []types.Message {
				messages := make([]types.Message, len(mockMessages))
				for i, msg := range mockMessages {
					messages[i] = types.Message{
						SeqId:     msg.SeqId,
						RoomId:    msg.RoomId,
						UserId:    msg.UserId,
						Content:   msg.Content,
						Timestamp: msg.CreatedAt,
					}
				}
				return messages
			}(),
			expectedErr: nil,
		},
		{
			name:                       "successfully retrieves messages with after",
			roomId:                     "EoGKUXPHgz",
			userId:                     1,
			mockRoom:                   database.Room{Id: 1, ExternalId: "EoGKUXPHgz"},
			mockGetRoomByExternalIdErr: nil,
			mockMessages: func() []database.Message {
				var messages []database.Message
				for _, msg := range mockMessages {
					if msg.SeqId > 1 {
						messages = append(messages, msg)
					}
				}
				return messages
			}(),
			mockGetMessagesErr: nil,
			after:              "1",
			expected: func() []types.Message {
				var messages []types.Message
				for _, msg := range mockMessages {
					if msg.SeqId > 1 {
						messages = append(messages, types.Message{
							SeqId:     msg.SeqId,
							RoomId:    msg.RoomId,
							UserId:    msg.UserId,
							Content:   msg.Content,
							Timestamp: msg.CreatedAt,
						})
					}
				}
				return messages
			}(),
			expectedErr: nil,
		},
		{
			name:                       "successfully retrieves messages with limit",
			roomId:                     "EoGKUXPHgz",
			userId:                     1,
			mockRoom:                   database.Room{Id: 1, ExternalId: "EoGKUXPHgz"},
			mockGetRoomByExternalIdErr: nil,
			mockMessages: func() []database.Message {
				messages := make([]database.Message, 0, 2)
				messages = append(messages, mockMessages[:2]...)
				return messages
			}(),
			mockGetMessagesErr: nil,
			limit:              "2",
			expected: func() []types.Message {
				messages := make([]types.Message, 0, 2)
				for _, msg := range mockMessages[:2] {
					messages = append(messages, types.Message{
						SeqId:     msg.SeqId,
						RoomId:    msg.RoomId,
						UserId:    msg.UserId,
						Content:   msg.Content,
						Timestamp: msg.CreatedAt,
					})
				}
				return messages
			}(),
		},
		{
			name:                       "successfully retrieves messages with before",
			roomId:                     "EoGKUXPHgz",
			userId:                     1,
			mockRoom:                   database.Room{Id: 1, ExternalId: "EoGKUXPHgz"},
			mockGetRoomByExternalIdErr: nil,
			mockMessages: func() []database.Message {
				messages := make([]database.Message, 0, 2)
				messages = append(messages, mockMessages[1:]...)
				return messages
			}(),
			mockGetMessagesErr: nil,
			before:             "3", // Get messages before SeqId 3
			expected: func() []types.Message {
				messages := make([]types.Message, 0, 2)
				for _, msg := range mockMessages[1:] { // Exclude the message with SeqId 3
					messages = append(messages, types.Message{
						SeqId:     msg.SeqId,
						RoomId:    msg.RoomId,
						UserId:    msg.UserId,
						Content:   msg.Content,
						Timestamp: msg.CreatedAt,
					})
				}
				return messages
			}(),
		},
		{
			name:                       "missing room_id query parameter",
			userId:                     1,
			mockRoom:                   database.Room{},
			mockGetRoomByExternalIdErr: nil,
			mockMessages:               nil,
			mockGetMessagesErr:         nil,
			expected:                   nil,
			expectedErr:                NewBadRequestError(),
		},
		{
			name:                       "room not found",
			roomId:                     "nonexistent",
			userId:                     1,
			mockRoom:                   database.Room{},
			mockGetRoomByExternalIdErr: sql.ErrNoRows,
			mockMessages:               nil,
			mockGetMessagesErr:         nil,
			expected:                   nil,
			expectedErr:                NewNotFoundError(),
		},
		{
			name:                       "GetExternalId db error",
			roomId:                     "EoGKUXPHgz",
			userId:                     1,
			mockRoom:                   database.Room{},
			mockGetRoomByExternalIdErr: errors.New("db error"),
			mockMessages:               nil,
			mockGetMessagesErr:         nil,
			expected:                   nil,
			expectedErr:                NewInternalServerError(nil),
		},
		{
			name:                       "GetMessages with no messages",
			roomId:                     "EoGKUXPHgz",
			userId:                     1,
			mockRoom:                   database.Room{Id: 1, ExternalId: "EoGKUXPHgz"},
			mockGetRoomByExternalIdErr: nil,
			mockMessages:               []database.Message{},
			mockGetMessagesErr:         nil,
			expected:                   []types.Message{},
			expectedErr:                nil,
		},
		{
			name:                       "GetMessages db error",
			roomId:                     "EoGKUXPHgz",
			userId:                     1,
			mockRoom:                   database.Room{Id: 1, ExternalId: "EoGKUXPHgz"},
			mockGetRoomByExternalIdErr: nil,
			mockMessages:               nil,
			mockGetMessagesErr:         errors.New("db error"),
			expected:                   nil,
			expectedErr:                NewInternalServerError(nil),
		},
		{
			name:                       "invalid after parameter",
			roomId:                     "EoGKUXPHgz",
			userId:                     1,
			mockRoom:                   database.Room{Id: 1, ExternalId: "EoGKUXPHgz"},
			mockGetRoomByExternalIdErr: nil,
			mockMessages:               nil,
			mockGetMessagesErr:         nil,
			after:                      "invalid", // Invalid after parameter
			expected:                   nil,
			expectedErr:                NewBadRequestError(),
		},
		{
			name:                       "invalid limit parameter",
			roomId:                     "EoGKUXPHgz",
			userId:                     1,
			mockRoom:                   database.Room{Id: 1, ExternalId: "EoGKUXPHgz"},
			mockGetRoomByExternalIdErr: nil,
			mockMessages:               nil,
			mockGetMessagesErr:         nil,
			limit:                      "invalid", // Invalid after parameter
			expected:                   nil,
			expectedErr:                NewBadRequestError(),
		},
		{
			name:                       "invalid before parameter",
			roomId:                     "EoGKUXPHgz",
			userId:                     1,
			mockRoom:                   database.Room{Id: 1, ExternalId: "EoGKUXPHgz"},
			mockGetRoomByExternalIdErr: nil,
			mockMessages:               nil,
			mockGetMessagesErr:         nil,
			before:                     "invalid", // Invalid after parameter
			expected:                   nil,
			expectedErr:                NewBadRequestError(),
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &database.MockGoChatRepository{}
			defer mockRepo.AssertExpectations(t)

			if tc.mockRoom.Id != 0 || tc.mockGetRoomByExternalIdErr != nil {
				mockRepo.On("GetRoomByExternalId", tc.roomId).Return(tc.mockRoom, tc.mockGetRoomByExternalIdErr).Once()
			}

			if tc.mockMessages != nil || tc.mockGetMessagesErr != nil {
				// Convert query parameters to integers for the mock
				// Note: limit, after, and before are optional, so they may be empty
				var limitInt, afterInt, beforeInt int
				var err error
				if tc.limit != "" {
					limitInt, err = strconv.Atoi(tc.limit)
				}
				if tc.after != "" {
					afterInt, err = strconv.Atoi(tc.after)
				}
				if tc.before != "" {
					beforeInt, err = strconv.Atoi(tc.before)
				}
				assert.NoError(t, err, "failed to convert query parameters to integers")
				mockRepo.On("GetMessages", tc.mockRoom.Id, afterInt, beforeInt, limitInt).Return(tc.mockMessages, tc.mockGetMessagesErr).Once()
			}

			app := NewGoChatApp(http.NewServeMux(), log.Default(), nil, mockRepo, nil, &config.Config{})

			var queryString string
			if tc.roomId != "" {
				queryString = fmt.Sprintf("?room_id=%s", tc.roomId)
			}

			if tc.limit != "" {
				queryString += fmt.Sprintf("&limit=%s", tc.limit)
			}

			if tc.after != "" {
				queryString += fmt.Sprintf("&after=%s", tc.after)
			}

			if tc.before != "" {
				queryString += fmt.Sprintf("&before=%s", tc.before)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/messages"+queryString, nil)

			if tc.userId > 0 {
				ctx := WithUserId(req.Context(), tc.userId)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			app.getMessages(rr, req)

			if tc.expectedErr != nil {
				var apiErr ApiError
				err := json.NewDecoder(rr.Body).Decode(&apiErr)
				assert.NoError(t, err, "failed to decode error response")
				assert.Equal(t, rr.Code, tc.expectedErr.StatusCode, "expected status code to match")
				assert.Equal(t, *tc.expectedErr, apiErr, "expected ApiError response")
				return
			}

			assert.Equal(t, http.StatusOK, rr.Code)
			var messages []types.Message
			err := json.NewDecoder(rr.Body).Decode(&messages)
			assert.NoError(t, err, "failed to decode response: %v", err)
			assert.Len(t, messages, len(tc.expected), "expected number of messages to match")
			for i := range messages {
				assert.Equal(t, tc.expected[i].UserId, messages[i].UserId)
				assert.Equal(t, tc.expected[i].Content, messages[i].Content)
				assert.Equal(t, tc.expected[i].SeqId, messages[i].SeqId)
				assert.Equal(t, tc.expected[i].Timestamp, messages[i].Timestamp)
			}
		})
	}
}
func Test_serveWs(t *testing.T) {
	mockUser := database.User{
		Id:           1,
		Username:     "testuser",
		EmailAddress: "testuser@example.com",
		PasswordHash: "examplehash",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	t.Run("successful websocket upgrade and client registration", func(t *testing.T) {
		mockRepo := &database.MockGoChatRepository{}
		defer mockRepo.AssertExpectations(t)

		su := &stats.MockStatsUpdater{}
		defer su.AssertExpectations(t)

		su.On("Incr", "NumActiveClients").Return(nil).Once()
		su.On("Decr", "NumActiveClients").Return(nil).Maybe()
		su.On("RegisterMetric", mock.Anything).Return(nil).Times(4)

		cs, err := server.NewChatServer(log.Default(), mockRepo, su)
		if err != nil {
			t.Fatalf("failed to create chat server: %v", err)
		}

		mockRepo.On("GetAccountById", mockUser.Id).Return(mockUser, nil).Once()
		mockRepo.On("ListSubscriptions", mockUser.Id).Return([]database.Subscription{}, nil).Once() // called during client registration

		app := NewGoChatApp(http.NewServeMux(), log.Default(), cs, mockRepo, nil, &config.Config{})

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithUserId(r.Context(), 1)
			r = r.WithContext(ctx)
			app.serveWs(w, r)
		}))
		defer srv.Close()

		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws"
		header := http.Header{}

		conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
		defer func() {
			if conn != nil {
				conn.Close()
			}
		}()
		assert.NoError(t, err)
		assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	})

	errorTestCases := []struct {
		name        string
		userId      int
		mockUser    database.User
		mockErr     error
		expectedErr *ApiError
	}{
		{
			name:        "unauthorized user",
			userId:      0,
			mockUser:    database.User{},
			mockErr:     nil,
			expectedErr: NewUnauthorizedError(),
		},
		{
			name:        "user not found",
			userId:      1,
			mockUser:    database.User{},
			mockErr:     sql.ErrNoRows,
			expectedErr: NewNotFoundError(),
		},
		{
			name:        "db error",
			userId:      1,
			mockUser:    database.User{},
			mockErr:     errors.New("db error"),
			expectedErr: NewInternalServerError(nil),
		},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &database.MockGoChatRepository{}
			defer mockRepo.AssertExpectations(t)

			su := &stats.MockStatsUpdater{}
			defer su.AssertExpectations(t)
			su.On("RegisterMetric", mock.Anything).Return(nil).Times(4)

			cs, err := server.NewChatServer(log.Default(), mockRepo, su)
			assert.NoError(t, err, "failed to create chat server")
			app := NewGoChatApp(http.NewServeMux(), log.Default(), cs, mockRepo, nil, &config.Config{})

			if tc.mockUser != (database.User{}) || tc.mockErr != nil {
				mockRepo.On("GetAccountById", tc.userId).Return(tc.mockUser, tc.mockErr).Once()
			}

			req := httptest.NewRequest(http.MethodGet, "/api/ws", nil)

			if tc.userId > 0 {
				ctx := WithUserId(req.Context(), 1)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			app.serveWs(rr, req)

			var apiErr ApiError
			err = json.NewDecoder(rr.Body).Decode(&apiErr)
			assert.NoError(t, err, "failed to decode ApiError response")
			assert.Equal(t, apiErr.StatusCode, rr.Code)
			assert.Equal(t, *tc.expectedErr, apiErr, "expected ApiError to match")
		})
	}
}
