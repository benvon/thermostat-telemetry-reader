package ecobee

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/benvon/thermostat-telemetry-reader/pkg/model"
	"github.com/benvon/thermostat-telemetry-reader/pkg/temperature"
)

const (
	ecobeeRuntimeDateFormat = "2006-01-02"
	errMsgMarshalSelection  = "marshaling selection: %w"
)

// Provider implements the Ecobee thermostat provider
type Provider struct {
	authManager *AuthManager
}

// NewProvider creates a new Ecobee provider
func NewProvider(clientID, refreshToken string) *Provider {
	return &Provider{
		authManager: NewAuthManager(clientID, refreshToken),
	}
}

// Info returns metadata about the provider
func (p *Provider) Info() model.ProviderInfo {
	return model.ProviderInfo{
		Name:        "ecobee",
		Version:     "1.0.0",
		Description: "Ecobee thermostat provider with smartRead scope",
	}
}

// ListThermostats returns all thermostats available to this provider
func (p *Provider) ListThermostats(ctx context.Context) ([]model.ThermostatRef, error) {
	resp, err := p.authManager.makeAuthenticatedRequest(ctx, "/thermostat", map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("requesting thermostats: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var result struct {
		ThermostatList []struct {
			Identifier string `json:"identifier"`
			Name       string `json:"name"`
			HouseID    string `json:"houseId"`
		} `json:"thermostatList"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding thermostats response: %w", err)
	}

	var thermostats []model.ThermostatRef
	for _, t := range result.ThermostatList {
		thermostats = append(thermostats, model.ThermostatRef{
			ID:          t.Identifier,
			Name:        t.Name,
			Provider:    "ecobee",
			HouseholdID: t.HouseID,
		})
	}

	return thermostats, nil
}

// GetSummary returns high-level information for change detection
func (p *Provider) GetSummary(ctx context.Context, tr model.ThermostatRef) (model.Summary, error) {
	selection := NewSummarySelection(tr.ID)
	selectionJSON, err := json.Marshal(selection)
	if err != nil {
		return model.Summary{}, fmt.Errorf(errMsgMarshalSelection, err)
	}

	resp, err := p.authManager.makeAuthenticatedRequest(ctx, "/thermostatSummary", map[string]string{
		"selection": string(selectionJSON),
	})
	if err != nil {
		return model.Summary{}, fmt.Errorf("requesting thermostat summary: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var result struct {
		Revision        string `json:"revision"`
		ThermostatCount int    `json:"thermostatCount"`
		StatusList      []struct {
			ThermostatIdentifier string `json:"thermostatIdentifier"`
			Connected            bool   `json:"connected"`
			ThermostatRevision   string `json:"thermostatRevision"`
			AlertsRevision       string `json:"alertsRevision"`
			RuntimeRevision      string `json:"runtimeRevision"`
			IntervalRevision     string `json:"intervalRevision"`
		} `json:"statusList"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return model.Summary{}, fmt.Errorf("decoding summary response: %w", err)
	}

	// Find the specific thermostat
	for _, status := range result.StatusList {
		if status.ThermostatIdentifier == tr.ID {
			return model.Summary{
				ThermostatRef: tr,
				Revision:      status.ThermostatRevision,
				LastUpdate:    time.Now(),
			}, nil
		}
	}

	return model.Summary{}, fmt.Errorf("thermostat %s not found in summary", tr.ID)
}

// GetSnapshot returns current thermostat state
func (p *Provider) GetSnapshot(ctx context.Context, tr model.ThermostatRef, since time.Time) (model.Snapshot, error) {
	selection := NewSnapshotSelection(tr.ID)
	selectionJSON, err := json.Marshal(selection)
	if err != nil {
		return model.Snapshot{}, fmt.Errorf(errMsgMarshalSelection, err)
	}

	resp, err := p.authManager.makeAuthenticatedRequest(ctx, "/thermostat", map[string]string{
		"selection": string(selectionJSON),
	})
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("requesting thermostat snapshot: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var result struct {
		ThermostatList []struct {
			Identifier string `json:"identifier"`
			Name       string `json:"name"`
			Runtime    any    `json:"runtime,omitempty"`
			Events     []any  `json:"events,omitempty"`
			Program    any    `json:"program,omitempty"`
		} `json:"thermostatList"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return model.Snapshot{}, fmt.Errorf("decoding snapshot response: %w", err)
	}

	// Find the specific thermostat
	for _, t := range result.ThermostatList {
		if t.Identifier == tr.ID {
			return model.Snapshot{
				ThermostatRef: tr,
				CollectedAt:   time.Now(),
				Program:       t.Program,
				EventsActive:  t.Events,
			}, nil
		}
	}

	return model.Snapshot{}, fmt.Errorf("thermostat %s not found in snapshot", tr.ID)
}

// GetRuntime returns historical runtime data for the specified time range
func (p *Provider) GetRuntime(ctx context.Context, tr model.ThermostatRef, from, to time.Time) ([]model.RuntimeRow, error) {
	// Format dates for Ecobee API (YYYY-MM-DD)
	startDate := from.Format(ecobeeRuntimeDateFormat)
	endDate := to.Format(ecobeeRuntimeDateFormat)

	selection := NewThermostatSelection(tr.ID)
	selectionJSON, err := json.Marshal(selection)
	if err != nil {
		return nil, fmt.Errorf(errMsgMarshalSelection, err)
	}

	params := map[string]string{
		"startDate": startDate,
		"endDate":   endDate,
		"columns":   "zoneHeatTemp,zoneCoolTemp,zoneAveTemp,outdoorTemp,outdoorHumidity,compHeat1,compHeat2,compCool1,compCool2,fan,hvacMode,zoneClimateRef",
		"selection": string(selectionJSON),
	}

	resp, err := p.authManager.makeAuthenticatedRequest(ctx, "/runtimeReport", params)
	if err != nil {
		return nil, fmt.Errorf("requesting runtime report: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var result struct {
		ReportList []struct {
			ThermostatIdentifier string `json:"thermostatIdentifier"`
			Columns              string `json:"columns"`
			Data                 []struct {
				Date string   `json:"date"`
				Zone string   `json:"zone"`
				Data []string `json:"data"`
			} `json:"data"`
		} `json:"reportList"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding runtime report response: %w", err)
	}

	var runtimeRows []model.RuntimeRow

	// Parse the runtime data
	for _, report := range result.ReportList {
		if report.ThermostatIdentifier != tr.ID {
			continue
		}

		// Parse column headers
		columns := parseColumns(report.Columns)

		for _, dataRow := range report.Data {
			row := model.RuntimeRow{
				ThermostatRef: tr,
			}

			// Parse date and time
			date, err := time.Parse(ecobeeRuntimeDateFormat, dataRow.Date)
			if err != nil {
				continue // Skip invalid dates
			}

			// Ecobee provides 5-minute intervals, so we need to calculate the time
			// This is a simplified parsing - in reality you'd need to handle the zone and time components
			row.EventTime = date

			// Parse data values based on column positions
			for i, value := range dataRow.Data {
				if i >= len(columns) {
					break
				}

				switch columns[i] {
				case "zoneHeatTemp":
					if temp := parseFloat(value); temp != nil {
						// Convert from Ecobee format (tenths of Fahrenheit) to Celsius
						if convertedTemp, err := temperature.ConvertFromEcobeeToCelsius(temp); err == nil {
							row.SetHeatC = convertedTemp
						}
					}
				case "zoneCoolTemp":
					if temp := parseFloat(value); temp != nil {
						// Convert from Ecobee format (tenths of Fahrenheit) to Celsius
						if convertedTemp, err := temperature.ConvertFromEcobeeToCelsius(temp); err == nil {
							row.SetCoolC = convertedTemp
						}
					}
				case "zoneAveTemp":
					if temp := parseFloat(value); temp != nil {
						// Convert from Ecobee format (tenths of Fahrenheit) to Celsius
						if convertedTemp, err := temperature.ConvertFromEcobeeToCelsius(temp); err == nil {
							row.AvgTempC = convertedTemp
						}
					}
				case "outdoorTemp":
					if temp := parseFloat(value); temp != nil {
						// Convert from Ecobee format (tenths of Fahrenheit) to Celsius
						if convertedTemp, err := temperature.ConvertFromEcobeeToCelsius(temp); err == nil {
							row.OutdoorTempC = convertedTemp
						}
					}
				case "outdoorHumidity":
					if humidity := parseInt(value); humidity != nil {
						row.OutdoorHumidity = humidity
					}
				case "hvacMode":
					row.Mode = value
				case "zoneClimateRef":
					row.Climate = value
				case "compHeat1", "compHeat2", "compCool1", "compCool2", "fan":
					if row.Equipment == nil {
						row.Equipment = make(map[string]bool)
					}
					row.Equipment[columns[i]] = value == "1" || value == "true"
				}
			}

			runtimeRows = append(runtimeRows, row)
		}
	}

	return runtimeRows, nil
}

// Auth returns the authentication manager for this provider
func (p *Provider) Auth() model.AuthManager {
	return p.authManager
}

// parseColumns parses the column header string from Ecobee
// The Ecobee API returns a comma-separated string like "zoneHeatTemp,zoneCoolTemp,..."
func parseColumns(columnStr string) []string {
	if columnStr == "" {
		return []string{}
	}

	// Split by comma and trim whitespace
	columns := strings.Split(columnStr, ",")
	for i, col := range columns {
		columns[i] = strings.TrimSpace(col)
	}

	return columns
}

// parseFloat parses a string to float64, returning nil if parsing fails
func parseFloat(s string) *float64 {
	if s == "" {
		return nil
	}

	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}

	return &value
}

// parseInt parses a string to int, returning nil if parsing fails
func parseInt(s string) *int {
	if s == "" {
		return nil
	}

	value, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}

	return &value
}
