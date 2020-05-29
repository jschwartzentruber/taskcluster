package tcmock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/taskcluster/httpbackoff/v3"
	tcclient "github.com/taskcluster/taskcluster/v30/clients/client-go"
	"github.com/taskcluster/taskcluster/v30/clients/client-go/tcqueue"
)

type Queue struct {
	mu sync.RWMutex
	t  *testing.T

	// orderedTasks stores FIFO sorted taskIds since `range q.tasks` returns
	// taskIds in an arbitrary order
	orderedTasks []string

	// tasks["<taskId>"]
	tasks map[string]*tcqueue.TaskDefinitionAndStatus

	// artifacts["<taskId>:<runId>"]["<name>"]
	artifacts map[string]map[string]interface{}
}

func (q *Queue) Handle(handler *http.ServeMux, t *testing.T) {

	const (
		PingPath      = "/ping"
		ClaimWorkPath = "/claim-work/"
		// ProvisionersPath     = "/provisioners"
		// ListProvisionersPath = "/provisioners/"
		// TaskGroupPath        = "/task-group/"
		// TaskPath             = "/task/"
	)

	handler.HandleFunc(ClaimWorkPath, func(w http.ResponseWriter, req *http.Request) {
		workerPool := PathSuffix(t, req, ClaimWorkPath)
		switch req.Method {
		case "POST":
			provisionerId, err := url.QueryUnescape(strings.SplitN(workerPool, "/", 2)[0])
			if err != nil {
				BadRequest(w, err)
			}
			workerType, err := url.QueryUnescape(strings.SplitN(workerPool, "/", 2)[1])
			if err != nil {
				BadRequest(w, err)
			}
			dec := json.NewDecoder(req.Body)
			dec.DisallowUnknownFields()
			var payload tcqueue.ClaimWorkRequest
			err = dec.Decode(&payload)
			if err != nil {
				BadRequest(w, err)
			}
			resp, err := q.ClaimWork(provisionerId, workerType, &payload)
			if err != nil {
				BadRequest(w, err)
			}
			WriteAsJSON(t, w, resp)
		default:
			InvalidMethod(w, req)
		}
	})

	handler.HandleFunc(PingPath, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case "GET":
			q.Ping(t, w, req)
		default:
			InvalidMethod(w, req)
		}
	})
}

func (queue *Queue) Ping(t *testing.T, w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)
}

/////////////////////////////////////////////////

func (queue *Queue) ClaimWork(provisionerId, workerType string, payload *tcqueue.ClaimWorkRequest) (*tcqueue.ClaimWorkResponse, error) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	maxTasks := payload.Tasks
	tasks := []tcqueue.TaskClaim{}
	for _, taskId := range queue.orderedTasks {
		j := queue.tasks[taskId]
		if j.Task.WorkerType == workerType && j.Task.ProvisionerID == provisionerId && j.Status.State == "pending" {
			j.Status.State = "running"
			j.Status.Runs = []tcqueue.RunInformation{
				{
					RunID:         0,
					ReasonCreated: "scheduled",
				},
			}
			tasks = append(
				tasks,
				tcqueue.TaskClaim{
					Task:   j.Task,
					Status: j.Status,
				},
			)
			if len(tasks) == int(maxTasks) {
				break
			}
		}
	}
	return &tcqueue.ClaimWorkResponse{
		Tasks: tasks,
	}, nil
}

func (queue *Queue) CreateArtifact(taskId, runId, name string, payload *tcqueue.PostArtifactRequest) (*tcqueue.PostArtifactResponse, error) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	queue.t.Logf("queue.CreateArtifact called with taskId %v and runId %v for artifact %v", taskId, runId, name)

	if _, mapAlreadyCreated := queue.artifacts[taskId+":"+runId]; !mapAlreadyCreated {
		queue.artifacts[taskId+":"+runId] = map[string]interface{}{}
	}

	var request tcqueue.Artifact
	err := json.Unmarshal([]byte(*payload), &request)
	if err != nil {
		queue.t.Fatalf("Error unmarshalling from json: %v", err)
	}

	var req, resp interface{}
	switch request.StorageType {
	case "s3":
		var s3Request tcqueue.S3ArtifactRequest
		err = json.Unmarshal([]byte(*payload), &s3Request)
		if err != nil {
			queue.t.Fatalf("Error unmarshalling S3 Artifact Request from json: %v", err)
		}
		req = &s3Request
		resp, err = queue.createS3Artifact(taskId, runId, name, &s3Request)
	case "error":
		var errorRequest tcqueue.ErrorArtifactRequest
		err = json.Unmarshal([]byte(*payload), &errorRequest)
		if err != nil {
			queue.t.Fatalf("Error unmarshalling Error Artifact Request from json: %v", err)
		}
		req = &errorRequest
		resp, err = queue.createErrorArtifact(taskId, runId, name, &errorRequest)
	case "reference":
		var redirectRequest tcqueue.RedirectArtifactRequest
		err = json.Unmarshal([]byte(*payload), &redirectRequest)
		if err != nil {
			queue.t.Fatalf("Error unmarshalling Redirect Artifact Request from json: %v", err)
		}
		req = &redirectRequest
		resp, err = queue.createRedirectArtifact(taskId, runId, name, &redirectRequest)
	default:
		queue.t.Fatalf("Unrecognised storage type: %v", request.StorageType)
	}

	if err != nil {
		return nil, err
	}

	queue.artifacts[taskId+":"+runId][name] = req
	queue.t.Logf("Added artifact %v for task %v run %v artifact %v", req, taskId, runId, name)

	var par tcqueue.PostArtifactResponse
	par, err = json.Marshal(resp)
	if err != nil {
		queue.t.Fatalf("Error marshalling into json: %v", err)
	}
	return &par, nil
}

func (queue *Queue) ensureUnchangedIfAlreadyExists(taskId, runId, name string, request interface{}) error {
	previousVersion, existed := queue.artifacts[taskId+":"+runId][name]
	if !existed || reflect.DeepEqual(previousVersion, request) {
		return nil
	}
	return &tcclient.APICallException{
		CallSummary: &tcclient.CallSummary{
			HTTPResponseBody: fmt.Sprintf("Request conflict: artifact %v in taskId %v and runId %v exists with different values: disallowing update %v -> %v", name, taskId, runId, previousVersion, request),
		},
		RootCause: httpbackoff.BadHttpResponseCode{
			HttpResponseCode: 409,
		},
	}
}

func (queue *Queue) createS3Artifact(taskId, runId, name string, s3Request *tcqueue.S3ArtifactRequest) (*tcqueue.S3ArtifactResponse, error) {
	err := queue.ensureUnchangedIfAlreadyExists(taskId, runId, name, s3Request)
	if err != nil {
		return nil, err
	}
	return &tcqueue.S3ArtifactResponse{
		ContentType: s3Request.ContentType,
		Expires:     s3Request.Expires,
		PutURL:      "http://localhost:12453",
		StorageType: s3Request.StorageType,
	}, nil
}

func (queue *Queue) createErrorArtifact(taskId, runId, name string, errorRequest *tcqueue.ErrorArtifactRequest) (*tcqueue.ErrorArtifactResponse, error) {
	err := queue.ensureUnchangedIfAlreadyExists(taskId, runId, name, errorRequest)
	if err != nil {
		return nil, err
	}
	return &tcqueue.ErrorArtifactResponse{
		StorageType: errorRequest.StorageType,
	}, nil
}

func (queue *Queue) createRedirectArtifact(taskId, runId, name string, redirectRequest *tcqueue.RedirectArtifactRequest) (*tcqueue.RedirectArtifactResponse, error) {
	previousVersion, existed := queue.artifacts[taskId+":"+runId][name]
	if !existed {
		return &tcqueue.RedirectArtifactResponse{
			StorageType: redirectRequest.StorageType,
		}, nil
	}
	if _, wasRedirect := previousVersion.(*tcqueue.RedirectArtifactRequest); wasRedirect {
		// new reference artifact allowed with different URL / Content Type / Expiry
		return &tcqueue.RedirectArtifactResponse{
			StorageType: redirectRequest.StorageType,
		}, nil
	}
	return nil, &tcclient.APICallException{
		CallSummary: &tcclient.CallSummary{
			HTTPResponseBody: fmt.Sprintf("Request conflict: redirect artifact %v in taskId %v and runId %v cannot replace a non-redirect artifact: disallowing update %v -> %v", name, taskId, runId, previousVersion, redirectRequest),
		},
		RootCause: httpbackoff.BadHttpResponseCode{
			HttpResponseCode: 409,
		},
	}
}

func (queue *Queue) CreateTask(taskId string, payload *tcqueue.TaskDefinitionRequest) (*tcqueue.TaskStatusResponse, error) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	queue.tasks[taskId] = &tcqueue.TaskDefinitionAndStatus{
		Status: tcqueue.TaskStatusStructure{
			TaskID: taskId,
			State:  "pending",
		},
		Task: tcqueue.TaskDefinitionResponse{
			Created:       payload.Created,
			Deadline:      payload.Deadline,
			Dependencies:  payload.Dependencies,
			Expires:       payload.Expires,
			Extra:         payload.Extra,
			Metadata:      payload.Metadata,
			Payload:       payload.Payload,
			Priority:      payload.Priority,
			ProvisionerID: payload.ProvisionerID,
			Requires:      payload.Requires,
			Retries:       payload.Retries,
			Routes:        payload.Routes,
			SchedulerID:   payload.SchedulerID,
			Scopes:        payload.Scopes,
			Tags:          payload.Tags,
			TaskGroupID:   payload.TaskGroupID,
			WorkerType:    payload.WorkerType,
		},
	}
	tsr := &tcqueue.TaskStatusResponse{
		Status: tcqueue.TaskStatusStructure{
			Deadline:      payload.Deadline,
			Expires:       payload.Expires,
			ProvisionerID: payload.ProvisionerID,
			RetriesLeft:   payload.Retries,
			Runs:          []tcqueue.RunInformation{},
			SchedulerID:   payload.SchedulerID,
			State:         "pending",
			TaskGroupID:   payload.TaskGroupID,
			TaskID:        taskId,
			WorkerType:    payload.WorkerType,
		},
	}
	queue.orderedTasks = append(queue.orderedTasks, taskId)
	return tsr, nil
}

func (queue *Queue) GetLatestArtifact_SignedURL(taskId, name string, duration time.Duration) (*url.URL, error) {
	queue.mu.RLock()
	defer queue.mu.RUnlock()
	queue.t.Logf("queue.GetLatestArtifact_SignedURL called with taskId %v", taskId)
	taskRunArtifacts, exists := queue.artifacts[taskId+":0"]
	if !exists {
		queue.t.Logf("No artifacts for task %v (runId 0) found", taskId)
		return nil, &tcclient.APICallException{
			RootCause: httpbackoff.BadHttpResponseCode{
				HttpResponseCode: 404,
			},
		}
	}
	artifact, exists := taskRunArtifacts[name]
	if !exists {
		queue.t.Logf("Task %v (runId 0) found, but does not have artifact %v", taskId, name)
		return nil, &tcclient.APICallException{
			RootCause: httpbackoff.BadHttpResponseCode{
				HttpResponseCode: 404,
			},
		}
	}
	switch a := artifact.(type) {
	case *tcqueue.S3ArtifactRequest:
		// nil simply signifies to mocks that the artifact isn't a redirect artifact
		return nil, nil
	case *tcqueue.ErrorArtifactRequest:
		// nil simply signifies to mocks that the artifact isn't a redirect artifact
		return nil, nil
	case *tcqueue.RedirectArtifactRequest:
		return url.Parse(a.URL)
	}
	queue.t.Fatalf("Unknown artifact type %T", artifact)
	return nil, fmt.Errorf("Unknown artifact type %T", artifact)
}

func (queue *Queue) ListArtifacts(taskId, runId, continuationToken, limit string) (*tcqueue.ListArtifactsResponse, error) {
	queue.mu.RLock()
	defer queue.mu.RUnlock()
	queue.t.Logf("queue.ListArtifacts called with taskId %v and runId %v", taskId, runId)
	artifacts := []tcqueue.Artifact{}
	for name, artifact := range queue.artifacts[taskId+":"+runId] {
		var a tcqueue.Artifact
		switch A := artifact.(type) {
		case *tcqueue.ErrorArtifactRequest:
			a = tcqueue.Artifact{
				ContentType: "application/json", // TODO - check this
				Expires:     A.Expires,
				Name:        name,
				StorageType: A.StorageType,
			}
		case *tcqueue.RedirectArtifactRequest:
			a = tcqueue.Artifact{
				ContentType: A.ContentType,
				Expires:     A.Expires,
				Name:        name,
				StorageType: A.StorageType,
			}
		case *tcqueue.S3ArtifactRequest:
			a = tcqueue.Artifact{
				ContentType: A.ContentType,
				Expires:     A.Expires,
				Name:        name,
				StorageType: A.StorageType,
			}
		default:
			queue.t.Fatalf("Invalid artifact request type for artifact %#v for task %v runId %v", a, taskId, runId)
		}
		artifacts = append(artifacts, a)
	}
	return &tcqueue.ListArtifactsResponse{
		Artifacts: artifacts,
	}, nil
}

func (queue *Queue) ReclaimTask(taskId, runId string) (*tcqueue.TaskReclaimResponse, error) {
	queue.mu.RLock()
	defer queue.mu.RUnlock()
	return &tcqueue.TaskReclaimResponse{
		Status: queue.tasks[taskId].Status,
	}, nil
}

func (queue *Queue) ReportCompleted(taskId, runId string) (*tcqueue.TaskStatusResponse, error) {
	queue.mu.Lock()
	queue.tasks[taskId].Status.Runs[0].ReasonResolved = "completed"
	queue.tasks[taskId].Status.Runs[0].State = "completed"
	queue.mu.Unlock()
	return queue.Status(taskId)
}

func (queue *Queue) ReportException(taskId, runId string, payload *tcqueue.TaskExceptionRequest) (*tcqueue.TaskStatusResponse, error) {
	queue.mu.Lock()
	queue.tasks[taskId].Status.Runs[0].ReasonResolved = payload.Reason
	queue.tasks[taskId].Status.Runs[0].State = "exception"
	queue.mu.Unlock()
	return queue.Status(taskId)
}

func (queue *Queue) ReportFailed(taskId, runId string) (*tcqueue.TaskStatusResponse, error) {
	queue.mu.Lock()
	queue.tasks[taskId].Status.Runs[0].ReasonResolved = "failed"
	queue.tasks[taskId].Status.Runs[0].State = "failed"
	queue.mu.Unlock()
	return queue.Status(taskId)
}

func (queue *Queue) Status(taskId string) (*tcqueue.TaskStatusResponse, error) {
	queue.mu.RLock()
	defer queue.mu.RUnlock()
	return &tcqueue.TaskStatusResponse{
		Status: queue.tasks[taskId].Status,
	}, nil
}

func (queue *Queue) Task(taskId string) (*tcqueue.TaskDefinitionResponse, error) {
	queue.mu.RLock()
	defer queue.mu.RUnlock()
	if _, exists := queue.tasks[taskId]; !exists {
		queue.t.Logf("Task definition for task %v not found", taskId)
		return nil, &tcclient.APICallException{
			RootCause: httpbackoff.BadHttpResponseCode{
				HttpResponseCode: 404,
			},
		}
	}
	return &queue.tasks[taskId].Task, nil
}

/////////////////////////////////////////////////

func NewQueue(t *testing.T) *Queue {
	return &Queue{
		t:         t,
		tasks:     map[string]*tcqueue.TaskDefinitionAndStatus{},
		artifacts: map[string]map[string]interface{}{},
	}
}
