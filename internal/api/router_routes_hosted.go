package api

import "time"

func (r *Router) registerHostedRoutes(hostedSignupHandlers *HostedSignupHandlers) {
	if hostedSignupHandlers == nil {
		return
	}
	if r.signupRateLimiter == nil {
		r.signupRateLimiter = NewRateLimiter(5, 1*time.Hour)
	}

	r.mux.HandleFunc("POST /api/public/signup", r.signupRateLimiter.Middleware(hostedSignupHandlers.HandlePublicSignup))
}
