package memory_test

import (
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/memory"
	"github.com/kmradished/GOsapiens/internal/storecontract"
)

func TestPairStoreContract(t *testing.T) {
	storecontract.Check(t, func() application.PairStore {
		return memory.NewPairStore()
	})
}
