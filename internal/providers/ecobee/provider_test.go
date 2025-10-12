package ecobee

import (
	"encoding/json"
	"testing"
)

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *float64
	}{
		{
			name:     "valid float",
			input:    "123.45",
			expected: floatPtr(123.45),
		},
		{
			name:     "valid integer",
			input:    "100",
			expected: floatPtr(100.0),
		},
		{
			name:     "negative float",
			input:    "-25.5",
			expected: floatPtr(-25.5),
		},
		{
			name:     "zero",
			input:    "0",
			expected: floatPtr(0.0),
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "invalid string",
			input:    "not-a-number",
			expected: nil,
		},
		{
			name:     "temperature in tenths",
			input:    "720",
			expected: floatPtr(720.0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFloat(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", *result)
				}
			} else {
				if result == nil {
					t.Errorf("Expected %f, got nil", *tt.expected)
				} else if *result != *tt.expected {
					t.Errorf("Expected %f, got %f", *tt.expected, *result)
				}
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *int
	}{
		{
			name:     "valid int",
			input:    "42",
			expected: intPtr(42),
		},
		{
			name:     "zero",
			input:    "0",
			expected: intPtr(0),
		},
		{
			name:     "negative int",
			input:    "-10",
			expected: intPtr(-10),
		},
		{
			name:     "humidity value",
			input:    "65",
			expected: intPtr(65),
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "invalid string",
			input:    "abc",
			expected: nil,
		},
		{
			name:     "float string",
			input:    "12.5",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseInt(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", *result)
				}
			} else {
				if result == nil {
					t.Errorf("Expected %d, got nil", *tt.expected)
				} else if *result != *tt.expected {
					t.Errorf("Expected %d, got %d", *tt.expected, *result)
				}
			}
		})
	}
}

func TestParseColumns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "standard columns",
			input:    "zoneHeatTemp,zoneCoolTemp,zoneAveTemp",
			expected: []string{"zoneHeatTemp", "zoneCoolTemp", "zoneAveTemp"},
		},
		{
			name:     "columns with spaces",
			input:    "col1, col2, col3",
			expected: []string{"col1", "col2", "col3"},
		},
		{
			name:     "single column",
			input:    "temperature",
			expected: []string{"temperature"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:  "full ecobee runtime columns",
			input: "zoneHeatTemp,zoneCoolTemp,zoneAveTemp,outdoorTemp,outdoorHumidity,compHeat1,compHeat2,compCool1,compCool2,fan,hvacMode,zoneClimateRef",
			expected: []string{
				"zoneHeatTemp", "zoneCoolTemp", "zoneAveTemp", "outdoorTemp",
				"outdoorHumidity", "compHeat1", "compHeat2", "compCool1", "compCool2",
				"fan", "hvacMode", "zoneClimateRef",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseColumns(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d columns, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Column %d: expected %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}

func TestNewDefaultSelection(t *testing.T) {
	selection := NewDefaultSelection()

	// Verify the struct fields are set correctly
	if selection.Selection.SelectionType != "registered" {
		t.Errorf("Expected SelectionType 'registered', got %s", selection.Selection.SelectionType)
	}
	if selection.Selection.SelectionMatch != "" {
		t.Errorf("Expected SelectionMatch empty string, got %s", selection.Selection.SelectionMatch)
	}
	if !selection.Selection.IncludeRuntime {
		t.Error("Expected IncludeRuntime to be true")
	}
	if !selection.Selection.IncludeSettings {
		t.Error("Expected IncludeSettings to be true")
	}
	if !selection.Selection.IncludeEvents {
		t.Error("Expected IncludeEvents to be true")
	}
	if !selection.Selection.IncludeProgram {
		t.Error("Expected IncludeProgram to be true")
	}
	if !selection.Selection.IncludeEquipmentStatus {
		t.Error("Expected IncludeEquipmentStatus to be true")
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(selection)
	if err != nil {
		t.Fatalf("Failed to marshal selection to JSON: %v", err)
	}

	// Verify the JSON output matches our expected structure
	expectedJSON := `{"selection":{"selectionType":"registered","selectionMatch":"","includeRuntime":true,"includeSettings":true,"includeEvents":true,"includeProgram":true,"includeEquipmentStatus":true}}`

	if string(jsonData) != expectedJSON {
		t.Errorf("JSON output mismatch.\nExpected: %s\nGot:      %s", expectedJSON, string(jsonData))
	}

	// Test that we can unmarshal it back
	var unmarshaled SelectionRequest
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal JSON back to struct: %v", err)
	}

	// Verify round-trip integrity
	if unmarshaled.Selection.SelectionType != selection.Selection.SelectionType {
		t.Error("Round-trip failed: SelectionType mismatch")
	}
	if unmarshaled.Selection.IncludeRuntime != selection.Selection.IncludeRuntime {
		t.Error("Round-trip failed: IncludeRuntime mismatch")
	}
}

// Helper functions
func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
