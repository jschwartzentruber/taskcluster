package tcmock

import (
	"encoding/json"
	"net/http"
	"testing"
)

func WriteAsJSON(t *testing.T, w http.ResponseWriter, resp interface{}) {
	bytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		t.Fatalf("Strange - I can't convert %#v to json: %v", resp, err)
	}
	_, err = w.Write(bytes)
	if err != nil {
		t.Logf("Response: %v", string(bytes))
		t.Fatalf("Error writing response: %v", err)
	}
}
