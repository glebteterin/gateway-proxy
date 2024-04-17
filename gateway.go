// Package gateway provides a simple reverse proxy server that can switch between two backends based on the response of the first one.
package gateway

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/go-pckg/pine"
)

type Server struct {
	logger     *pine.Logger
	lock       sync.Mutex
	httpServer *http.Server

	aTarget *url.URL
	bTarget *url.URL

	fallbackProxy *httputil.ReverseProxy
	proxyA        *httputil.ReverseProxy
	proxyB        *httputil.ReverseProxy

	routes map[string]*url.URL
}

func NewServer(aURL, bURL string, logger *pine.Logger) (*Server, error) {
	remoteA, err := url.Parse(aURL)
	if err != nil {
		return nil, fmt.Errorf("invalid a url: %w", err)
	}

	remoteB, err := url.Parse(bURL)
	if err != nil {
		return nil, fmt.Errorf("invalid b url: %w", err)
	}

	server := &Server{aTarget: remoteA, bTarget: remoteB, routes: map[string]*url.URL{}, logger: logger}

	server.proxyA = server.setupProxyA()
	server.proxyB = server.setupProxyB()
	server.fallbackProxy = server.setupFallbackProxy()

	return server, nil
}

func (s *Server) Run(port string) {
	s.lock.Lock()
	router := http.NewServeMux()
	router.HandleFunc("/", s.mainHandler)
	s.httpServer = s.makeHTTPServer(port, router)
	s.lock.Unlock()

	err := s.httpServer.ListenAndServe()
	s.logger.Warn("HTTP Server terminated", pine.Err(err))
}

func (s *Server) Shutdown() {
	s.logger.Warn("Shutdown rest server")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s.lock.Lock()
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.Warn("HTTP Shutdown error", pine.Err(err))
		}
		s.logger.Debugf("Shutdown http server completed")
	}

	s.lock.Unlock()
}

func (s *Server) makeHTTPServer(port string, router http.Handler) *http.Server {
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	return httpServer
}

func (s *Server) mainHandler(w http.ResponseWriter, r *http.Request) {
	route := s.routes[routeKey(r)]
	if route == nil {
		// cached route not found, buffering body for possible fallback redirect

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("error reading body: %s", err), 500)
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(body))
		r.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewBuffer(body)), nil
		}

		start := time.Now()
		s.fallbackProxy.ServeHTTP(w, r)
		s.logger.Debugf("[%v %v] served with fallback in %vms", r.Method, r.URL.Path, time.Since(start).Milliseconds())
	} else {
		if *route == *s.bTarget {
			start := time.Now()
			s.proxyB.ServeHTTP(w, r)
			s.logger.Debugf("[%v %v] served from B (cached) in %vms", r.Method, r.URL.Path, time.Since(start).Milliseconds())
		} else {
			start := time.Now()
			s.proxyA.ServeHTTP(w, r)
			s.logger.Debugf("[%v %v] served from A (cached) in %vms", r.Method, r.URL.Path, time.Since(start).Milliseconds())
		}
	}
}

func (s *Server) setupProxyB() *httputil.ReverseProxy {
	return httputil.NewSingleHostReverseProxy(s.bTarget)
}

func (s *Server) setupProxyA() *httputil.ReverseProxy {
	return httputil.NewSingleHostReverseProxy(s.aTarget)
}

func (s *Server) setupFallbackProxy() *httputil.ReverseProxy {
	fbp := httputil.NewSingleHostReverseProxy(s.aTarget)
	fbp.ModifyResponse = func(response *http.Response) error {
		if response.StatusCode == 404 && response.Header.Get(HeaderNoRoute) != "" {
			// cache the fallback B route
			s.routes[routeKey(response.Request)] = s.bTarget
			return fmt.Errorf("not found")
		}

		// cache the successful A route
		s.routes[routeKey(response.Request)] = s.aTarget

		return nil
	}
	fbp.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		// fallback to B

		// unwind body
		if req.GetBody != nil {
			b, err := req.GetBody()
			if err != nil {
				http.Error(rw, fmt.Sprintf("error re-reading body: %s", err), 500)
				return
			}
			req.Body = b
		}

		start := time.Now()
		s.proxyB.ServeHTTP(rw, req)
		s.logger.Debugf("[%v %v] served from B fallback in %vms", req.Method, req.URL.Path, time.Since(start).Milliseconds())
	}
	return fbp
}

func routeKey(r *http.Request) string {
	return fmt.Sprintf("%v %v", r.Method, r.URL.Path)
}
