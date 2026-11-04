// This program is a minimal diagnostics adapter used as executable contract
// documentation. It reads exactly one request and writes exactly one response.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"dev-tools/internal/devresult"
)

func main() {
	decoder := json.NewDecoder(os.Stdin)
	decoder.DisallowUnknownFields()
	var request devresult.DiagnosticsRequest
	if err := decoder.Decode(&request); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "decode request: %v\n", err)
		os.Exit(2)
	}
	response := devresult.DiagnosticsResponse{
		Schema:        devresult.DiagnosticsResponseSchemaV1,
		EvidenceState: devresult.EvidenceComplete,
	}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "encode response: %v\n", err)
		os.Exit(2)
	}
}
