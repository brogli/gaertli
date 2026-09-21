package funcresult

import "context"

type API interface {
	Hook(ctx context.Context) (func(), error)
}
