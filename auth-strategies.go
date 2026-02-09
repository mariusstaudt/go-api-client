package api

import "net/http"

type AuthStrategy func(req *http.Request, token string)

var (
	BearerStrategy AuthStrategy = func(r *http.Request, t string) { r.Header.Set("Authorization", "Bearer "+t) }
	BasicStrategy  AuthStrategy = func(r *http.Request, t string) { r.Header.Set("Authorization", "Basic "+t) }
	GitLabStrategy AuthStrategy = func(r *http.Request, t string) { r.Header.Set("PRIVATE-TOKEN", t) }
	NoAuthStrategy AuthStrategy = func(r *http.Request, t string) {}
)
