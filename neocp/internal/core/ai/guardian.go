package ai

import (
	"context"
	"strings"
)

type DiagnosticSuggestion struct {
	Type        string `json:"type"` // security, performance, health
	Message     string `json:"message"`
	ActionLabel string `json:"action_label"`
	ActionCmd   string `json:"action_cmd"`
}

// AnalyzeLogs simulates an AI analysis of system logs
func AnalyzeLogs(ctx context.Context, logs []string) []DiagnosticSuggestion {
	var suggestions []DiagnosticSuggestion

	for _, log := range logs {
		if strings.Contains(log, "denied") || strings.Contains(log, "failed") {
			suggestions = append(suggestions, DiagnosticSuggestion{
				Type:        "security",
				Message:     "Detected multiple access denied attempts in auth logs.",
				ActionLabel: "Enable cPHulk Protection",
				ActionCmd:   "enable_cphulk",
			})
			break
		}
		if strings.Contains(log, "slow") || strings.Contains(log, "timeout") {
			suggestions = append(suggestions, DiagnosticSuggestion{
				Type:        "performance",
				Message:     "Web server reports slow upstream response from PHP-FPM.",
				ActionLabel: "Increase PHP Memory Limit",
				ActionCmd:   "increase_php_mem",
			})
			break
		}
	}

	// Default suggestion if nothing found
	if len(suggestions) == 0 {
		suggestions = append(suggestions, DiagnosticSuggestion{
			Type:        "health",
			Message:     "System appears healthy. Recommend a weekly full server scan.",
			ActionLabel: "Schedule Scan",
			ActionCmd:   "schedule_scan",
		})
	}

	return suggestions
}
