package tcmock

import (
	"testing"

	tcclient "github.com/taskcluster/taskcluster/v30/clients/client-go"
	"github.com/taskcluster/taskcluster/v30/clients/client-go/tcsecrets"
	"github.com/taskcluster/taskcluster/v30/clients/client-go/tcworkermanager"
	"github.com/taskcluster/taskcluster/v30/workers/generic-worker/tc"
)

type ServiceFactory struct {
	auth          tc.Auth
	queue         tc.Queue
	secrets       tc.Secrets
	purgeCache    tc.PurgeCache
	workerManager tc.WorkerManager
	artifacts     tc.Artifacts
}

func NewServiceFactory(t *testing.T) *ServiceFactory {
	return &ServiceFactory{
		auth:          NewAuth(t),
		queue:         NewQueue(t),
		secrets:       tcsecrets.New(nil, "http://localhost:13243"),
		purgeCache:    NewPurgeCache(t),
		workerManager: tcworkermanager.New(nil, "http://localhost:13243"),
		artifacts:     NewArtifacts(t),
	}
}

func (sf *ServiceFactory) Auth(creds *tcclient.Credentials, rootURL string) tc.Auth {
	return sf.auth
}

func (sf *ServiceFactory) Queue(creds *tcclient.Credentials, rootURL string) tc.Queue {
	return sf.queue
}

func (sf *ServiceFactory) Secrets(creds *tcclient.Credentials, rootURL string) tc.Secrets {
	return sf.secrets
}

func (sf *ServiceFactory) PurgeCache(creds *tcclient.Credentials, rootURL string) tc.PurgeCache {
	return sf.purgeCache
}

func (sf *ServiceFactory) WorkerManager(creds *tcclient.Credentials, rootURL string) tc.WorkerManager {
	return sf.workerManager
}

func (sf *ServiceFactory) Artifacts(creds *tcclient.Credentials, rootURL string) tc.Artifacts {
	return sf.artifacts
}
