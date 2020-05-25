package tcmock

import (
	"testing"

	"github.com/taskcluster/taskcluster/v30/clients/client-go/tcworkermanager"
)

type WorkerManager struct {
}

/////////////////////////////////////////////////

func (workerManager *WorkerManager) RegisterWorker(payload *tcworkermanager.RegisterWorkerRequest) (*tcworkermanager.RegisterWorkerResponse, error) {
	return &tcworkermanager.RegisterWorkerResponse{}, nil
}

func (workerManager *WorkerManager) WorkerPool(workerPoolId string) (*tcworkermanager.WorkerPoolFullDefinition, error) {
	return &tcworkermanager.WorkerPoolFullDefinition{}, nil
}

/////////////////////////////////////////////////

func NewWorkerManager(t *testing.T) *WorkerManager {
	wm := &WorkerManager{}
	return wm
}
