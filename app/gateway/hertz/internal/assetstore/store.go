package assetstore

import (
	"context"
	"io"
)

// Store is the write boundary used by HTTP handlers. Implementations may
// persist assets on a local filesystem or in object storage.
type Store interface {
	Save(context.Context, string, string, io.Reader, int64) (Asset, error)
}
