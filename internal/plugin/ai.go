package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	log "github.com/sirupsen/logrus"
	"google.golang.org/genai"
)

// Google RPC error detail type URLs
const (
	typeURLRetryInfo    = "type.googleapis.com/google.rpc.RetryInfo"
	typeURLQuotaFailure = "type.googleapis.com/google.rpc.QuotaFailure"
)

// AIAnalysisResult represents the result of AI analysis
type AIAnalysisResult struct {
	Text       string `json:"text"`
	Promote    bool   `json:"promote"`
	Confidence int    `json:"confidence"`
}

// AIAnalysisParams represents parameters for AI analysis
type AIAnalysisParams struct {
	ModelName   string
	LogsContext string
	ExtraPrompt string
}

// analyzeLogsWithAI analyzes canary logs using AI
var analyzeLogsWithAI = func(params AIAnalysisParams) (rawJSON string, result AIAnalysisResult, err error) {
	apiKey, err := getSecretValue("argo-rollouts", "google_api_key")
	if err != nil {
		return "", AIAnalysisResult{}, fmt.Errorf("failed to get Google API key from secret: %v", err)
	}
	ctx := context.Background()

	// Create client using the new Google Gen AI Go SDK
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", AIAnalysisResult{}, err
	}

	system := "Analyze what was this canary behavior based on these logs, compare the stable version vs the canary version. " +
		"Write only a json text with these entries and nothing else: " +
		"one named 'text' with your analysis text; " +
		"one named 'promote' with true or false; " +
		"one named 'confidence' with a number from 0 to 100 representing your confidence in the decision. " +
		"The stable version logs start with '--- STABLE LOGS ---' and the canary version logs start with '--- CANARY LOGS ---'." +
		"In case that you cannot make a determination due to lack of information, default to promote: true."

	// Append extra prompt if provided
	if params.ExtraPrompt != "" {
		system += "\n\nAdditional context: " + params.ExtraPrompt
	}

	// Use the new API structure
	parts := []*genai.Part{
		{Text: system + "\n\n" + params.LogsContext},
	}

	var resp *genai.GenerateContentResponse
	err = retryWithBackoff(ctx, func() error {
		var apiErr error
		resp, apiErr = client.Models.GenerateContent(ctx, params.ModelName, []*genai.Content{{Parts: parts}}, nil)
		return apiErr
	}, 3) // Max 3 retries
	if err != nil {
		return "", AIAnalysisResult{}, err
	}

	txt := concatCandidates(resp)
	rawJSON = strings.TrimSpace(txt)

	// attempt to parse
	var obj AIAnalysisResult
	if e := json.Unmarshal([]byte(rawJSON), &obj); e != nil {
		// model might have returned extra text; try to extract JSON block
		if j := extractFirstJSON(rawJSON); j != "" {
			rawJSON = j
			_ = json.Unmarshal([]byte(rawJSON), &obj)
		}
	}
	return rawJSON, obj, nil
}

// retryWithBackoff implements exponential backoff for API calls with 429 error handling
func retryWithBackoff(ctx context.Context, operation func() error, maxRetries int) error {
	// Configure exponential backoff
	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = 1 * time.Second
	backoffConfig.MaxInterval = 60 * time.Second
	backoffConfig.Multiplier = 2.0
	backoffConfig.RandomizationFactor = 0.1

	// Create a custom backoff that respects API-provided wait times
	backoffConfig.Reset()

	var lastErr error
	attempt := 0

	operationWithLogging := func() (interface{}, error) {
		attempt++

		err := operation()
		if err != nil {
			lastErr = err

			// Check if it's a 429 error (rate limit)
			// Try to get the full APIError with all details (note: value type, not pointer)
			if apiErr, ok := err.(genai.APIError); ok {
				log.WithFields(log.Fields{
					"code":    apiErr.Code,
					"message": apiErr.Message,
					"status":  apiErr.Status,
				}).Error("Gemini API Error")

				// Check for ResourceExhausted (429)
				if apiErr.Code == http.StatusTooManyRequests || apiErr.Status == "RESOURCE_EXHAUSTED" {
					// Extract retry delay from API details
					var apiWaitTime time.Duration
					for _, detail := range apiErr.Details {
						detailType, _ := detail["@type"].(string)
						switch detailType {
						case typeURLRetryInfo:
							if retryDelayStr, ok := detail["retryDelay"].(string); ok && retryDelayStr != "" {
								// Parse duration string like "30s"
								if parsed, err := time.ParseDuration(retryDelayStr); err == nil {
									apiWaitTime = parsed
								}
							}
						case typeURLQuotaFailure:
							// Extract quota information
							violations, _ := detail["violations"].([]interface{})
							for _, violation := range violations {
								violationMap, _ := violation.(map[string]interface{})
								quotaMetric, _ := violationMap["quotaMetric"].(string)
								quotaId, _ := violationMap["quotaId"].(string)
								quotaValue, _ := violationMap["quotaValue"].(string)
								quotaDimensions, _ := violationMap["quotaDimensions"].(map[string]interface{})

								log.WithFields(log.Fields{
									"quotaMetric":     quotaMetric,
									"quotaId":         quotaId,
									"quotaValue":      quotaValue,
									"quotaDimensions": quotaDimensions,
								}).Warn("Quota violation - API rate limit exceeded")
							}
						}
					}

					// Use API-provided wait time or fall back to exponential backoff
					if apiWaitTime > 0 {
						log.WithFields(log.Fields{
							"attempt":     attempt,
							"apiWaitTime": apiWaitTime,
						}).Warn("Rate limit exceeded, using API-suggested wait time")

						// Override backoff with API-suggested wait time
						backoffConfig.Reset()
						backoffConfig.InitialInterval = apiWaitTime
						backoffConfig.MaxInterval = apiWaitTime
					} else {
						log.WithFields(log.Fields{
							"attempt": attempt,
						}).Warn("Rate limit exceeded, using exponential backoff")
					}

					return nil, err
				}
			}

			// For non-429 errors, don't retry
			return nil, backoff.Permanent(err)
		}

		// Success
		return nil, nil
	}

	// Use the backoff library with context support
	_, err := backoff.Retry(ctx, operationWithLogging, backoff.WithBackOff(backoffConfig))
	if err != nil {
		return fmt.Errorf("max retries exceeded after %d attempts, last error: %v", attempt, lastErr)
	}

	return nil
}

// concatCandidates concatenates text from all candidates in the response
func concatCandidates(resp *genai.GenerateContentResponse) string {
	var b strings.Builder
	if resp == nil {
		return ""
	}
	for _, cand := range resp.Candidates {
		for _, part := range cand.Content.Parts {
			if part.Text != "" {
				b.WriteString(part.Text)
			}
		}
	}
	return b.String()
}

// extractFirstJSON extracts the first JSON block from a string
func extractFirstJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}
