package tcmock

import (
	"testing"

	tcclient "github.com/taskcluster/taskcluster/v30/clients/client-go"
	"github.com/taskcluster/taskcluster/v30/workers/generic-worker/tc"
)

type ServiceFactory struct {
	t               *testing.T
	TCAuth          tc.Auth
	TCQueue         tc.Queue
	TCSecrets       tc.Secrets
	TCPurgeCache    tc.PurgeCache
	TCWorkerManager tc.WorkerManager
	TCArtifacts     tc.Artifacts
}

func NewServiceFactory(t *testing.T) *ServiceFactory {
	return &ServiceFactory{
		TCAuth:          NewAuth(t),
		TCQueue:         NewQueue(t),
		TCSecrets:       NewSecrets(t),
		TCPurgeCache:    NewPurgeCache(t),
		TCWorkerManager: NewWorkerManager(t),
		TCArtifacts:     NewArtifacts(t),
	}
}

func (sf *ServiceFactory) Auth(creds *tcclient.Credentials, rootURL string) tc.Auth {
	return sf.TCAuth
}

func (sf *ServiceFactory) Queue(creds *tcclient.Credentials, rootURL string) tc.Queue {
	return sf.TCQueue
}

func (sf *ServiceFactory) Secrets(creds *tcclient.Credentials, rootURL string) tc.Secrets {
	return sf.TCSecrets
}

func (sf *ServiceFactory) PurgeCache(creds *tcclient.Credentials, rootURL string) tc.PurgeCache {
	return sf.TCPurgeCache
}

func (sf *ServiceFactory) WorkerManager(creds *tcclient.Credentials, rootURL string) tc.WorkerManager {
	return sf.TCWorkerManager
}

func (sf *ServiceFactory) Artifacts(creds *tcclient.Credentials, rootURL string) tc.Artifacts {
	return sf.TCArtifacts
}
