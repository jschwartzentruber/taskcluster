package tcmock

import (
	"testing"

	"github.com/taskcluster/taskcluster/v30/clients/client-go/tcsecrets"
)

type Secrets struct {
}

/////////////////////////////////////////////////

func (secrets *Secrets) Get(name string) (*tcsecrets.Secret, error) {
	return &tcsecrets.Secret{}, nil
}

/////////////////////////////////////////////////

func NewSecrets(t *testing.T) *Secrets {
	s := &Secrets{}
	return s
}
