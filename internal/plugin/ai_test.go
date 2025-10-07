package plugin

import (
	"encoding/json"
	"os"
	"testing"

	"google.golang.org/genai"
)

// TestConcatCandidates tests the concatCandidates function
func TestConcatCandidates(t *testing.T) {
	tests := []struct {
		name     string
		response *genai.GenerateContentResponse
		expected string
	}{
		{
			name:     "nil response",
			response: nil,
			expected: "",
		},
		{
			name: "empty response",
			response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{},
			},
			expected: "",
		},
		{
			name: "single candidate with text",
			response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: "Hello world"},
							},
						},
					},
				},
			},
			expected: "Hello world",
		},
		{
			name: "multiple parts in single candidate",
			response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: "First part"},
								{Text: " second part"},
							},
						},
					},
				},
			},
			expected: "First part second part",
		},
		{
			name: "multiple candidates",
			response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: "First candidate"},
							},
						},
					},
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: "Second candidate"},
							},
						},
					},
				},
			},
			expected: "First candidateSecond candidate",
		},
		{
			name: "empty text parts",
			response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: ""},
								{Text: "Not empty"},
								{Text: ""},
							},
						},
					},
				},
			},
			expected: "Not empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := concatCandidates(tt.response)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestExtractFirstJSON tests the extractFirstJSON function
func TestExtractFirstJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no JSON",
			input:    "just some text",
			expected: "",
		},
		{
			name:     "simple JSON",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with extra text before",
			input:    `Here's the result: {"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with extra text after",
			input:    `{"key": "value"} and more text`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with extra text both sides",
			input:    `Prefix text {"key": "value"} suffix text`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "nested JSON",
			input:    `{"outer": {"inner": "value"}}`,
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "deeply nested JSON",
			input:    `{"level1": {"level2": {"level3": "value"}}}`,
			expected: `{"level1": {"level2": {"level3": "value"}}}`,
		},
		{
			name:     "multiple JSON objects - returns first",
			input:    `{"first": "value"} and {"second": "value"}`,
			expected: `{"first": "value"}`,
		},
		{
			name:     "malformed JSON - unclosed brace",
			input:    `{"key": "value"`,
			expected: "",
		},
		{
			name:     "malformed JSON - extra closing brace",
			input:    `{"key": "value"}}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with newlines",
			input:    "{\n  \"key\": \"value\"\n}",
			expected: "{\n  \"key\": \"value\"\n}",
		},
		{
			name:     "JSON array",
			input:    `{"items": [1, 2, 3]}`,
			expected: `{"items": [1, 2, 3]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFirstJSON(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestAnalyzeLogsWithAI_Integration is an integration test that uses real API credentials
// Skip this test in normal runs, only run with: go test -run TestAnalyzeLogsWithAI_Integration
// Requires GOOGLE_API_KEY environment variable to be set
func TestAnalyzeLogsWithAI_Integration(t *testing.T) {
	// Skip if not explicitly requested or if API key is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: GOOGLE_API_KEY not set")
	}

	// Create test log context
	logsContext := `--- STABLE LOGS ---
2024-10-01 10:00:00 INFO  Application started successfully
2024-10-01 10:00:01 INFO  Processed 100 requests
2024-10-01 10:00:02 INFO  Average response time: 50ms
2024-10-01 10:00:03 INFO  No errors detected
2024-10-01 10:00:04 INFO  Health check: OK

--- CANARY LOGS ---
2024-10-01 10:00:00 INFO  Application started successfully
2024-10-01 10:00:01 INFO  Processed 100 requests
2024-10-01 10:00:02 INFO  Average response time: 52ms
2024-10-01 10:00:03 INFO  No errors detected
2024-10-01 10:00:04 INFO  Health check: OK`

	modelName := "gemini-2.0-flash-exp"

	// Set the global API key (since getSecretValue reads from globals)
	oldAPIKey := googleAPIKey
	googleAPIKey = apiKey
	defer func() { googleAPIKey = oldAPIKey }()

	// Call the real function
	t.Log("Calling real Google Gemini API...")
	params := AIAnalysisParams{
		ModelName:   modelName,
		LogsContext: logsContext,
		ExtraPrompt: "",
	}
	rawJSON, result, err := analyzeLogsWithAI(params)
	for i := 0; i < 5; i++ {
		rawJSON, result, err = analyzeLogsWithAI(params)
	}

	// Verify results
	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}

	t.Logf("Raw JSON response: %s", rawJSON)
	t.Logf("Promote decision: %v", result.Promote)
	t.Logf("Analysis text: %s", result.Text)
	t.Logf("Confidence: %d", result.Confidence)

	// Verify response structure
	if rawJSON == "" {
		t.Error("Expected non-empty JSON response")
	}

	// Verify JSON can be parsed
	var obj struct {
		Text       string `json:"text"`
		Promote    bool   `json:"promote"`
		Confidence int    `json:"confidence"`
	}
	if parseErr := json.Unmarshal([]byte(rawJSON), &obj); parseErr != nil {
		t.Errorf("Failed to parse JSON response: %v", parseErr)
	} else {
		t.Logf("Parsed - Text: %s, Promote: %v, Confidence: %d", obj.Text, obj.Promote, obj.Confidence)

		// Verify parsed values match returned values
		if obj.Promote != result.Promote {
			t.Errorf("Parsed promote (%v) doesn't match returned promote (%v)", obj.Promote, result.Promote)
		}
		if obj.Text != result.Text {
			t.Errorf("Parsed text (%s) doesn't match returned text (%s)", obj.Text, result.Text)
		}
		if obj.Confidence != result.Confidence {
			t.Errorf("Parsed confidence (%d) doesn't match returned confidence (%d)", obj.Confidence, result.Confidence)
		}

		// Verify confidence is in valid range
		if obj.Confidence < 0 || obj.Confidence > 100 {
			t.Errorf("Confidence %d is out of valid range [0, 100]", obj.Confidence)
		}
	}

	// Verify analysis text is not empty
	if result.Text == "" {
		t.Error("Expected non-empty analysis text")
	}

}

// TestAnalyzeLogsWithAI_Integration_ErrorHandling tests error scenarios with real API
func TestAnalyzeLogsWithAI_Integration_ErrorHandling(t *testing.T) {
	// Skip if not explicitly requested
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: GOOGLE_API_KEY not set")
	}

	// Test with invalid model name
	t.Run("invalid_model", func(t *testing.T) {
		oldAPIKey := googleAPIKey
		googleAPIKey = apiKey
		defer func() { googleAPIKey = oldAPIKey }()

		logsContext := "test logs"
		params := AIAnalysisParams{
			ModelName:   "invalid-model-name-12345",
			LogsContext: logsContext,
			ExtraPrompt: "",
		}
		_, _, err := analyzeLogsWithAI(params)

		if err == nil {
			t.Error("Expected error with invalid model name")
		} else {
			t.Logf("Got expected error: %v", err)
		}
	})

	// Test with empty logs
	t.Run("empty_logs", func(t *testing.T) {
		oldAPIKey := googleAPIKey
		googleAPIKey = apiKey
		defer func() { googleAPIKey = oldAPIKey }()

		params := AIAnalysisParams{
			ModelName:   "gemini-2.0-flash-exp",
			LogsContext: "",
			ExtraPrompt: "",
		}
		_, result, err := analyzeLogsWithAI(params)

		// Should still work but might default to promote:true
		if err != nil {
			t.Logf("Error with empty logs (may be expected): %v", err)
		} else {
			t.Logf("Empty logs result - promote: %v", result.Promote)
			// Per the prompt, should default to promote:true when lacking information
			if !result.Promote {
				t.Log("Note: Empty logs resulted in promote:false (may want to verify this behavior)")
			}
		}
	})
}
