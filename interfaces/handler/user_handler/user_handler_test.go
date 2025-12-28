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
	userrepository "Goshop/domain/repository/user_repository" // IMPORT AJOUTÃ‰
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
	handler := userhandler.NewUserHandler(mockRepo, setupLogging.GetTestLogger())

	// GIVEN: payload
	body := map[string]string{
		"email":    "test@example.com",
		"password": "123456",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// MOCK: user nâ€™existe pas encore
	mockRepo.
		EXPECT().
		FindUserByEmail("test@example.com").
		Return(nil, userrepository.ErrUserNotFound) // âš ï¸ PAS DE POINT ICI !

	// MOCK: crÃ©ation OK
	mockRepo.
		EXPECT().
		CreateUser(gomock.Any()).
		Return(&userentity.UserEntity{
			ID:       "test-id",
			Email:    "test@example.com",
			Password: "hashed-password",
		}, nil)

	// WHEN: appel handler
	err := handler.Register(w, req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// THEN
	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestLoginHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	handler := userhandler.NewUserHandler(repo, setupLogging.GetTestLogger())

	// 1. PrÃ©paration du mot de passe hashÃ©
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("pwd123"), bcrypt.DefaultCost)

	// 2. Mock du repository
	repo.EXPECT().
		FindUserByEmail("test@example.com").
		Return(&userentity.UserEntity{
			ID:       "123",
			Email:    "test@example.com",
			Password: string(hashedPassword), // Hash rÃ©el
		}, nil)

	// 3. RequÃªte HTTP
	body := map[string]string{
		"email":    "test@example.com",
		"password": "pwd123",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// 4. Appel direct du handler (pas via Chi router)
	err := handler.Login(w, req)

	// 5. VÃ©rifications
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	// VÃ©rifie la rÃ©ponse JSON
	var response map[string]string
	err = json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Contains(t, response, "token")
	assert.NotEmpty(t, response["token"])
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	handler := userhandler.NewUserHandler(repo, setupLogging.GetTestLogger())

	// Mock : utilisateur non trouvÃ©
	repo.EXPECT().
		FindUserByEmail("wrong@example.com").
		Return(nil, userrepository.ErrUserNotFound)

	// RequÃªte avec mauvais email
	body := map[string]string{
		"email":    "wrong@example.com",
		"password": "pwd123",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Appel du handler
	err := handler.Login(w, req)

	// Doit retourner une erreur
	assert.Error(t, err)
	// VÃ©rifie que c'est bien une erreur d'authentification
	// (dÃ©pend de comment tu gÃ¨res les erreurs dans ton handler)
}

func TestMeHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	handler := userhandler.NewUserHandler(repo, setupLogging.GetTestLogger())

	// GIVEN: Un utilisateur existant
	expectedUser := &userentity.UserEntity{
		ID:       "user-123",
		Email:    "test@example.com",
		Password: "hashed-password",
	}

	// ðŸ”¥ CORRECTION: Mock FindUserByID, pas FindUserByEmail
	repo.EXPECT().
		FindUserByID("user-123").
		Return(expectedUser, nil).
		Times(1)

	// WHEN: RequÃªte avec userID dans le contexte
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	ctx := utils.SetUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Appel du handler
	err := handler.Me(w, req)

	// THEN: Pas d'erreur, status 200
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	// VÃ©rifie la rÃ©ponse JSON
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
	handler := userhandler.NewUserHandler(repo, setupLogging.GetTestLogger())

	// GIVEN: RequÃªte SANS userID dans le contexte
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()

	// WHEN: Appel du handler
	err := handler.Me(w, req)

	// THEN: Doit retourner ErrUnauthorized
	assert.Error(t, err)
	assert.Equal(t, utils.ErrUnauthorized, err)
}

func TestMeHandler_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	handler := userhandler.NewUserHandler(repo, setupLogging.GetTestLogger())

	// GIVEN: UserID existe mais utilisateur pas en base
	repo.EXPECT().
		FindUserByID("user-123").
		Return(nil, userrepository.ErrUserNotFound).
		Times(1)

	// WHEN: RequÃªte avec userID
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	ctx := utils.SetUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Appel du handler
	err := handler.Me(w, req)

	// THEN: Erreur car utilisateur non trouvÃ©
	assert.Error(t, err)
	// Selon ton GetProfileUsecase, devrait Ãªtre utils.ErrUserNotFound
}

func TestMeHandler_InternalServerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mockrepo.NewMockUserRepository(ctrl)
	handler := userhandler.NewUserHandler(repo, setupLogging.GetTestLogger())

	// GIVEN: Erreur interne du repository
	repo.EXPECT().
		FindUserByID("user-123").
		Return(nil, userrepository.ErrUserCreateFailed).
		Times(1)

	// WHEN: RequÃªte avec userID
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	ctx := utils.SetUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Appel du handler
	err := handler.Me(w, req)

	// THEN: Erreur interne
	assert.Error(t, err)

	// ðŸ”¥ ADAPTÃ‰ Ã  ton implÃ©mentation :
	// Si ton GetProfileUsecase retourne l'erreur brute :
	// assert.Equal(t, userrepository.ErrUserCreateFailed, err)

	// Si ton GetProfileUsecase transforme en utils.ErrInternalServer :
	assert.Equal(t, utils.ErrInternalServer, err)
}
