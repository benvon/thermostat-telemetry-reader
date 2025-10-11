package model

import (
	"testing"
)

func TestToEquipmentMap(t *testing.T) {
	tests := []struct {
		name     string
		state    EquipmentState
		expected map[string]bool
	}{
		{
			name: "all equipment active",
			state: EquipmentState{
				CompHeat1: true,
				CompHeat2: true,
				CompCool1: true,
				CompCool2: true,
				Fan:       true,
			},
			expected: map[string]bool{
				"compHeat1": true,
				"compHeat2": true,
				"compCool1": true,
				"compCool2": true,
				"fan":       true,
			},
		},
		{
			name: "no equipment active",
			state: EquipmentState{
				CompHeat1: false,
				CompHeat2: false,
				CompCool1: false,
				CompCool2: false,
				Fan:       false,
			},
			expected: map[string]bool{},
		},
		{
			name: "only heating active",
			state: EquipmentState{
				CompHeat1: true,
				CompHeat2: false,
				CompCool1: false,
				CompCool2: false,
				Fan:       true,
			},
			expected: map[string]bool{
				"compHeat1": true,
				"fan":       true,
			},
		},
		{
			name: "only cooling active",
			state: EquipmentState{
				CompHeat1: false,
				CompHeat2: false,
				CompCool1: true,
				CompCool2: false,
				Fan:       true,
			},
			expected: map[string]bool{
				"compCool1": true,
				"fan":       true,
			},
		},
		{
			name: "dual stage heating",
			state: EquipmentState{
				CompHeat1: true,
				CompHeat2: true,
				CompCool1: false,
				CompCool2: false,
				Fan:       true,
			},
			expected: map[string]bool{
				"compHeat1": true,
				"compHeat2": true,
				"fan":       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.state.ToEquipmentMap()

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d entries, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				resultValue, exists := result[key]
				if !exists {
					t.Errorf("Expected key %s to exist", key)
				} else if resultValue != expectedValue {
					t.Errorf("Key %s: expected %v, got %v", key, expectedValue, resultValue)
				}
			}

			// Verify no unexpected keys
			for key := range result {
				if _, expected := tt.expected[key]; !expected {
					t.Errorf("Unexpected key %s in result", key)
				}
			}
		})
	}
}

func TestFromEquipmentMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]bool
		expected EquipmentState
	}{
		{
			name: "all equipment active",
			input: map[string]bool{
				"compHeat1": true,
				"compHeat2": true,
				"compCool1": true,
				"compCool2": true,
				"fan":       true,
			},
			expected: EquipmentState{
				CompHeat1: true,
				CompHeat2: true,
				CompCool1: true,
				CompCool2: true,
				Fan:       true,
			},
		},
		{
			name:  "empty map",
			input: map[string]bool{},
			expected: EquipmentState{
				CompHeat1: false,
				CompHeat2: false,
				CompCool1: false,
				CompCool2: false,
				Fan:       false,
			},
		},
		{
			name: "partial equipment",
			input: map[string]bool{
				"compHeat1": true,
				"fan":       true,
			},
			expected: EquipmentState{
				CompHeat1: true,
				CompHeat2: false,
				CompCool1: false,
				CompCool2: false,
				Fan:       true,
			},
		},
		{
			name: "unknown keys ignored",
			input: map[string]bool{
				"compHeat1":  true,
				"unknownKey": true,
				"anotherKey": false,
			},
			expected: EquipmentState{
				CompHeat1: true,
				CompHeat2: false,
				CompCool1: false,
				CompCool2: false,
				Fan:       false,
			},
		},
		{
			name:  "nil map",
			input: nil,
			expected: EquipmentState{
				CompHeat1: false,
				CompHeat2: false,
				CompCool1: false,
				CompCool2: false,
				Fan:       false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromEquipmentMap(tt.input)

			if result.CompHeat1 != tt.expected.CompHeat1 {
				t.Errorf("CompHeat1: expected %v, got %v", tt.expected.CompHeat1, result.CompHeat1)
			}
			if result.CompHeat2 != tt.expected.CompHeat2 {
				t.Errorf("CompHeat2: expected %v, got %v", tt.expected.CompHeat2, result.CompHeat2)
			}
			if result.CompCool1 != tt.expected.CompCool1 {
				t.Errorf("CompCool1: expected %v, got %v", tt.expected.CompCool1, result.CompCool1)
			}
			if result.CompCool2 != tt.expected.CompCool2 {
				t.Errorf("CompCool2: expected %v, got %v", tt.expected.CompCool2, result.CompCool2)
			}
			if result.Fan != tt.expected.Fan {
				t.Errorf("Fan: expected %v, got %v", tt.expected.Fan, result.Fan)
			}
		})
	}
}

func TestEquipmentMapRoundTrip(t *testing.T) {
	// Test that converting to map and back preserves the data
	original := EquipmentState{
		CompHeat1: true,
		CompHeat2: false,
		CompCool1: true,
		CompCool2: false,
		Fan:       true,
	}

	equipMap := original.ToEquipmentMap()
	roundTrip := FromEquipmentMap(equipMap)

	if roundTrip != original {
		t.Errorf("Round trip failed. Original: %+v, RoundTrip: %+v", original, roundTrip)
	}
}
