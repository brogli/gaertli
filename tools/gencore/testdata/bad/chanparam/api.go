package chanparam

import "context"

type API interface {
	Stream(ctx context.Context, out chan string) error
}
