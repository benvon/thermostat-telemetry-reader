package model

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
)

var errNilDocument = errors.New("document is nil")

const (
	// timestampFormat is the standard timestamp format used for document IDs
	timestampFormat = "2006-01-02T15:04:05Z"
)

// IDGenerator implements deterministic document ID generation
// IDs are generated according to requirements:
//   - runtime_5m: thermostat_id:event_time:type:hash(body)
//   - transition: thermostat_id:event_time:hash(prev,next)
//   - device_snapshot: thermostat_id:collected_at
type IDGenerator struct{}

// NewIDGenerator creates a new ID generator
func NewIDGenerator() DocumentIDGenerator {
	return &IDGenerator{}
}

// GenerateRuntime5mID generates a deterministic ID for runtime_5m documents
// Format: thermostat_id:event_time:type:hash(body)
func (g *IDGenerator) GenerateRuntime5mID(doc *Runtime5m) (string, error) {
	if doc == nil {
		return "", errNilDocument
	}

	eventTimeStr := doc.EventTime.Format(timestampFormat)
	bodyHash, err := g.hashDocument(doc)
	if err != nil {
		return "", fmt.Errorf("hashing runtime document: %w", err)
	}
	return fmt.Sprintf("%s:%s:%s:%s", doc.ThermostatID, eventTimeStr, doc.Type, bodyHash), nil
}

// GenerateTransitionID generates a deterministic ID for transition documents
// Format: thermostat_id:event_time:hash(prev,next)
func (g *IDGenerator) GenerateTransitionID(doc *Transition) (string, error) {
	if doc == nil {
		return "", errNilDocument
	}

	eventTimeStr := doc.EventTime.Format(timestampFormat)
	prevNextHash, err := g.hashTransition(doc.Prev, doc.Next)
	if err != nil {
		return "", fmt.Errorf("hashing transition: %w", err)
	}
	return fmt.Sprintf("%s:%s:%s", doc.ThermostatID, eventTimeStr, prevNextHash), nil
}

// GenerateDeviceSnapshotID generates a deterministic ID for device_snapshot documents
// Format: thermostat_id:collected_at
func (g *IDGenerator) GenerateDeviceSnapshotID(doc *DeviceSnapshot) (string, error) {
	if doc == nil {
		return "", errNilDocument
	}

	collectedAtStr := doc.CollectedAt.Format(timestampFormat)
	return fmt.Sprintf("%s:%s", doc.ThermostatID, collectedAtStr), nil
}

// hashDocument creates a hash of the document body
func (g *IDGenerator) hashDocument(doc any) (string, error) {
	docBytes, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshaling document for hash: %w", err)
	}
	hash := sha256.Sum256(docBytes)
	return fmt.Sprintf("%x", hash)[:16], nil // Use first 16 characters
}

// hashTransition creates a hash of the previous and next states
func (g *IDGenerator) hashTransition(prev, next State) (string, error) {
	transition := map[string]any{
		"prev": prev,
		"next": next,
	}
	transitionBytes, err := json.Marshal(transition)
	if err != nil {
		return "", fmt.Errorf("marshaling transition for hash: %w", err)
	}
	hash := sha256.Sum256(transitionBytes)
	return fmt.Sprintf("%x", hash)[:16], nil // Use first 16 characters
}
