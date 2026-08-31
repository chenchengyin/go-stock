package flutter_api

import (
	"net/http"
	"strings"
)

func NewAdminHTTPHandler(service *AdminService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/admin/login", handleAdminLogin(service))
	usersHandler := RequireAdmin(service, http.HandlerFunc(handleAdminUsers(service)))
	mux.Handle("/api/admin/users", usersHandler)
	mux.Handle("/api/admin/users/", usersHandler)
	return mux
}

func RequireAdmin(service *AdminService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		if err := service.Authenticate(bearerTokenFromRequest(r)); err != nil {
			WriteAuthError(w, err)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handleAdminLogin(service *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var input AdminLoginInput
		if err := decodeJSONBody(r, &input); err != nil {
			WriteAuthError(w, err)
			return
		}

		response, err := service.Login(r.Context(), input)
		if err != nil {
			WriteAuthError(w, err)
			return
		}

		WriteJSON(w, response)
	}
}

func handleAdminUsers(adminService *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/admin/users" {
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			users, err := adminService.ListUsers(r.Context(), r.URL.Query().Get("keyword"))
			if err != nil {
				WriteAuthError(w, err)
				return
			}
			WriteJSON(w, users)
			return
		}

		const prefix = "/api/admin/users/"
		if !strings.HasPrefix(r.URL.Path, prefix) || r.Method != http.MethodPatch {
			http.NotFound(w, r)
			return
		}

		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] != "status" {
			http.NotFound(w, r)
			return
		}

		var input struct {
			Status string `json:"status"`
		}
		if err := decodeJSONBody(r, &input); err != nil {
			WriteAuthError(w, err)
			return
		}

		user, err := adminService.UpdateUserStatus(r.Context(), parts[0], input.Status)
		if err != nil {
			WriteAuthError(w, err)
			return
		}
		WriteJSON(w, user)
	}
}
