package app

import (
	"encoding/json"
	"testing"

	"atlasnote/internal/organize"
)

func TestValidOrganizationRequestID(t *testing.T) {
	for _, testCase := range []struct {
		name string
		id   string
		want bool
	}{
		{name: "uuid", id: "12345678-1234-4abc-9def-123456789abc", want: true},
		{name: "missing", id: "", want: false},
		{name: "long", id: "12345678-1234-4abc-9def-123456789abc-extra", want: false},
		{name: "invalid character", id: "12345678-1234-4abc-9def-123456789abg", want: false},
		{name: "missing separator", id: "123456781234-4abc-9def-123456789abc", want: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := validOrganizationRequestID(testCase.id); got != testCase.want {
				t.Fatalf("validOrganizationRequestID = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestOrganizationProgressPayloadContainsOnlyRequestAndCounts(t *testing.T) {
	data, err := json.Marshal(organizationProgressPayload{
		RequestID: "12345678-1234-4abc-9def-123456789abc",
		AnalysisProgress: organize.AnalysisProgress{
			Phase: "reading", ProcessedNotes: 100, TotalNotes: 200,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 4 || fields["phase"] != "reading" || fields["processedNotes"] != float64(100) || fields["totalNotes"] != float64(200) {
		t.Fatalf("progress fields = %#v", fields)
	}
}
