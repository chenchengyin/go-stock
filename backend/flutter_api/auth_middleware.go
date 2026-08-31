package flutter_api

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type authPrincipalContextKey struct{}

var publicPaths = map[string]bool{
	"/api/admin/login":   true,
	"/api/auth/register": true,
	"/api/auth/login":    true,
	"/api/health":        true,
}

func PrincipalFromContext(ctx context.Context) (AuthPrincipal, bool) {
	principal, ok := ctx.Value(authPrincipalContextKey{}).(AuthPrincipal)
	return principal, ok
}

func RequireAuth(service *AuthService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || !isProtectedPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		principal, err := service.Authenticate(r.Context(), bearerTokenFromRequest(r))
		if err != nil {
			WriteAuthError(w, err)
			return
		}

		ctx := context.WithValue(r.Context(), authPrincipalContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isProtectedPath(path string) bool {
	if publicPaths[path] {
		return false
	}
	if path == "/api/admin" || strings.HasPrefix(path, "/api/admin/") {
		return false
	}
	return path == "/api" || strings.HasPrefix(path, "/api/") ||
		path == "/uploads" || strings.HasPrefix(path, "/uploads/") ||
		path == "/ws" || strings.HasPrefix(path, "/ws/")
}

func WriteAuthError(w http.ResponseWriter, err error) {
	var authErr *AuthError
	if errors.As(err, &authErr) {
		WriteJSONStatus(w, authErr.Status, map[string]string{
			"code":    authErr.Code,
			"message": authErr.Message,
		})
		return
	}

	WriteJSONStatus(w, http.StatusInternalServerError, map[string]string{
		"code":    "INTERNAL",
		"message": "服务器开小差了",
	})
}

func bearerTokenFromRequest(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) < len("Bearer ") || !strings.EqualFold(value[:len("Bearer")], "Bearer") || value[len("Bearer")] != ' ' {
		return ""
	}
	return strings.TrimSpace(value[len("Bearer "):])
}
