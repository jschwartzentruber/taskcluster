package tcmock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster/v30/clients/client-go/tcworkermanager"
)

type WorkerManager struct {
	NewDeploymentID    string
	RegisterWorkerFunc func(t *testing.T, w http.ResponseWriter, req *http.Request)
	WorkerPoolFunc     func(workerPool string, t *testing.T, w http.ResponseWriter, req *http.Request)
}

func (wm *WorkerManager) Handle(handler *http.ServeMux, t *testing.T) {

	const (
		RegisterPath   = "/api/worker-manager/v1/worker/register"
		WorkerPoolPath = "/api/worker-manager/v1/worker-pool/"
	)

	handler.HandleFunc(RegisterPath, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case "POST":
			wm.RegisterWorker(t, w, req)
		default:
			InvalidMethod(w, req)
		}
	})

	handler.HandleFunc(WorkerPoolPath, func(w http.ResponseWriter, req *http.Request) {
		workerpool := PathSuffix(t, req, WorkerPoolPath)
		switch req.Method {
		case "PUT":
			NotImplemented(w, req, "CreateWorkerPool")
		case "POST":
			NotImplemented(w, req, "UpdateWorkerPool")
		case "DELETE":
			NotImplemented(w, req, "DeleteWorkerPool")
		case "GET":
			wm.WorkerPool(workerpool, t, w, req)
		default:
			InvalidMethod(w, req)
		}
	})
}

func (wm *WorkerManager) RegisterWorker(t *testing.T, w http.ResponseWriter, req *http.Request) {
	if wm.RegisterWorkerFunc != nil {
		wm.RegisterWorkerFunc(t, w, req)
		return
	}
	d := json.NewDecoder(req.Body)
	d.DisallowUnknownFields()
	b := tcworkermanager.RegisterWorkerRequest{}
	err := d.Decode(&b)
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "%v", err)
	}
	d = json.NewDecoder(bytes.NewBuffer(b.WorkerIdentityProof))
	d.DisallowUnknownFields()
	g := tcworkermanager.AwsProviderType{}
	err = d.Decode(&g)
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "%v", err)
	}
	if g.Signature != "test-signature" {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Got signature %q but was expecting %q", g.Signature, "test-signature")
	}
	resp := tcworkermanager.RegisterWorkerResponse{
		Credentials: tcworkermanager.Credentials{
			ClientID:    "fake-client-id",
			Certificate: "fake-cert",
			AccessToken: slugid.Nice(),
		},
	}
	WriteAsJSON(t, w, resp)
}

func (wm *WorkerManager) WorkerPool(workerPool string, t *testing.T, w http.ResponseWriter, req *http.Request) {
	if wm.WorkerPoolFunc != nil {
		wm.WorkerPoolFunc(workerPool, t, w, req)
		return
	}
	resp := tcworkermanager.WorkerPoolFullDefinition{
		Config: json.RawMessage(`{
			"launchConfigs": [
				{
					"workerConfig": {
						"genericWorker": {
							"config": {
								"deploymentId": "` + wm.NewDeploymentID + `"
							}
						}
					}
				}
			]
		}`),
	}
	log.Printf("%v", string(resp.Config))
	WriteAsJSON(t, w, resp)
}
