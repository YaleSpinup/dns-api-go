package models

import (
	"dns-api-go/internal/common"
	"testing"
)

func TestToBluecatJSON(t *testing.T) {
	tests := []struct {
		name           string
		entity         Entity
		expectedOutput string
		expectError    bool
	}{
		{
			name: "Successful conversion",
			entity: Entity{
				ID:   1,
				Name: "Test Entity",
				Type: "HostRecord",
				Properties: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expectedOutput: `{"id":1,"name":"Test Entity","type":"HostRecord","properties":"key1=value1|key2=value2"}`,
			expectError:    false,
		},
		{
			name: "Empty properties",
			entity: Entity{
				ID:         2,
				Name:       "Empty Properties Entity",
				Type:       "HostRecord",
				Properties: map[string]string{},
			},
			expectedOutput: `{"id":2,"name":"Empty Properties Entity","type":"HostRecord","properties":""}`,
			expectError:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, err := tc.entity.ToBluecatJSON()
			common.CheckError(t, tc.name, nil, err)
			common.CheckResponse(t, tc.name, tc.expectedOutput, string(output))
		})
	}
}
