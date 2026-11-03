package noctx

type API interface {
	Version() (string, error)
}
