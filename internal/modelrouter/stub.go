package modelrouter

type StubAdapter struct{}

func NewStubAdapter() Adapter {
	return StubAdapter{}
}

func (StubAdapter) Name() string {
	return "stub"
}

func (StubAdapter) Supports(provider string) bool {
	return provider == "stub"
}

func (StubAdapter) BuildRequest(req Request) (Request, error) {
	req.RequestPreview = buildPreview(req)
	req.Status = "stub_ready"
	return req, nil
}

func (StubAdapter) Execute(req Request) (Response, error) {
	return Response{
		Status:   "stubbed",
		Warnings: req.Warnings,
		Details: map[string]any{
			"adapter": "stub",
		},
	}, nil
}
