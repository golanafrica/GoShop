package userhandler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Goshop/config/setupLogging"
	mockrepo "Goshop/mocks/repository"

	userentity "Goshop/domain/entity/user_entity"
	userrepository "Goshop/domain/repository/user_repository"
	userhandler "Goshop/interfaces/handler/user_handler"
	"Goshop/interfaces/utils"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

func TestRegister_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mockrepo.NewMockUserRepository(ctrl)
	handler := userhandler.NewUserHandler(
		mockRepo,
		nil,
		nil,
		setupLogging.GetTestLogger(),
	)

	// ✅ Mot de passe fort : Majuscule, Chiffre, Caractère spécial, > 8 chars
	body := map[string]string{
		"email":    "test@example.com",
		"password": "ValidPassword123!",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mockRepo.
		EXPECT().
		FindUserByEmail("test@example.com").
		Return(nil, userrepository.ErrUserNotFound)

	mockRepo.
		EXPECT().
		CreateUser(gomock.Any()).
		Return(&userentity.UserEntity{
			ID:       "test-id",
			Email:    "test@example.com",
			Password: "hashed-password",
			Role:     userentity.RoleMerchant,
			Active:   true,
			Status:   userentity.StatusActive,
		}, nil)

	err := handler.Register(w, req)

	assert.NoError(t, err, "L'inscription ne doit pas retourner d'erreur")
	assert.Equal(t, http.StatusCreated, w.Code, "Le code HTTP doit être 201 Created")
}

func TestLoginHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	handler := userhandler.NewUserHandler(
		repo,
		nil,
		nil,
		setupLogging.GetTestLogger(),
	)

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("pwd123"), bcrypt.DefaultCost)

	repo.EXPECT().
		FindUserByEmail("test@example.com").
		Return(&userentity.UserEntity{
			ID:       "123",
			Email:    "test@example.com",
			Password: string(hashedPassword),
			Role:     userentity.RoleMerchant,
			Active:   true,
			Status:   userentity.StatusActive,
		}, nil)

	body := map[string]string{
		"email":    "test@example.com",
		"password": "pwd123",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	err := handler.Login(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	// ✅ CORRECTION CRITIQUE : Utiliser map[string]interface{} car la réponse contient "expires_in": 900 (un nombre)
	var response map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Contains(t, response, "token")
	assert.NotEmpty(t, response["token"])
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	handler := userhandler.NewUserHandler(
		repo,
		nil,
		nil,
		setupLogging.GetTestLogger(),
	)

	repo.EXPECT().
		FindUserByEmail("wrong@example.com").
		Return(nil, userrepository.ErrUserNotFound)

	body := map[string]string{
		"email":    "wrong@example.com",
		"password": "pwd123",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	err := handler.Login(w, req)

	assert.Error(t, err)
}

func TestMeHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	handler := userhandler.NewUserHandler(
		repo,
		nil,
		nil,
		setupLogging.GetTestLogger(),
	)

	expectedUser := &userentity.UserEntity{
		ID:       "user-123",
		Email:    "test@example.com",
		Password: "hashed-password",
		Role:     userentity.RoleMerchant,
		Active:   true,
		Status:   userentity.StatusActive,
	}

	repo.EXPECT().
		FindUserByID("user-123").
		Return(expectedUser, nil).
		Times(1)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	ctx := utils.SetUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	err := handler.Me(w, req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err = json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)

	assert.Equal(t, "user-123", response["id"])
	assert.Equal(t, "tes***@example.com", response["email"])
}

func TestMeHandler_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	handler := userhandler.NewUserHandler(
		repo,
		nil,
		nil,
		setupLogging.GetTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()

	err := handler.Me(w, req)

	assert.Error(t, err)
	assert.Equal(t, utils.ErrUnauthorized, err)
}

func TestMeHandler_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	handler := userhandler.NewUserHandler(
		repo,
		nil,
		nil,
		setupLogging.GetTestLogger(),
	)

	repo.EXPECT().
		FindUserByID("user-123").
		Return(nil, userrepository.ErrUserNotFound).
		Times(1)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	ctx := utils.SetUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	err := handler.Me(w, req)

	assert.Error(t, err)
}

func TestMeHandler_InternalServerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	handler := userhandler.NewUserHandler(
		repo,
		nil,
		nil,
		setupLogging.GetTestLogger(),
	)

	repo.EXPECT().
		FindUserByID("user-123").
		Return(nil, userrepository.ErrUserCreateFailed).
		Times(1)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	ctx := utils.SetUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	err := handler.Me(w, req)

	assert.Error(t, err)
	assert.Equal(t, utils.ErrInternalServer, err)
}
