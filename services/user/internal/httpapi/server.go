package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"kitdesign/services/user/internal/account"
	"kitdesign/services/user/internal/authorization"
	"kitdesign/services/user/internal/config"
	"kitdesign/services/user/internal/login"
	"kitdesign/services/user/internal/recovery"
	"kitdesign/services/user/internal/register"
	"kitdesign/services/user/internal/session"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Server struct {
	config        config.Config
	logger        *slog.Logger
	pinger        Pinger
	register      *register.Service
	login         *login.Service
	sessions      *session.Service
	account       *account.Service
	recovery      *recovery.Service
	authorization *authorization.Service
	tokens        *session.TokenManager
	limiter       *Limiter
}

func New(
	cfg config.Config,
	logger *slog.Logger,
	pinger Pinger,
	registerService *register.Service,
	loginService *login.Service,
	sessionService *session.Service,
	accountService *account.Service,
	recoveryService *recovery.Service,
	authorizationService *authorization.Service,
	tokens *session.TokenManager,
) *Server {
	return &Server{
		config:        cfg,
		logger:        logger,
		pinger:        pinger,
		register:      registerService,
		login:         loginService,
		sessions:      sessionService,
		account:       accountService,
		recovery:      recoveryService,
		authorization: authorizationService,
		tokens:        tokens,
		limiter:       newLimiter(cfg.RateLimit),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /readyz", s.handleReady)

	mux.Handle("POST /v1/auth/register", s.rateLimit(rate.Every(12*time.Second), 5, http.HandlerFunc(s.handleRegister)))
	mux.Handle("POST /v1/auth/email/verify", s.rateLimit(rate.Every(3*time.Second), 10, http.HandlerFunc(s.handleVerifyEmail)))
	mux.Handle("POST /v1/auth/email/resend", s.rateLimit(rate.Every(12*time.Second), 5, http.HandlerFunc(s.handleResendVerification)))
	mux.Handle("POST /v1/auth/login", s.rateLimit(rate.Every(6*time.Second), 10, http.HandlerFunc(s.handleLogin)))
	mux.Handle("POST /v1/auth/refresh", s.rateLimit(rate.Every(time.Second), 30, http.HandlerFunc(s.handleRefresh)))
	mux.Handle("POST /v1/auth/logout", s.requireAuth(http.HandlerFunc(s.handleLogout)))
	mux.Handle("POST /v1/auth/password/forgot", s.rateLimit(rate.Every(12*time.Second), 5, http.HandlerFunc(s.handleForgotPassword)))
	mux.Handle("POST /v1/auth/password/reset", s.rateLimit(rate.Every(6*time.Second), 5, http.HandlerFunc(s.handleResetPassword)))

	mux.Handle("GET /v1/account", s.requirePermission(permissionAccountRead, http.HandlerFunc(s.handleGetAccount)))
	mux.Handle("PATCH /v1/account", s.requirePermission(permissionAccountUpdate, http.HandlerFunc(s.handleUpdateAccount)))
	mux.Handle("GET /v1/account/sessions", s.requirePermission(permissionSessionsRead, http.HandlerFunc(s.handleListSessions)))
	mux.Handle("DELETE /v1/account/sessions/{id}", s.requirePermission(permissionSessionsRevoke, http.HandlerFunc(s.handleRevokeSession)))
	mux.Handle("GET /v1/account/events", s.requirePermission(permissionAccountRead, http.HandlerFunc(s.handleAccountEvents)))

	mux.Handle("GET /v1/admin/users", s.requirePermission(permissionUsersRead, http.HandlerFunc(s.handleAdminListUsers)))
	mux.Handle("PATCH /v1/admin/users/{id}/status", s.requirePermission(permissionUsersManage, http.HandlerFunc(s.handleAdminSetStatus)))
	mux.Handle("POST /v1/admin/users/{id}/roles", s.requirePermission(permissionRolesManage, http.HandlerFunc(s.handleAdminGrantRole)))
	mux.Handle("DELETE /v1/admin/users/{id}/roles/{role}", s.requirePermission(permissionRolesManage, http.HandlerFunc(s.handleAdminRevokeRole)))
	mux.Handle("GET /v1/admin/roles", s.requirePermission(permissionRolesManage, http.HandlerFunc(s.handleAdminListRoles)))

	mux.HandleFunc("POST /internal/v1/authz/check", s.handleInternalAuthz)

	handler := s.withRequestID(
		s.recoverer(
			s.logRequests(
				s.cors(
					s.securityHeaders(
						s.limitBody(mux),
					),
				),
			),
		),
	)
	return handler
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.pinger.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
