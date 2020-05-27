package httputil

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/taskcluster/httpbackoff/v3"
	"github.com/taskcluster/taskcluster/v30/workers/generic-worker/fileutil"
	"github.com/taskcluster/taskcluster/v30/workers/generic-worker/tclog"
)

// Utility function to aggressively download a url to a file location
func DownloadFile(url, contentSource, file string, logger tclog.Logger) (sha256, contentType string, err error) {
	var contentSize int64
	// httpbackoff.Get(url) is not sufficient as that only guarantees we have
	// an http response to read from, but does not retry if we lose
	// connectivity while reading from it. Therefore include the reading of the
	// response body inside the retry function.
	retryFunc := func() (resp *http.Response, tempError error, permError error) {
		logger.Infof("[mounts] Downloading %v to %v", contentSource, file)
		resp, err := http.Get(url)
		// assume all errors should result in a retry
		if err != nil {
			logger.Warnf("[mounts] Download of %v failed on this attempt: %v", contentSource, err)
			// temporary error!
			return resp, err, nil
		}
		defer resp.Body.Close()
		contentType = resp.Header.Get("Content-Type")
		f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			logger.Errorf("[mounts] Could not open file %v: %v", file, err)
			// permanent error!
			return resp, nil, err
		}
		defer f.Close()
		contentSize, err = io.Copy(f, resp.Body)
		if err != nil {
			logger.Warnf("[mounts] Could not write http response from %v to file %v on this attempt: %v", contentSource, file, err)
			// likely a temporary error - network blip
			return resp, err, nil
		}
		return resp, nil, nil
	}
	var resp *http.Response
	resp, _, err = httpbackoff.Retry(retryFunc)
	if err != nil {
		logger.Errorf("[mounts] Could not fetch from %v into file %v: %v", contentSource, file, err)
		return
	}
	defer resp.Body.Close()
	sha256, err = fileutil.CalculateSHA256(file)
	if err != nil {
		logger.Infof("[mounts] Downloaded %v bytes from %v to %v but cannot calculate SHA256", contentSize, contentSource, file)
		panic(fmt.Sprintf("Internal worker bug! Cannot calculate SHA256 of file %v that I just downloaded: %v", file, err))
	}
	logger.Infof("[mounts] Downloaded %v bytes with SHA256 %v from %v to %v", contentSize, sha256, contentSource, file)
	return
}

func WaitForLocalTCPListener(port uint16, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "localhost:"+strconv.Itoa(int(port)), 60*time.Second)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
		} else {
			_ = conn.Close()
			return nil
		}
	}
	return fmt.Errorf("Timed out waiting for port %v to be active after %v", port, timeout)
}
