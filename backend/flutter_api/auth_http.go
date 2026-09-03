package flutter_api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"gorm.io/gorm"
)

func NewAuthHTTPHandler(service *AuthService, moduleServices ...*ModuleService) http.Handler {
	moduleService := NewModuleService(service.dao)
	if len(moduleServices) > 0 && moduleServices[0] != nil {
		moduleService = moduleServices[0]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/register", handleAuthRegister(service))
	mux.HandleFunc("/api/auth/login", handleAuthLogin(service))
	mux.HandleFunc("/api/auth/me", handleAuthMe(service))
	mux.HandleFunc("/api/auth/logout", handleAuthLogout(service))
	mux.HandleFunc("/api/auth/profile", handleAuthProfile(service))
	mux.HandleFunc("/api/auth/modules", handleAuthModules(moduleService))
	mux.HandleFunc("/register", handleAuthRegister(service))
	mux.HandleFunc("/login", handleAuthLogin(service))
	mux.HandleFunc("/me", handleAuthMe(service))
	mux.HandleFunc("/logout", handleAuthLogout(service))
	mux.HandleFunc("/profile", handleAuthProfile(service))
	return mux
}

func handleAuthRegister(service *AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var input RegisterInput
		if err := decodeJSONBody(r, &input); err != nil {
			WriteAuthError(w, err)
			return
		}

		result, err := service.Register(r.Context(), input)
		if err != nil {
			WriteAuthError(w, err)
			return
		}

		WriteJSONStatus(w, http.StatusCreated, result)
	}
}

func handleAuthLogin(service *AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var input LoginInput
		if err := decodeJSONBody(r, &input); err != nil {
			WriteAuthError(w, err)
			return
		}

		session, err := service.Login(r.Context(), input)
		if err != nil {
			WriteAuthError(w, err)
			return
		}

		WriteJSON(w, session)
	}
}

func handleAuthMe(service *AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			WriteAuthError(w, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证"))
			return
		}

		var user AuthUser
		if err := service.dao.WithContext(r.Context()).First(&user, "id = ?", principal.UserID).Error; err != nil {
			WriteAuthError(w, authHTTPError(err))
			return
		}

		var session AuthSession
		if err := service.dao.WithContext(r.Context()).First(&session, "id = ?", principal.SessionID).Error; err != nil {
			WriteAuthError(w, authHTTPError(err))
			return
		}

		WriteJSON(w, map[string]any{
			"user":      toPublicUser(user),
			"expiresAt": session.ExpiresAt,
		})
	}
}

func handleAuthLogout(service *AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := service.Logout(r.Context(), bearerTokenFromRequest(r)); err != nil {
			WriteAuthError(w, err)
			return
		}

		WriteJSON(w, map[string]string{"status": "ok"})
	}
}

func handleAuthProfile(service *AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			WriteAuthError(w, newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证"))
			return
		}

		var input UpdateProfileInput
		if err := decodeJSONBody(r, &input); err != nil {
			WriteAuthError(w, err)
			return
		}

		user, err := service.UpdateProfile(r.Context(), principal, UpdateProfileInput{Nickname: input.Nickname})
		if err != nil {
			WriteAuthError(w, err)
			return
		}

		WriteJSON(w, user)
	}
}

func decodeJSONBody(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "请求参数格式不正确")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return newAuthError(http.StatusBadRequest, "INVALID_ARGUMENT", "请求参数格式不正确")
	}
	return nil
}

func authHTTPError(err error) error {
	if err == nil {
		return nil
	}

	var authErr *AuthError
	if errors.As(err, &authErr) {
		return err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return newAuthError(http.StatusUnauthorized, "UNAUTHENTICATED", "未认证")
	}

	return err
}
