package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taskcluster/taskcluster/v30/workers/generic-worker/gwconfig"
	"github.com/taskcluster/taskcluster/v30/workers/generic-worker/tcmock"
)

type MockAWSProvisionedEnvironment struct {
	PublicFiles                      []map[string]string
	PrivateFiles                     []map[string]string
	PublicConfig                     map[string]interface{}
	PrivateConfig                    map[string]interface{}
	WorkerTypeSecretFunc             func(secret string, t *testing.T, w http.ResponseWriter, req *http.Request)
	WorkerTypeDefinitionUserDataFunc func(t *testing.T) interface{}
	Terminating                      bool
	PretendMetadata                  string
	OldDeploymentID                  string
	NewDeploymentID                  string
}

func (m *MockAWSProvisionedEnvironment) ValidPublicConfig(t *testing.T) map[string]interface{} {
	result := map[string]interface{}{
		// Need common caches directory across tests, since files
		// directory-caches.json and file-caches.json are not per-test.
		"cachesDir":       filepath.Join(cwd, "caches"),
		"cleanUpTaskDirs": false,
		"deploymentId":    m.OldDeploymentID,
		"disableReboots":  true,
		// Need common downloads directory across tests, since files
		// directory-caches.json and file-caches.json are not per-test.
		"downloadsDir":       filepath.Join(cwd, "downloads"),
		"idleTimeoutSecs":    60,
		"numberOfTasksToRun": 1,
		// should be enough for tests, and travis-ci.org CI environments
		// don't have a lot of free disk
		"requiredDiskSpaceMegabytes":     16,
		"rootURL":                        "https://fake.aws.prov.root.url",
		"sentryProject":                  "generic-worker-tests",
		"shutdownMachineOnIdle":          false,
		"shutdownMachineOnInternalError": false,
		"ed25519SigningKeyLocation":      filepath.Join(testdataDir, "ed25519_private_key"),
		"tasksDir":                       filepath.Join(testdataDir, t.Name()),
		"workerTypeMetadata": map[string]interface{}{
			"machine-setup": map[string]string{
				"pretend-metadata": m.PretendMetadata,
			},
		},
	}
	EngineTestSettings(result)
	return result
}

func (m *MockAWSProvisionedEnvironment) ValidPrivateConfig(t *testing.T) map[string]interface{} {
	return map[string]interface{}{}
}

func WriteJSON(t *testing.T, w http.ResponseWriter, resp interface{}) {
	bytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		t.Fatalf("Strange - I can't convert %#v to json: %v", resp, err)
	}
	_, _ = w.Write(bytes)
}

func (m *MockAWSProvisionedEnvironment) userData(t *testing.T, w http.ResponseWriter, workerType string) {
	var data interface{}
	if m.WorkerTypeDefinitionUserDataFunc != nil {
		data = m.WorkerTypeDefinitionUserDataFunc(t)
	} else {
		data = m.WorkerTypeDefinitionUserData(t)
	}
	resp := map[string]interface{}{
		"workerPoolId": "test-provisioner/" + workerType,
		"providerId":   "test-provider",
		"workerGroup":  "test-worker-group",
		"rootUrl":      "http://localhost:13243",
		"workerConfig": data,
	}
	WriteJSON(t, w, resp)
}

func (m *MockAWSProvisionedEnvironment) WorkerTypeDefinitionUserData(t *testing.T) map[string]map[string]interface{} {

	workerTypeDefinitionUserData := map[string]map[string]interface{}{
		"genericWorker": map[string]interface{}{},
	}
	if m.PublicConfig != nil {
		workerTypeDefinitionUserData["genericWorker"]["config"] = m.PublicConfig
	} else {
		workerTypeDefinitionUserData["genericWorker"]["config"] = m.ValidPublicConfig(t)
	}
	if m.PublicFiles != nil {
		workerTypeDefinitionUserData["genericWorker"]["files"] = m.PublicFiles
	}
	return workerTypeDefinitionUserData
}

func (m *MockAWSProvisionedEnvironment) PrivateHostSetup(t *testing.T) interface{} {

	privateHostSetup := map[string]interface{}{}

	if m.PrivateConfig != nil {
		privateHostSetup["config"] = m.PrivateConfig
	} else {
		privateHostSetup["config"] = m.ValidPrivateConfig(t)
	}
	if m.PrivateFiles != nil {
		privateHostSetup["files"] = m.PrivateFiles
	}
	return privateHostSetup
}

func (m *MockAWSProvisionedEnvironment) Setup(t *testing.T) (teardown func(), err error) {
	td := setupEnvironment(t)
	workerType := testWorkerType()
	configureForAWS = true
	oldEC2MetadataBaseURL := EC2MetadataBaseURL
	EC2MetadataBaseURL = "http://localhost:13243/latest"

	// we need to use a non-default port for the livelog internalGETPort, so
	// that we don't conflict with a generic-worker in which the tests are
	// running
	oldInternalPUTPort := internalPUTPort
	internalPUTPort = 30584
	oldInternalGETPort := internalGETPort
	internalGETPort = 30583

	// Create custom *http.ServeMux rather than using http.DefaultServeMux, so
	// registered handler functions won't interfere with future tests that also
	// use http.DefaultServeMux.
	mocksHandler := http.NewServeMux()
	secrets := tcmock.NewSecrets()
	secrets.GetFunc = func(secret string, t *testing.T, w http.ResponseWriter, req *http.Request) {
		if m.WorkerTypeSecretFunc != nil {
			m.WorkerTypeSecretFunc(secret, t, w, req)
			return
		}
		resp := map[string]interface{}{
			"secret":  m.PrivateHostSetup(t),
			"expires": "2077-08-19T00:00:00.000Z",
		}
		WriteJSON(t, w, resp)
	}
	secrets.Handle(mocksHandler, t)
	wm := &tcmock.WorkerManager{
		NewDeploymentID: m.NewDeploymentID,
	}
	wm.Handle(mocksHandler, t)

	mocksHandler.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.EscapedPath() {

		// simulate AWS endpoints
		case "/latest/dynamic/instance-identity/document":
			resp := map[string]interface{}{
				"availabilityZone": "outer-space",
				"privateIp":        "87.65.43.21",
				"version":          "2017-09-30",
				"instanceId":       "test-worker-id",
				"instanceType":     "p3.teenyweeny",
				"accountId":        "123456789012",
				"imageId":          "test-ami",
				"pendingTime":      "2016-11-19T16:32:11Z",
				"architecture":     "x86_64",
				"region":           "quadrant-4",
			}
			WriteJSON(t, w, resp)
		case "/latest/dynamic/instance-identity/signature":
			fmt.Fprint(w, "test-signature")
		case "/latest/meta-data/ami-id":
			fmt.Fprint(w, "test-ami")
		case "/latest/meta-data/spot/termination-time":
			if m.Terminating {
				fmt.Fprint(w, "time to die")
			} else {
				w.WriteHeader(404)
			}
		case "/latest/meta-data/placement/availability-zone":
			fmt.Fprint(w, "outer-space")
		case "/latest/meta-data/instance-type":
			fmt.Fprint(w, "p3.teenyweeny")
		case "/latest/meta-data/instance-id":
			fmt.Fprint(w, "test-instance-id")
		case "/latest/meta-data/public-hostname":
			fmt.Fprint(w, "MadamaButterfly")
		case "/latest/meta-data/local-ipv4":
			fmt.Fprint(w, "87.65.43.21")
		case "/latest/meta-data/public-ipv4":
			fmt.Fprint(w, "12.34.56.78")
		case "/latest/user-data":
			m.userData(t, w, workerType)
		default:
			w.WriteHeader(400)
			fmt.Fprintf(w, "Cannot serve URL %v", req.URL)
		}
	})

	s := &http.Server{
		Addr:           "localhost:13243",
		Handler:        mocksHandler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	go func() {
		_ = s.ListenAndServe()
	}()
	configFile := &gwconfig.File{
		Path: filepath.Join(testdataDir, t.Name(), "generic-worker.config"),
	}
	configProvider, err = loadConfig(configFile, AWS_PROVIDER)
	return func() {
		td()
		err := s.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("Error shutting down http server: %v", err)
		}
		t.Log("HTTP server for mock Provisioner and EC2 metadata endpoints stopped")
		EC2MetadataBaseURL = oldEC2MetadataBaseURL
		configureForAWS = false
		internalPUTPort = oldInternalPUTPort
		internalGETPort = oldInternalGETPort
	}, err
}

func (m *MockAWSProvisionedEnvironment) ExpectError(t *testing.T, errorText string, err error) {
	if err == nil || !strings.Contains(err.Error(), errorText) {
		t.Fatalf("Was expecting error to include %q but got: %v", errorText, err)
	}
}

func (m *MockAWSProvisionedEnvironment) ExpectNoError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("Was expecting no error but got: %v", err)
	}
}
