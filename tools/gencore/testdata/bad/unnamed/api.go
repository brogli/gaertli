package unnamed

import "context"

type API interface {
	Load(ctx context.Context, _ string) error
}
