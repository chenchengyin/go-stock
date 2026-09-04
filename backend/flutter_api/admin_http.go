package flutter_api

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const adminSessionCookieName = "go_stock_admin_session"

type adminPrincipalContextKey struct{}

func AdminPrincipalFromContext(ctx context.Context) (AdminPrincipal, bool) {
	principal, ok := ctx.Value(adminPrincipalContextKey{}).(AdminPrincipal)
	return principal, ok
}

func NewAdminHTTPHandler(service *AdminService, moduleService *ModuleService) http.Handler {
	policy := newAdminOriginPolicy(os.Getenv("GO_STOCK_ADMIN_DEV_ORIGINS"))
	mux := http.NewServeMux()
	mux.HandleFunc("/api/admin/login", handleAdminLoginWithPolicy(service, policy))
	mux.HandleFunc("/api/admin/logout", handleAdminLogoutWithPolicy(service, policy))
	usersHandler := requireAdminWithPolicy(service, http.HandlerFunc(handleAdminUsers(service)), policy)
	mux.Handle("/api/admin/users", usersHandler)
	mux.Handle("/api/admin/users/", usersHandler)
	meHandler := requireAdminWithPolicy(service, handleAdminMe(service), policy)
	mux.Handle("/api/admin/me", meHandler)
	modulesHandler := requireAdminWithPolicy(service, handleAdminModules(moduleService), policy)
	mux.Handle("/api/admin/modules", modulesHandler)
	moduleUsersHandler := requireAdminWithPolicy(service, handleAdminModuleUsers(moduleService), policy)
	mux.Handle("/api/admin/modules/", moduleUsersHandler)
	accessHandler := requireAdminWithPolicy(service, handleAdminAccess(moduleService), policy)
	mux.Handle("/api/admin/access", accessHandler)
	return adminCORSMiddleware(policy, mux)
}

func RequireAdmin(service *AdminService, next http.Handler) http.Handler {
	return requireAdminWithPolicy(service, next, newAdminOriginPolicy(os.Getenv("GO_STOCK_ADMIN_DEV_ORIGINS")))
}

func requireAdminWithPolicy(service *AdminService, next http.Handler, policy adminOriginPolicy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if !policy.requestAllowed(r) {
			writeAdminCSRFError(w)
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
	return handleAdminLoginWithPolicy(service, newAdminOriginPolicy(os.Getenv("GO_STOCK_ADMIN_DEV_ORIGINS")))
}

func handleAdminLoginWithPolicy(service *AdminService, policy adminOriginPolicy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !policy.requestAllowed(r) {
			writeAdminCSRFError(w)
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
	return handleAdminLogoutWithPolicy(service, newAdminOriginPolicy(os.Getenv("GO_STOCK_ADMIN_DEV_ORIGINS")))
}

func handleAdminLogoutWithPolicy(service *AdminService, policy adminOriginPolicy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !policy.requestAllowed(r) {
			writeAdminCSRFError(w)
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

func handleAdminMe(service *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		principal, ok := AdminPrincipalFromContext(r.Context())
		if !ok {
			WriteAuthError(w, newAuthError(http.StatusUnauthorized, "ADMIN_UNAUTHENTICATED", "管理员未登录"))
			return
		}
		var user AuthUser
		if err := service.dao.WithContext(r.Context()).First(&user, "id = ?", principal.UserID).Error; err != nil {
			WriteAuthError(w, authHTTPError(err))
			return
		}
		var session AdminSession
		if err := service.dao.WithContext(r.Context()).First(&session, "id = ? AND user_id = ?", principal.SessionID, principal.UserID).Error; err != nil {
			WriteAuthError(w, authHTTPError(err))
			return
		}
		WriteJSON(w, struct {
			User      AdminUser `json:"user"`
			ExpiresAt time.Time `json:"expiresAt"`
		}{User: toAdminUser(user), ExpiresAt: session.ExpiresAt})
	}
}

func adminSessionCookie(r *http.Request, value string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{Name: adminSessionCookieName, Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: expiresAt, Secure: r.TLS != nil}
}

func clearAdminSessionCookie(r *http.Request) *http.Cookie {
	return &http.Cookie{Name: adminSessionCookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: time.Unix(1, 0), MaxAge: -1, Secure: r.TLS != nil}
}

func adminRequestOriginAllowed(r *http.Request) bool {
	return newAdminOriginPolicy(os.Getenv("GO_STOCK_ADMIN_DEV_ORIGINS")).requestAllowed(r)
}

type adminOriginPolicy struct {
	development []adminOrigin
}

func newAdminOriginPolicy(raw string) adminOriginPolicy {
	policy := adminOriginPolicy{}
	for _, configured := range strings.Split(raw, ",") {
		if origin, ok := parseAdminOrigin(strings.TrimSpace(configured)); ok {
			policy.development = append(policy.development, origin)
		}
	}
	return policy
}

func (p adminOriginPolicy) requestAllowed(r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
		return true
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsedOrigin, ok := parseAdminOrigin(origin)
	if !ok {
		return false
	}
	return p.originAllowed(r, parsedOrigin)
}

func (p adminOriginPolicy) originAllowed(r *http.Request, origin adminOrigin) bool {
	if requestOrigin, ok := requestAdminOrigin(r); ok && origin == requestOrigin {
		return true
	}
	for _, configuredOrigin := range p.development {
		if configuredOrigin == origin {
			return true
		}
	}
	return false
}

func writeAdminCSRFError(w http.ResponseWriter) {
	WriteAuthError(w, newAuthError(http.StatusForbidden, "CSRF_INVALID", "请求来源不被允许"))
}

func adminCORSMiddleware(policy adminOriginPolicy, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		originAllowed := false
		if origin != "" {
			if parsedOrigin, ok := parseAdminOrigin(origin); ok {
				originAllowed = policy.originAllowed(r, parsedOrigin)
				if originAllowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Requested-With")
					w.Header().Set("Access-Control-Max-Age", "86400")
					w.Header().Add("Vary", "Origin")
				}
			}
		}
		if r.Method == http.MethodOptions {
			if origin != "" && !originAllowed {
				writeAdminCSRFError(w)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type adminOrigin struct {
	scheme string
	host   string
	port   int
}

func parseAdminOrigin(raw string) (adminOrigin, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return adminOrigin{}, false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return adminOrigin{}, false
	}
	port, ok := effectiveAdminPort(scheme, parsed.Port())
	if !ok || parsed.Hostname() == "" {
		return adminOrigin{}, false
	}
	return adminOrigin{scheme: scheme, host: strings.ToLower(parsed.Hostname()), port: port}, true
}

func requestAdminOrigin(r *http.Request) (adminOrigin, bool) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if r.URL != nil && r.URL.Scheme != "" {
		scheme = strings.ToLower(r.URL.Scheme)
	}
	if r.Host == "" {
		return adminOrigin{}, false
	}
	parsed, err := url.Parse("//" + r.Host)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Hostname() == "" {
		return adminOrigin{}, false
	}
	port, ok := effectiveAdminPort(scheme, parsed.Port())
	if !ok {
		return adminOrigin{}, false
	}
	return adminOrigin{scheme: scheme, host: strings.ToLower(parsed.Hostname()), port: port}, true
}

func effectiveAdminPort(scheme, portValue string) (int, bool) {
	if portValue == "" {
		if scheme == "https" {
			return 443, true
		}
		if scheme == "http" {
			return 80, true
		}
		return 0, false
	}
	port, err := strconv.Atoi(portValue)
	return port, err == nil && port > 0 && port <= 65535
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
