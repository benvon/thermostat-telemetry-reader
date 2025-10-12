package temperature

import (
	"fmt"
)

// Unit represents a temperature unit
type Unit string

const (
	Celsius    Unit = "celsius"
	Fahrenheit Unit = "fahrenheit"
	Kelvin     Unit = "kelvin"
)

// Scale represents how the temperature value is scaled
type Scale float64

const (
	ScaleNone       Scale = 1.0   // No scaling (e.g., 72.5°F)
	ScaleTenths     Scale = 10.0  // Tenths (e.g., 725 = 72.5°F)
	ScaleHundredths Scale = 100.0 // Hundredths (e.g., 7250 = 72.5°F)
)

// Format describes how a temperature value is formatted
type Format struct {
	Unit  Unit
	Scale Scale
}

// Common temperature formats used by different providers
var (
	// EcobeeFormat - tenths of degrees Fahrenheit
	EcobeeFormat = Format{Unit: Fahrenheit, Scale: ScaleTenths}

	// StandardCelsius - standard Celsius format
	StandardCelsius = Format{Unit: Celsius, Scale: ScaleNone}

	// StandardFahrenheit - standard Fahrenheit format
	StandardFahrenheit = Format{Unit: Fahrenheit, Scale: ScaleNone}
)

// Converter handles temperature conversions between different formats
type Converter struct {
	sourceFormat Format
	targetFormat Format
}

// NewConverter creates a new temperature converter
func NewConverter(sourceFormat, targetFormat Format) *Converter {
	return &Converter{
		sourceFormat: sourceFormat,
		targetFormat: targetFormat,
	}
}

// Convert converts a temperature value from source format to target format
func (c *Converter) Convert(temp *float64) (*float64, error) {
	if temp == nil {
		return nil, nil
	}

	// First, unscale the source value
	unscaledTemp := *temp / float64(c.sourceFormat.Scale)

	// Convert to Celsius as intermediate format
	var tempC float64
	switch c.sourceFormat.Unit {
	case Celsius:
		tempC = unscaledTemp
	case Fahrenheit:
		tempC = (unscaledTemp - 32.0) * 5.0 / 9.0
	case Kelvin:
		tempC = unscaledTemp - 273.15
	default:
		return nil, fmt.Errorf("unsupported source temperature unit: %s", c.sourceFormat.Unit)
	}

	// Convert from Celsius to target unit
	var targetTemp float64
	switch c.targetFormat.Unit {
	case Celsius:
		targetTemp = tempC
	case Fahrenheit:
		targetTemp = tempC*9.0/5.0 + 32.0
	case Kelvin:
		targetTemp = tempC + 273.15
	default:
		return nil, fmt.Errorf("unsupported target temperature unit: %s", c.targetFormat.Unit)
	}

	// Apply target scaling
	scaledTemp := targetTemp * float64(c.targetFormat.Scale)

	return &scaledTemp, nil
}

// ConvertToCelsius is a convenience function to convert any temperature format to Celsius
func ConvertToCelsius(temp *float64, sourceFormat Format) (*float64, error) {
	converter := NewConverter(sourceFormat, StandardCelsius)
	return converter.Convert(temp)
}

// ConvertFromEcobeeToCelsius is a convenience function specifically for Ecobee temperatures
func ConvertFromEcobeeToCelsius(temp *float64) (*float64, error) {
	return ConvertToCelsius(temp, EcobeeFormat)
}
