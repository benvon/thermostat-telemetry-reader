package temperature

import (
	"fmt"
	"testing"
)

func TestConverterConvert(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		sourceFormat Format
		targetFormat Format
		input        *float64
		want         *float64
		wantErr      bool
	}{
		{
			name:         "nil input returns nil",
			sourceFormat: EcobeeFormat,
			targetFormat: StandardCelsius,
			input:        nil,
			want:         nil,
			wantErr:      false,
		},
		{
			name:         "Ecobee format to Celsius - 72°F (720 tenths)",
			sourceFormat: EcobeeFormat,
			targetFormat: StandardCelsius,
			input:        floatPtr(720.0),
			want:         floatPtr(22.222222222222218), // 72°F = ~22.22°C
			wantErr:      false,
		},
		{
			name:         "Ecobee format to Celsius - 32°F (320 tenths)",
			sourceFormat: EcobeeFormat,
			targetFormat: StandardCelsius,
			input:        floatPtr(320.0),
			want:         floatPtr(0.0), // 32°F = 0°C
			wantErr:      false,
		},
		{
			name:         "Standard Celsius to Celsius - no conversion needed",
			sourceFormat: StandardCelsius,
			targetFormat: StandardCelsius,
			input:        floatPtr(25.0),
			want:         floatPtr(25.0),
			wantErr:      false,
		},
		{
			name:         "Standard Fahrenheit to Celsius",
			sourceFormat: StandardFahrenheit,
			targetFormat: StandardCelsius,
			input:        floatPtr(72.0),
			want:         floatPtr(22.222222222222218),
			wantErr:      false,
		},
		{
			name:         "Celsius to Fahrenheit",
			sourceFormat: StandardCelsius,
			targetFormat: StandardFahrenheit,
			input:        floatPtr(0.0),
			want:         floatPtr(32.0),
			wantErr:      false,
		},
		{
			name:         "Kelvin to Celsius",
			sourceFormat: Format{Unit: Kelvin, Scale: ScaleNone},
			targetFormat: StandardCelsius,
			input:        floatPtr(273.15),
			want:         floatPtr(0.0),
			wantErr:      false,
		},
		{
			name:         "Celsius to Kelvin",
			sourceFormat: StandardCelsius,
			targetFormat: Format{Unit: Kelvin, Scale: ScaleNone},
			input:        floatPtr(0.0),
			want:         floatPtr(273.15),
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			converter := NewConverter(tt.sourceFormat, tt.targetFormat)
			got, err := converter.Convert(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("Convert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !floatPtrEqual(got, tt.want) {
				t.Errorf("Convert() = %v, want %v", ptrToString(got), ptrToString(tt.want))
			}
		})
	}
}

func TestConvertToCelsius(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		temp         *float64
		sourceFormat Format
		want         *float64
		wantErr      bool
	}{
		{
			name:         "nil input",
			temp:         nil,
			sourceFormat: EcobeeFormat,
			want:         nil,
			wantErr:      false,
		},
		{
			name:         "Ecobee format conversion",
			temp:         floatPtr(720.0), // 72°F in tenths
			sourceFormat: EcobeeFormat,
			want:         floatPtr(22.222222222222218),
			wantErr:      false,
		},
		{
			name:         "Already Celsius",
			temp:         floatPtr(25.0),
			sourceFormat: StandardCelsius,
			want:         floatPtr(25.0),
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertToCelsius(tt.temp, tt.sourceFormat)

			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertToCelsius() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !floatPtrEqual(got, tt.want) {
				t.Errorf("ConvertToCelsius() = %v, want %v", ptrToString(got), ptrToString(tt.want))
			}
		})
	}
}

func TestConvertFromEcobeeToCelsius(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		temp    *float64
		want    *float64
		wantErr bool
	}{
		{
			name:    "nil input",
			temp:    nil,
			want:    nil,
			wantErr: false,
		},
		{
			name:    "freezing point - 32°F (320 tenths)",
			temp:    floatPtr(320.0),
			want:    floatPtr(0.0),
			wantErr: false,
		},
		{
			name:    "room temperature - 72°F (720 tenths)",
			temp:    floatPtr(720.0),
			want:    floatPtr(22.222222222222218),
			wantErr: false,
		},
		{
			name:    "negative temperature - -10°F (-100 tenths)",
			temp:    floatPtr(-100.0),
			want:    floatPtr(-23.333333333333332),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertFromEcobeeToCelsius(tt.temp)

			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertFromEcobeeToCelsius() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !floatPtrEqual(got, tt.want) {
				t.Errorf("ConvertFromEcobeeToCelsius() = %v, want %v", ptrToString(got), ptrToString(tt.want))
			}
		})
	}
}

// Helper functions for testing

func floatPtr(f float64) *float64 {
	return &f
}

func floatPtrEqual(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Use a small epsilon for floating point comparison
	const epsilon = 1e-9
	diff := *a - *b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

func ptrToString(f *float64) string {
	if f == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%f", *f)
}
