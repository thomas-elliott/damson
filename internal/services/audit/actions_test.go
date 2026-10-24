package audit

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

// The SPA's label and chip tests read this copy, so a type added here
// without updating it would escape them.
func TestActionTypesMatchWebCopy(t *testing.T) {
	const path = "../../../web/src/lib/auditActionTypes.json"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var web []string
	if err := json.Unmarshal(b, &web); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if !slices.Equal(web, ActionTypes) {
		want, _ := json.MarshalIndent(ActionTypes, "", "  ")
		t.Fatalf("%s is out of step with audit.ActionTypes; replace it with:\n%s", path, want)
	}
}

func TestWriteRefusesUnknownActionType(t *testing.T) {
	// Refused before the tx is touched, so no database is needed.
	err := Write(nil, Entry{ActionType: "asset.frobnicate", TargetType: "asset"})
	if err == nil || !strings.Contains(err.Error(), "unknown action_type") {
		t.Fatalf("want unknown action_type error, got %v", err)
	}
}
