package ifacefield

import "context"

type Opts struct {
	Filter interface{ Match(string) bool }
}

type API interface {
	Load(ctx context.Context, o Opts) error
}
