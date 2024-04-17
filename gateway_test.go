package gateway

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-pckg/pine"
)

func newRequest(t *testing.T, method, url string, body io.Reader) *http.Request {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

type expectedResponse struct {
	Code int
	Body string
}

func TestNewServer(t *testing.T) {
	const aBackendResponse = "I am the A backend"
	const bBackendResponse = "I am the B backend"

	logger := pine.New(pine.WithLevel(pine.DisabledLevel))

	hits := []string{}

	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, "a")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		switch r.URL.Path {
		case "/exists-in-a":
			w.WriteHeader(http.StatusOK)
		case "/exists-in-a-but-no-data":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.Header().Set(HeaderNoRoute, "a")
			w.WriteHeader(http.StatusNotFound)
		}

		io.WriteString(w, aBackendResponse+", you sent "+string(body))
	}))
	defer backendA.Close()

	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, "b")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		switch r.URL.Path {
		case "/exists-in-b":
			w.WriteHeader(http.StatusOK)
		case "/exists-in-b-but-no-data":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.Header().Set(HeaderNoRoute, "b")
			w.WriteHeader(http.StatusNotFound)
		}

		io.WriteString(w, bBackendResponse+", you sent "+string(body))
	}))
	defer backendB.Close()

	server, err := NewServer(backendA.URL, backendB.URL, logger)
	if err != nil {
		t.Fatal(err, "error creating server")
	}

	go func() {
		server.Run("80")
	}()

	time.Sleep(time.Millisecond)

	const request = "hello"

	tests := []struct {
		name string
		req  []*http.Request
		res  []*expectedResponse
		hits string
	}{
		{
			name: "first request served from a",
			req: []*http.Request{
				newRequest(t, "POST", "http://localhost:80/exists-in-a", bytes.NewBuffer([]byte(request))),
			},
			hits: "a",
			res: []*expectedResponse{
				{Code: 200, Body: aBackendResponse + ", you sent hello"},
			},
		},
		{
			name: "first request served from b",
			req: []*http.Request{
				newRequest(t, "GET", "http://localhost:80/exists-in-b", bytes.NewBuffer([]byte(request))),
			},
			hits: "a, b",
			res: []*expectedResponse{
				{Code: 200, Body: bBackendResponse + ", you sent hello"},
			},
		},
		{
			name: "two requests served from b",
			req: []*http.Request{
				newRequest(t, "POST", "http://localhost:80/exists-in-b", bytes.NewBuffer([]byte(request))),
				newRequest(t, "POST", "http://localhost:80/exists-in-b", bytes.NewBuffer([]byte(request))),
			},
			hits: "a, b, b",
			res: []*expectedResponse{
				{Code: 200, Body: bBackendResponse + ", you sent hello"},
				{Code: 200, Body: bBackendResponse + ", you sent hello"},
			},
		},
		{
			name: "two requests served from a",
			req: []*http.Request{
				newRequest(t, "POST", "http://localhost:80/exists-in-a", bytes.NewBuffer([]byte(request))),
				newRequest(t, "POST", "http://localhost:80/exists-in-a", bytes.NewBuffer([]byte(request))),
			},
			hits: "a, a",
			res: []*expectedResponse{
				{Code: 200, Body: aBackendResponse + ", you sent hello"},
				{Code: 200, Body: aBackendResponse + ", you sent hello"},
			},
		},
		{
			name: "two no data requests served from a",
			req: []*http.Request{
				newRequest(t, "POST", "http://localhost:80/exists-in-a-but-no-data", bytes.NewBuffer([]byte(request))),
				newRequest(t, "POST", "http://localhost:80/exists-in-a-but-no-data", bytes.NewBuffer([]byte(request))),
			},
			hits: "a, a",
			res: []*expectedResponse{
				{Code: 404, Body: aBackendResponse + ", you sent hello"},
				{Code: 404, Body: aBackendResponse + ", you sent hello"},
			},
		},
		{
			name: "two no data requests served from b",
			req: []*http.Request{
				newRequest(t, "POST", "http://localhost:80/exists-in-b-but-no-data", bytes.NewBuffer([]byte(request))),
				newRequest(t, "POST", "http://localhost:80/exists-in-b-but-no-data", bytes.NewBuffer([]byte(request))),
			},
			hits: "a, b, b",
			res: []*expectedResponse{
				{Code: 404, Body: bBackendResponse + ", you sent hello"},
				{Code: 404, Body: bBackendResponse + ", you sent hello"},
			},
		},
		{
			name: "missing path",
			req: []*http.Request{
				newRequest(t, "POST", "http://localhost:80/blah", bytes.NewBuffer([]byte(request))),
				newRequest(t, "POST", "http://localhost:80/blah", bytes.NewBuffer([]byte(request))),
			},
			hits: "a, b, b",
			res: []*expectedResponse{
				{Code: 404, Body: bBackendResponse + ", you sent hello"},
				{Code: 404, Body: bBackendResponse + ", you sent hello"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hits = []string{}
			server.routes = map[string]*url.URL{}

			for i, r := range tt.req {
				res, err := http.DefaultClient.Do(r)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				resBody, err := io.ReadAll(res.Body)
				if err != nil {
					t.Fatal(err)
				}

				wantRes := tt.res[i]
				if wantRes.Code != res.StatusCode {
					t.Errorf("response %v status code is %v, expected %v", i, res.StatusCode, wantRes.Code)
				}
				if wantRes.Body != string(resBody) {
					t.Errorf("response %v body is '%v', expected '%v'", i, string(resBody), wantRes.Body)
				}
			}

			gotHits := strings.Join(hits, ", ")
			if gotHits != tt.hits {
				t.Errorf("hits order is '%v', expected '%v'", gotHits, tt.hits)
			}
		})
	}
}
