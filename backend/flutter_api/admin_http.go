package flutter_api

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const adminSessionCookieName = "go_stock_admin_session"

type adminPrincipalContextKey struct{}

func AdminPrincipalFromContext(ctx context.Context) (AdminPrincipal, bool) {
	principal, ok := ctx.Value(adminPrincipalContextKey{}).(AdminPrincipal)
	return principal, ok
}

func NewAdminHTTPHandler(service *AdminService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/admin/login", handleAdminLogin(service))
	mux.HandleFunc("/api/admin/logout", handleAdminLogout(service))
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
		if !adminRequestOriginAllowed(r) {
			WriteAuthError(w, newAuthError(http.StatusForbidden, "ORIGIN_FORBIDDEN", "请求来源不被允许"))
			return
		}
		cookie, err := r.Cookie(adminSessionCookieName)
		if err != nil {
			WriteAuthError(w, newAuthError(http.StatusUnauthorized, "ADMIN_UNAUTHENTICATED", "管理员未登录"))
			return
		}
		principal, err := service.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			WriteAuthError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), adminPrincipalContextKey{}, principal)))
	})
}

func handleAdminLogin(service *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !adminRequestOriginAllowed(r) {
			WriteAuthError(w, newAuthError(http.StatusForbidden, "ORIGIN_FORBIDDEN", "请求来源不被允许"))
			return
		}
		var input AdminLoginInput
		if err := decodeJSONBody(r, &input); err != nil {
			WriteAuthError(w, err)
			return
		}
		rawToken, response, err := service.Login(r.Context(), input)
		if err != nil {
			WriteAuthError(w, err)
			return
		}
		http.SetCookie(w, adminSessionCookie(r, rawToken, response.ExpiresAt))
		WriteJSON(w, struct {
			Status string `json:"status"`
			*AdminSessionResponse
		}{Status: "ok", AdminSessionResponse: response})
	}
}

func handleAdminLogout(service *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !adminRequestOriginAllowed(r) {
			WriteAuthError(w, newAuthError(http.StatusForbidden, "ORIGIN_FORBIDDEN", "请求来源不被允许"))
			return
		}
		if cookie, err := r.Cookie(adminSessionCookieName); err == nil {
			if err := service.Logout(r.Context(), cookie.Value); err != nil {
				WriteAuthError(w, err)
				return
			}
		}
		http.SetCookie(w, clearAdminSessionCookie(r))
		WriteJSON(w, map[string]string{"status": "ok"})
	}
}

func adminSessionCookie(r *http.Request, value string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{Name: adminSessionCookieName, Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: expiresAt, Secure: r.TLS != nil}
}

func clearAdminSessionCookie(r *http.Request) *http.Cookie {
	return &http.Cookie{Name: adminSessionCookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: time.Unix(1, 0), MaxAge: -1, Secure: r.TLS != nil}
}

func adminRequestOriginAllowed(r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
		return true
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if strings.EqualFold(parsed.Host, r.Host) {
		return true
	}
	for _, configured := range strings.Split(os.Getenv("GO_STOCK_ADMIN_DEV_ORIGIN"), ",") {
		if strings.TrimSpace(configured) == origin {
			return true
		}
	}
	return false
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
