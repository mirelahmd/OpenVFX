package modelrouter

type DecisionInput struct {
	ID          string  `json:"id"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Decision    string  `json:"decision"`
	Reason      string  `json:"reason"`
	TextPreview string  `json:"text_preview,omitempty"`
}

type RequestInput struct {
	Decisions      []DecisionInput `json:"decisions"`
	Constraints    map[string]any  `json:"constraints"`
	OutputContract map[string]any  `json:"output_contract"`
}

type RequestPreview struct {
	System       string `json:"system"`
	User         string `json:"user"`
	OutputSchema string `json:"output_schema"`
}

type Request struct {
	TaskID         string         `json:"task_id"`
	TaskType       string         `json:"task_type"`
	RouteName      string         `json:"route_name"`
	ModelEntryName string         `json:"model_entry_name,omitempty"`
	Provider       string         `json:"provider,omitempty"`
	Model          string         `json:"model,omitempty"`
	Role           string         `json:"role,omitempty"`
	BaseURL        string         `json:"base_url,omitempty"`
	Options        map[string]any `json:"options,omitempty"`
	Input          RequestInput   `json:"input"`
	RequestPreview RequestPreview `json:"request_preview"`
	Status         string         `json:"status"`
	Warnings       []string       `json:"warnings,omitempty"`
}

type Response struct {
	Status   string         `json:"status"`
	Texts    []string       `json:"texts,omitempty"`
	Mode     string         `json:"mode,omitempty"`
	Warnings []string       `json:"warnings,omitempty"`
	Details  map[string]any `json:"details,omitempty"`
}
