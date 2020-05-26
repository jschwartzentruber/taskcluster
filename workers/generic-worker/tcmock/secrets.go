package tcmock

import (
	"net/http"
	"testing"

	"github.com/taskcluster/taskcluster/v30/clients/client-go/tcsecrets"
)

type Secrets struct {
	// map from secret name to secret value
	Secrets map[string]tcsecrets.Secret
	GetFunc func(t *testing.T, w http.ResponseWriter, name string)
}

func NewSecrets() *Secrets {
	return &Secrets{
		Secrets: map[string]tcsecrets.Secret{},
		GetFunc: nil,
	}
}

func (s *Secrets) Handle(handler *http.ServeMux, t *testing.T) {

	const (
		PathPing    = "/api/secrets/v1/ping"
		PathSecrets = "/api/secrets/v1/secrets"
		PathSecret  = "/api/secrets/v1/secret/"
	)

	handler.HandleFunc(PathPing, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case "GET":
			s.Ping(t, w)
		default:
			w.WriteHeader(400)
		}
	})

	handler.HandleFunc(PathSecrets, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case "GET":
			s.List(t, w)
		default:
			w.WriteHeader(400)
		}
	})

	handler.HandleFunc(PathSecret, func(w http.ResponseWriter, req *http.Request) {
		secretName := req.URL.EscapedPath()[len(PathSecret):]
		switch req.Method {
		case "GET":
			s.Get(t, w, secretName)
		case "PUT":
			s.Set(t, w, secretName, req)
		case "DELETE":
			s.Remove(t, w, secretName)
		default:
			w.WriteHeader(400)
		}
	})
}

func (s *Secrets) Ping(t *testing.T, w http.ResponseWriter) {
	w.WriteHeader(200)
}

func (s *Secrets) List(t *testing.T, w http.ResponseWriter) {
	names := []string{}
	for name, _ := range s.Secrets {
		names = append(names, name)
	}
	list := &tcsecrets.SecretsList{
		Secrets: names,
	}
	WriteAsJSON(t, w, list)
}

func (s *Secrets) Get(t *testing.T, w http.ResponseWriter, secret string) {
	if s.GetFunc != nil {
		s.GetFunc(t, w, secret)
		return
	}
	if sec, exists := s.Secrets[secret]; exists {
		WriteAsJSON(t, w, sec)
		return
	}
	w.WriteHeader(404)
}

func (s *Secrets) Set(t *testing.T, w http.ResponseWriter, secret string, req *http.Request) {
	// TODO
}

func (s *Secrets) Remove(t *testing.T, w http.ResponseWriter, secret string) {
	if _, exists := s.Secrets[secret]; exists {
		delete(s.Secrets, secret)
		w.WriteHeader(200)
		return
	}
	w.WriteHeader(404)
}
