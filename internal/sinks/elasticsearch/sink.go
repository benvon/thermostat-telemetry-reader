package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/benvon/thermostat-telemetry-reader/pkg/model"
)

// Sink implements the Elasticsearch data sink
type Sink struct {
	client          *http.Client
	url             string
	apiKey          string
	indexPrefix     string
	createTemplates bool
}

// NewSink creates a new Elasticsearch sink
func NewSink(url, apiKey, indexPrefix string, createTemplates bool) *Sink {
	return &Sink{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		url:             url,
		apiKey:          apiKey,
		indexPrefix:     indexPrefix,
		createTemplates: createTemplates,
	}
}

// Info returns metadata about the sink
func (s *Sink) Info() model.SinkInfo {
	return model.SinkInfo{
		Name:        "elasticsearch",
		Version:     "1.0.0",
		Description: "Elasticsearch sink with bulk operations and deterministic IDs",
	}
}

// Open initializes the sink connection and creates index templates if needed
func (s *Sink) Open(ctx context.Context) error {
	if s.createTemplates {
		if err := s.createIndexTemplates(ctx); err != nil {
			return fmt.Errorf("creating index templates: %w", err)
		}
	}
	return nil
}

// Write writes documents to Elasticsearch using bulk operations
func (s *Sink) Write(ctx context.Context, docs []model.Doc) (model.WriteResult, error) {
	if len(docs) == 0 {
		return model.WriteResult{SuccessCount: 0, ErrorCount: 0}, nil
	}

	// Prepare bulk request
	var bulkBody strings.Builder
	for _, doc := range docs {
		// Create index action
		indexAction := map[string]any{
			"index": map[string]any{
				"_index": s.getIndexName(doc.Type),
				"_id":    doc.ID,
			},
		}

		// Serialize index action
		actionBytes, err := json.Marshal(indexAction)
		if err != nil {
			return model.WriteResult{}, fmt.Errorf("marshaling index action: %w", err)
		}
		bulkBody.Write(actionBytes)
		bulkBody.WriteString("\n")

		// Serialize document
		docBytes, err := json.Marshal(doc.Body)
		if err != nil {
			return model.WriteResult{}, fmt.Errorf("marshaling document: %w", err)
		}
		bulkBody.Write(docBytes)
		bulkBody.WriteString("\n")
	}

	// Make bulk request
	req, err := http.NewRequestWithContext(ctx, "POST", s.url+"/_bulk", strings.NewReader(bulkBody.String()))
	if err != nil {
		return model.WriteResult{}, fmt.Errorf("creating bulk request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-ndjson")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return model.WriteResult{}, fmt.Errorf("executing bulk request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Parse response
	var bulkResponse struct {
		Errors bool `json:"errors"`
		Items  []struct {
			Index struct {
				Status int    `json:"status"`
				Error  any    `json:"error"`
				ID     string `json:"_id"`
			} `json:"index"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&bulkResponse); err != nil {
		return model.WriteResult{}, fmt.Errorf("decoding bulk response: %w", err)
	}

	result := model.WriteResult{
		SuccessCount: 0,
		ErrorCount:   0,
		Errors:       []string{},
	}

	// Count successes and errors
	for _, item := range bulkResponse.Items {
		if item.Index.Status >= 200 && item.Index.Status < 300 {
			result.SuccessCount++
		} else {
			result.ErrorCount++
			if item.Index.Error != nil {
				errorBytes, _ := json.Marshal(item.Index.Error)
				result.Errors = append(result.Errors, fmt.Sprintf("ID %s: %s", item.Index.ID, string(errorBytes)))
			}
		}
	}

	return result, nil
}

// Close closes the sink connection
func (s *Sink) Close(ctx context.Context) error {
	// No persistent connections to close for HTTP client
	return nil
}

// getIndexName generates the index name for a document type
func (s *Sink) getIndexName(docType string) string {
	date := time.Now().Format("2006.01.02")
	return fmt.Sprintf("%s-%s-%s", s.indexPrefix, docType, date)
}

// createIndexTemplates creates Elasticsearch index templates for the document types
func (s *Sink) createIndexTemplates(ctx context.Context) error {
	templates := map[string]string{
		"runtime_5m": `
{
	"index_patterns": ["` + s.indexPrefix + `-runtime_5m-*"],
	"template": {
		"mappings": {
			"properties": {
				"type": {"type": "keyword"},
				"thermostat_id": {"type": "keyword"},
				"thermostat_name": {"type": "keyword"},
				"household_id": {"type": "keyword"},
				"event_time": {"type": "date"},
				"mode": {"type": "keyword"},
				"climate": {"type": "keyword"},
				"set_heat_c": {"type": "float"},
				"set_cool_c": {"type": "float"},
				"avg_temp_c": {"type": "float"},
				"outdoor_temp_c": {"type": "float"},
				"outdoor_humidity_pct": {"type": "integer"},
				"equip": {"type": "object"},
				"sensors": {"type": "object"},
				"provider": {"type": "object"}
			}
		}
	}
}`,
		"transition": `
{
	"index_patterns": ["` + s.indexPrefix + `-transition-*"],
	"template": {
		"mappings": {
			"properties": {
				"type": {"type": "keyword"},
				"event_time": {"type": "date"},
				"thermostat_id": {"type": "keyword"},
				"thermostat_name": {"type": "keyword"},
				"prev": {"type": "object"},
				"next": {"type": "object"},
				"event": {"type": "object"},
				"provider": {"type": "object"}
			}
		}
	}
}`,
		"device_snapshot": `
{
	"index_patterns": ["` + s.indexPrefix + `-device_snapshot-*"],
	"template": {
		"mappings": {
			"properties": {
				"type": {"type": "keyword"},
				"collected_at": {"type": "date"},
				"thermostat_id": {"type": "keyword"},
				"thermostat_name": {"type": "keyword"},
				"program": {"type": "object"},
				"events_active": {"type": "object"},
				"provider": {"type": "object"}
			}
		}
	}
}`,
	}

	for templateName, templateBody := range templates {
		if err := s.createTemplate(ctx, templateName, templateBody); err != nil {
			return fmt.Errorf("creating template %s: %w", templateName, err)
		}
	}

	return nil
}

// createTemplate creates a single index template
func (s *Sink) createTemplate(ctx context.Context, templateName, templateBody string) error {
	req, err := http.NewRequestWithContext(ctx, "PUT", s.url+"/_index_template/"+templateName, strings.NewReader(templateBody))
	if err != nil {
		return fmt.Errorf("creating template request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "ApiKey "+s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing template request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("template creation failed with status %d", resp.StatusCode)
	}

	return nil
}
