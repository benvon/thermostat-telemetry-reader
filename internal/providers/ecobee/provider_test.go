package ecobee

import (
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

// Helper functions
func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
