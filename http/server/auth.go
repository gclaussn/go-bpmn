package server

import (
	"context"
	"log"
	"net/http"

	"github.com/gclaussn/go-bpmn/engine/pg"
)

// auth is used as context value key by the authHandler.
type auth struct{}

type authHandler struct {
	apiKeyManager pg.ApiKeyManager
	handler       http.Handler
}

func (h *authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI == PathReadiness {
		h.handler.ServeHTTP(w, r)
		return
	}

	authorization := r.Header.Get(HeaderAuthorization)

	apiKey, err := h.apiKeyManager.GetApiKey(r.Context(), authorization)
	if err != nil {
		log.Printf("%s %s: authentication failed for %s: %v", r.Method, r.RequestURI, r.RemoteAddr, err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	ctx := context.WithValue(r.Context(), auth{}, apiKey)
	h.handler.ServeHTTP(w, r.WithContext(ctx))
}

type basicAuthHandler struct {
	username string
	password string
	handler  http.Handler
}

func (h *basicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI == PathReadiness {
		h.handler.ServeHTTP(w, r)
		return
	}

	username, password, ok := r.BasicAuth()
	if !ok || username != h.username || password != h.password {
		log.Printf("%s %s: authentication failed for %s", r.Method, r.RequestURI, r.RemoteAddr)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	h.handler.ServeHTTP(w, r)
}
