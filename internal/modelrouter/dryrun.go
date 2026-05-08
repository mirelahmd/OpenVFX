package modelrouter

import (
	"fmt"
	"strings"
)

type DryRunAdapter struct{}

func NewDryRunAdapter() Adapter {
	return DryRunAdapter{}
}

func (DryRunAdapter) Name() string {
	return "dry-run"
}

func (DryRunAdapter) Supports(provider string) bool {
	return provider == "dry-run"
}

func (DryRunAdapter) BuildRequest(req Request) (Request, error) {
	req.RequestPreview = buildPreview(req)
	req.Status = "dry_run"
	return req, nil
}

func (DryRunAdapter) Execute(req Request) (Response, error) {
	return Response{
		Status:   "dry_run",
		Warnings: req.Warnings,
		Details: map[string]any{
			"adapter": "dry-run",
		},
	}, nil
}

func buildPreview(req Request) RequestPreview {
	contractBits := []string{}
	if maxItems, ok := req.Input.OutputContract["max_items"]; ok {
		contractBits = append(contractBits, fmt.Sprintf("max_items=%v", maxItems))
	}
	if maxWords, ok := req.Input.OutputContract["max_words"]; ok {
		contractBits = append(contractBits, fmt.Sprintf("max_words=%v", maxWords))
	}
	if style, ok := req.Input.OutputContract["style"]; ok {
		contractBits = append(contractBits, fmt.Sprintf("style=%v", style))
	}
	system := "Follow the inference mask exactly. Do not invent facts, timestamps, or speakers."
	decisionBits := []string{}
	for _, decision := range req.Input.Decisions {
		decisionBits = append(decisionBits, fmt.Sprintf("%s %.2f-%.2f %s", decision.ID, decision.Start, decision.End, decision.TextPreview))
	}
	var user string
	switch req.TaskType {
	case "caption_variants":
		user = fmt.Sprintf("Generate short caption variants for this clip. Do not invent facts. Return JSON: {\"items\":[{\"text\":\"caption\"}]}. Decisions: %s. Route: %s. Contract: %s.",
			strings.Join(decisionBits, " | "), req.RouteName, strings.Join(contractBits, ", "))
	case "timeline_labels":
		user = fmt.Sprintf("Generate short timeline labels for this clip. Do not invent facts. Return JSON: {\"labels\":[\"label\"]}. Decisions: %s. Route: %s. Contract: %s.",
			strings.Join(decisionBits, " | "), req.RouteName, strings.Join(contractBits, ", "))
	case "short_descriptions":
		user = fmt.Sprintf("Generate concise short descriptions for this clip. Do not invent facts. Return JSON: {\"descriptions\":[\"description\"]}. Decisions: %s. Route: %s. Contract: %s.",
			strings.Join(decisionBits, " | "), req.RouteName, strings.Join(contractBits, ", "))
	case "goal_reranking":
		user = fmt.Sprintf("Rerank these highlight candidates for the user goal. Goal: %v. Constraints: %v. Return JSON: {\"ranked_highlights\":[{\"highlight_id\":\"hl_0001\",\"goal_score\":0.91,\"reason\":\"Strong match for the goal.\"}]}. Candidates: %s.",
			req.Input.Constraints["goal"], req.Input.Constraints, strings.Join(decisionBits, " | "))
	default:
		user = fmt.Sprintf("Task type: %s. Decisions: %d. Route: %s. Contract: %s.",
			req.TaskType, len(req.Input.Decisions), req.RouteName, strings.Join(contractBits, ", "))
	}
	return RequestPreview{
		System:       system,
		User:         user,
		OutputSchema: outputSchema(req.TaskType),
	}
}

func outputSchema(taskType string) string {
	switch taskType {
	case "goal_reranking":
		return "goal_rerank.response.v1"
	default:
		return "expansion_output.v1"
	}
}
