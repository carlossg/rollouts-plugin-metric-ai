package plugin

import (
	"encoding/json"
	"os"

	log "github.com/sirupsen/logrus"
)

// Analysis mode constants
const (
	AnalysisModeDefault = "default" // Current implementation
	AnalysisModeAgent   = "agent"   // Delegate to kubernetes-agent
)

// analyzeWithMode analyzes logs using the specified mode
func analyzeWithMode(mode, modelName, logsContext, namespace, podName, extraPrompt string) (string, AIAnalysisResult, error) {
	log.WithFields(log.Fields{
		"mode":      mode,
		"namespace": namespace,
		"podName":   podName,
	}).Info("Analyzing with mode")

	switch mode {
	case AnalysisModeAgent:
		return analyzeWithKubernetesAgent(namespace, podName, logsContext)
	default:
		return analyzeLogsWithAI(modelName, logsContext, extraPrompt)
	}
}

// analyzeWithKubernetesAgent delegates analysis to the Kubernetes Agent via A2A
func analyzeWithKubernetesAgent(namespace, podName, logsContext string) (string, AIAnalysisResult, error) {
	agentURL := os.Getenv("K8S_AGENT_URL")
	if agentURL == "" {
		agentURL = "http://kubernetes-agent.argo-rollouts.svc.cluster.local:8080"
	}

	log.WithField("agentURL", agentURL).Info("Using Kubernetes Agent for analysis")

	client := NewA2AClient(agentURL)

	// Health check first
	if err := client.HealthCheck(); err != nil {
		log.WithError(err).Error("Kubernetes Agent health check failed, falling back to default mode")
		return analyzeLogsWithAI("gemini-2.0-flash-exp", logsContext, "")
	}

	// Extract stable and canary logs from logsContext
	stableLogs, canaryLogs := splitLogs(logsContext)

	// Send request to agent
	resp, err := client.AnalyzeWithAgent(namespace, podName, stableLogs, canaryLogs)
	if err != nil {
		log.WithError(err).Error("Failed to analyze with kubernetes-agent, falling back to default mode")
		return analyzeLogsWithAI("gemini-2.0-flash-exp", logsContext, "")
	}

	// Build result object
	result := AIAnalysisResult{
		Text:       resp.Analysis,
		Promote:    resp.Promote,
		Confidence: resp.Confidence,
	}

	// Build JSON response for Argo Rollouts
	jsonResp := map[string]interface{}{
		"text":        resp.Analysis,
		"promote":     resp.Promote,
		"confidence":  resp.Confidence,
		"rootCause":   resp.RootCause,
		"remediation": resp.Remediation,
	}
	if resp.PRLink != "" {
		jsonResp["prLink"] = resp.PRLink
		log.WithField("prLink", resp.PRLink).Info("Agent created a PR with fix")
	}

	rawJSON, err := json.Marshal(jsonResp)
	if err != nil {
		return "", AIAnalysisResult{}, err
	}

	log.WithFields(log.Fields{
		"promote":    resp.Promote,
		"confidence": resp.Confidence,
	}).Info("Analysis completed via Kubernetes Agent")

	return string(rawJSON), result, nil
}
