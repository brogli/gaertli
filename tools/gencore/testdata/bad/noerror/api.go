package noerror

import "context"

type API interface {
	Ping(ctx context.Context) string
}
