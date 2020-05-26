package tcmock

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/taskcluster/taskcluster/v30/clients/client-go/tcsecrets"
)

type Secrets struct {
	// map from secret name to secret value
	Secrets    map[string]*tcsecrets.Secret
	PingFunc   func(t *testing.T, w http.ResponseWriter, req *http.Request)
	ListFunc   func(t *testing.T, w http.ResponseWriter, req *http.Request)
	GetFunc    func(secret string, t *testing.T, w http.ResponseWriter, req *http.Request)
	SetFunc    func(secret string, t *testing.T, w http.ResponseWriter, req *http.Request)
	RemoveFunc func(secret string, t *testing.T, w http.ResponseWriter, req *http.Request)
}

func NewSecrets() *Secrets {
	return &Secrets{
		Secrets: map[string]*tcsecrets.Secret{},
		GetFunc: nil,
	}
}

func (s *Secrets) Handle(handler *http.ServeMux, t *testing.T) {

	const (
		PingPath    = "/api/secrets/v1/ping"
		SecretsPath = "/api/secrets/v1/secrets"
		SecretPath  = "/api/secrets/v1/secret/"
	)

	handler.HandleFunc(PingPath, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case "GET":
			s.Ping(t, w, req)
		default:
			w.WriteHeader(400)
		}
	})

	handler.HandleFunc(SecretsPath, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case "GET":
			s.List(t, w, req)
		default:
			w.WriteHeader(400)
		}
	})

	handler.HandleFunc(SecretPath, func(w http.ResponseWriter, req *http.Request) {
		secret := req.URL.EscapedPath()[len(SecretPath):]
		switch req.Method {
		case "GET":
			s.Get(secret, t, w, req)
		case "PUT":
			s.Set(secret, t, w, req)
		case "DELETE":
			s.Remove(secret, t, w, req)
		default:
			w.WriteHeader(400)
		}
	})
}

func (s *Secrets) Ping(t *testing.T, w http.ResponseWriter, req *http.Request) {
	if s.PingFunc != nil {
		s.PingFunc(t, w, req)
		return
	}
	w.WriteHeader(200)
}

func (s *Secrets) List(t *testing.T, w http.ResponseWriter, req *http.Request) {
	if s.ListFunc != nil {
		s.ListFunc(t, w, req)
		return
	}
	names := []string{}
	for name, _ := range s.Secrets {
		names = append(names, name)
	}
	list := &tcsecrets.SecretsList{
		Secrets: names,
	}
	WriteAsJSON(t, w, list)
}

func (s *Secrets) Get(secret string, t *testing.T, w http.ResponseWriter, req *http.Request) {
	if s.GetFunc != nil {
		s.GetFunc(secret, t, w, req)
		return
	}
	if sec, exists := s.Secrets[secret]; exists {
		WriteAsJSON(t, w, sec)
		return
	}
	w.WriteHeader(404)
}

func (s *Secrets) Set(secret string, t *testing.T, w http.ResponseWriter, req *http.Request) {
	if s.SetFunc != nil {
		s.SetFunc(secret, t, w, req)
		return
	}
	var sec tcsecrets.Secret
	dec := json.NewDecoder(req.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&sec)
	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte(err.Error()))
		return
	}
	s.Secrets[secret] = &sec
	w.WriteHeader(200)
}

func (s *Secrets) Remove(secret string, t *testing.T, w http.ResponseWriter, req *http.Request) {
	if s.RemoveFunc != nil {
		s.RemoveFunc(secret, t, w, req)
		return
	}
	if _, exists := s.Secrets[secret]; exists {
		delete(s.Secrets, secret)
		w.WriteHeader(200)
		return
	}
	w.WriteHeader(404)
}
