package tcmock

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"
)

func WriteAsJSON(t *testing.T, w http.ResponseWriter, resp interface{}) {
	bytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		log.Printf("%v", err)
		t.Fatalf("Strange - I can't convert %#v to json: %v", resp, err)
	}
	_, err = w.Write(bytes)
	if err != nil {
		log.Printf("%v", err)
		t.Logf("Response: %v", string(bytes))
		t.Fatalf("Error writing response: %v", err)
	}
}

func BadRequest(w http.ResponseWriter, resp ...interface{}) {
	w.WriteHeader(400)
	fmt.Fprintf(w, "Bad request: %v", resp...)
}

func InvalidMethod(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(400)
	fmt.Fprintf(w, "HTTP method %v not supported for request path %v", req.Method, req.URL.EscapedPath())
}

func NotImplemented(w http.ResponseWriter, req *http.Request, api string) {
	w.WriteHeader(501)
	fmt.Fprintf(w, "API method %v not yet implemented for HTTP method %v and request path %v", api, req.Method, req.URL.EscapedPath())
}

func PathSuffix(t *testing.T, req *http.Request, prefix string) string {
	if !strings.HasPrefix(req.URL.RawPath, prefix) {
		t.Fatalf("BUG - URL path %v does not have prefix %v", req.URL.RawPath, prefix)
	}
	return req.URL.RawPath[len(prefix):]
}
