package tcmock

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/taskcluster/httpbackoff/v3"
	tcclient "github.com/taskcluster/taskcluster/v30/clients/client-go"
	"github.com/taskcluster/taskcluster/v30/workers/generic-worker/fileutil"
	tchttputil "github.com/taskcluster/taskcluster/v30/workers/generic-worker/httputil"
	"github.com/taskcluster/taskcluster/v30/workers/generic-worker/tc"
	"github.com/taskcluster/taskcluster/v30/workers/generic-worker/tclog"
)

type Artifacts struct {
	t *testing.T
	// artifacts["<taskId>:<name>"]
	artifacts map[string]*Artifact
	queue     tc.Queue
}

type Artifact struct {
	taskId          string
	runId           uint
	name            string
	contentType     string
	contentEncoding string
	data            []byte
}

/////////////////////////////////////////////////

func (a *Artifacts) Publish(taskId string, runId uint, name, putURL, contentType, contentEncoding, file string) error {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		a.t.Fatalf("Could not read file %v for artifact %v of task %v: %v", file, name, taskId, err)
	}
	//	if A, exists := a.artifacts[taskId+":"+name]; exists {
	//		if contentType != A.contentType || contentEncoding != A.contentEncoding || !reflect.DeepEqual(b, A.data) {
	//			return &tcclient.APICallException{
	//				CallSummary: &tcclient.CallSummary{
	//					HTTPResponseBody: fmt.Sprintf("Conflicting request for task %v run %v artifact %v", taskId, runId, name),
	//				},
	//				RootCause: httpbackoff.BadHttpResponseCode{
	//					HttpResponseCode: 409,
	//				},
	//			}
	//		}
	//	}
	a.artifacts[taskId+":"+name] = &Artifact{
		taskId:          taskId,
		runId:           runId,
		name:            name,
		contentType:     contentType,
		contentEncoding: contentEncoding,
		data:            b,
	}
	return nil
}

func (a *Artifacts) GetLatest(taskId, name, file string, timeout time.Duration, logger tclog.Logger) (sha256, contentEncoding, contentType string, err error) {
	a.t.Logf("artifacts.GetLatest called with taskId %v", taskId)
	u, err := a.queue.GetLatestArtifact_SignedURL(taskId, name, timeout)
	if err != nil {
		return "", "", "", err
	}
	contentSource := "task " + taskId + " artifact " + name
	logger.Infof("[mounts] Downloading %v to %v", contentSource, file)

	// only reference artifacts return URLs...
	if u != nil {
		sha256, contentType, err = tchttputil.DownloadFile(u.String(), "task "+taskId+" artifact "+name, file, logger)
		contentEncoding = "unknown"
		return
	}

	if _, exists := a.artifacts[taskId+":"+name]; !exists {
		return "", "", "", &tcclient.APICallException{
			CallSummary: &tcclient.CallSummary{
				HTTPResponseBody: "Not found",
			},
			RootCause: httpbackoff.BadHttpResponseCode{
				HttpResponseCode: 404,
			},
		}
	}
	artifact := a.artifacts[taskId+":"+name]
	var size int64
	size, err = artifact.WriteToDisk(file)
	if err != nil {
		return
	}
	sha256, err = fileutil.CalculateSHA256(file)
	if err != nil {
		return
	}
	logger.Infof("[mounts] Downloaded %v bytes with SHA256 %v from %v to %v", size, sha256, contentSource, file)
	contentEncoding = artifact.contentEncoding
	contentType = artifact.contentType
	return
}

func (artifact *Artifact) WriteToDisk(file string) (size int64, err error) {
	if artifact.contentEncoding == "gzip" {
		var zr *gzip.Reader
		var f *os.File
		f, err = os.Create(file)
		if err != nil {
			return
		}
		defer func() {
			err2 := f.Close()
			if err == nil {
				err = err2
			}
		}()
		zr, err = gzip.NewReader(bytes.NewBuffer(artifact.data))
		size, err = io.Copy(f, zr)
	} else {
		size = int64(len(artifact.data))
		err = ioutil.WriteFile(file, artifact.data, 0777)
	}
	return
}

/////////////////////////////////////////////////

func NewArtifacts(t *testing.T, queue tc.Queue) *Artifacts {
	return &Artifacts{
		t:         t,
		artifacts: map[string]*Artifact{},
		queue:     queue,
	}
}
