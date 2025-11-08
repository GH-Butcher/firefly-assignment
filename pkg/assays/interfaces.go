package assays

import "context"

type Assay interface {
	HandleAssays(ctx context.Context) (Result, error)
}
