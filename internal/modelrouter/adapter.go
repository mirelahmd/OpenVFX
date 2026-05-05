package modelrouter

type Adapter interface {
	Name() string
	Supports(provider string) bool
	BuildRequest(req Request) (Request, error)
	Execute(req Request) (Response, error)
}
