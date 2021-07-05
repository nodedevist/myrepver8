package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/OpenZeppelin/disco/config"
	"github.com/OpenZeppelin/disco/proxy/services"
)

// ListenAndServe starts the proxy and listens to the port.
func ListenAndServe() error {
	distrUrl, err := url.Parse(fmt.Sprintf("http://localhost%s", config.DistributionConfig.HTTP.Addr))
	if err != nil {
		return err
	}

	rp := httputil.NewSingleHostReverseProxy(distrUrl)

	return (&http.Server{
		Addr:         fmt.Sprintf(":%d", config.Vars.DiscoPort),
		Handler:      NewHandler(rp, services.NewDiscoService()),
		ReadTimeout:  time.Second * 30,
		WriteTimeout: time.Second * 30,
		IdleTimeout:  time.Second * 30,
	}).ListenAndServe()
}

// NewHandler creates a new handler which consumes Disco service.
func NewHandler(rp *httputil.ReverseProxy, disco *services.Disco) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if done := preHandle(rw, r, disco); done {
			return
		}
		rp.ServeHTTP(rw, r)
		postHandle(rw, r, disco)
	})
}

func preHandle(rw http.ResponseWriter, r *http.Request, disco *services.Disco) bool {
	// Disallow overwriting to CID v1 and digest repos.
	if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/manifests/latest") {
		repoName := strings.Split(r.URL.Path[1:], "/")[1]
		if disco.IsOnlyPullable(repoName) {
			rw.WriteHeader(401)
			return true
		}
	}

	if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/manifests/latest") {
		repoName := strings.Split(r.URL.Path[1:], "/")[1]
		if err := disco.CloneGlobalRepo(r.Context(), repoName); err != nil {
			log.Printf("failed to clone global repo: %v", err)
			// TODO: Handle 404
			rw.WriteHeader(500)
			return true
		}
	}
	return false
}

func postHandle(rw http.ResponseWriter, r *http.Request, disco *services.Disco) {
	if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/manifests/latest") {
		repoName := strings.Split(r.URL.Path[1:], "/")[1]
		if err := disco.MakeGlobalRepo(r.Context(), repoName); err != nil {
			log.Printf("failed to make global repo: %v", err)
		}
	}
}
