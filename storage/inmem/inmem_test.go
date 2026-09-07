package inmem_test

import (
	"testing"

	"github.com/deploymenttheory/go-microsoft-dm/storage"
	"github.com/deploymenttheory/go-microsoft-dm/storage/inmem"
	"github.com/deploymenttheory/go-microsoft-dm/storage/storagetest"
)

func TestContract(t *testing.T) {
	t.Parallel()
	storagetest.RunAll(t, func(*testing.T) storage.Store { return inmem.New() })
}
