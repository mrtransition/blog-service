package main

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
)

// isLoopbackBind 判断监听地址是否只面向本机。默认（空值）按 127.0.0.1 处理。
func isLoopbackBind(bind string) bool {
	bind = strings.TrimSpace(bind)
	if bind == "" || strings.EqualFold(bind, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(bind, "[]"))
	return ip != nil && ip.IsLoopback()
}

// reloadAllowed 允许两类调用方：本机请求，或携带共享密钥的外部流水线。
// 未配置密钥时，非本机请求一律拒绝（满足 P3-1）。
func (a *App) reloadAllowed(r *http.Request) bool {
	if isLoopbackRequest(r) {
		return true
	}
	if a.cfg.ReloadToken == "" {
		return false
	}
	return constantTimeEqual(reloadTokenFromRequest(r), a.cfg.ReloadToken)
}

func reloadTokenFromRequest(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return strings.TrimSpace(r.Header.Get("X-Reload-Token"))
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// authMiddleware 仅在绑定非回环地址且配置了用户名/密码时启用。
// /reload 走独立的回环/共享密钥校验，避免流水线同时维护两套凭据。
func (a *App) authMiddleware(next http.Handler) http.Handler {
	if isLoopbackBind(a.cfg.Bind) || a.cfg.AuthUser == "" || a.cfg.AuthPass == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reload" {
			next.ServeHTTP(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		if !ok || !constantTimeEqual(user, a.cfg.AuthUser) || !constantTimeEqual(pass, a.cfg.AuthPass) {
			w.Header().Set("WWW-Authenticate", `Basic realm="blog", charset="UTF-8"`)
			http.Error(w, "需要认证", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
