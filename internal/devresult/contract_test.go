package devresult

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiagnosticsContractDefaultsAndValidation(t *testing.T) {
	limits := DefaultAdapterLimits()
	assert.Equal(t, 2*time.Second, limits.Timeout)
	assert.EqualValues(t, 1<<20, limits.StdoutBytes)
	assert.EqualValues(t, 64<<10, limits.StderrBytes)
	assert.Equal(t, 1000, limits.MaxDiagnostics)
	assert.EqualValues(t, 10<<20, limits.PerSourceBytes)
	assert.EqualValues(t, 50<<20, limits.TotalSourceBytes)

	response := DiagnosticsResponse{
		Schema:        DiagnosticsResponseSchemaV1,
		EvidenceState: EvidenceComplete,
		Failures: []Diagnostic{{
			Identity: "checkout/rejects-expired-card",
			Message:  "Expected the expired card to be rejected",
			Package:  "checkout",
			File:     "checkout/card_test.go",
			Line:     73,
			Suite:    "Card validation",
			Test:     "rejects an expired card",
		}},
		Sources: []DiagnosticSource{{Kind: "junit", Path: "junit/checkout.xml"}},
	}
	require.NoError(t, ValidateDiagnosticsResponse(response, limits))

	response.Schema = "dev-tools/diagnostics-response/v2"
	assert.ErrorContains(t, ValidateDiagnosticsResponse(response, limits), "unsupported diagnostics response schema")
	response.Schema = DiagnosticsResponseSchemaV1
	response.EvidenceState = "mystery"
	assert.ErrorContains(t, ValidateDiagnosticsResponse(response, limits), "evidence state")
	response.EvidenceState = EvidenceComplete
	response.Failures[0].Identity = ""
	assert.ErrorContains(t, ValidateDiagnosticsResponse(response, limits), "identity")
}

func TestDiagnosticsPublishedFixturesConform(t *testing.T) {
	requestData, err := os.ReadFile(filepath.Join("testdata", "diagnostics-v1", "request.json"))
	require.NoError(t, err)
	var request DiagnosticsRequest
	require.NoError(t, json.Unmarshal(requestData, &request))
	assert.Equal(t, DiagnosticsRequestSchemaV1, request.Schema)
	assert.Equal(t, "test-016", request.RunID)

	responseData, err := os.ReadFile(filepath.Join("testdata", "diagnostics-v1", "response.json"))
	require.NoError(t, err)
	var response DiagnosticsResponse
	require.NoError(t, json.Unmarshal(responseData, &response))
	require.NoError(t, ValidateDiagnosticsResponse(response, DefaultAdapterLimits()))
	assert.Equal(t, "checkout/rejects-expired-card", response.Failures[0].Identity)
}

func TestDiagnosticsContractRejectsExcessRecordsAndInvalidStates(t *testing.T) {
	limits := DefaultAdapterLimits()
	limits.MaxDiagnostics = 2
	response := DiagnosticsResponse{
		Schema:        DiagnosticsResponseSchemaV1,
		EvidenceState: EvidencePartial,
		Failures:      []Diagnostic{{Identity: "one", Message: "one"}, {Identity: "two", Message: "two"}},
		Warnings:      []Diagnostic{{Identity: "three", Message: "three"}},
		Problems:      []EvidenceProblem{{Message: "billing.xml was not produced"}},
	}
	assert.ErrorContains(t, ValidateDiagnosticsResponse(response, limits), "diagnostic record limit")

	response.Warnings = nil
	require.NoError(t, ValidateDiagnosticsResponse(response, limits))
	response.EvidenceState = EvidenceUnavailable
	assert.ErrorContains(t, ValidateDiagnosticsResponse(response, limits), "unavailable evidence cannot contain diagnostics")
}
