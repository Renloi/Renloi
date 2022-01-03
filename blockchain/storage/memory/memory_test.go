package memory

import (
	"testing"

	"github.com/Renloi/Renloi/blockchain/storage"
)

func TestStorage(t *testing.T) {
	f := func(t *testing.T) (storage.Storage, func()) {
		s, _ := NewMemoryStorage(nil)
		return s, func() {}
	}
	storage.TestStorage(t, f)
}
