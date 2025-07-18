package config

import (
	"testing"
)

func TestNew(t *testing.T) {
	cfg := New()

	// Test default values
	if cfg.MaxTokens != 600 {
		t.Errorf("expected default MaxTokens 600, got %d", cfg.MaxTokens)
	}
	if cfg.Temperature != 1.0 {
		t.Errorf("expected default Temperature 1.0, got %f", cfg.Temperature)
	}
	if cfg.TopP != 0.75 {
		t.Errorf("expected default TopP 0.75, got %f", cfg.TopP)
	}
	if cfg.FrequencyPenalty != 0.0 {
		t.Errorf("expected default FrequencyPenalty 0.0, got %f", cfg.FrequencyPenalty)
	}
	if cfg.PresencePenalty != 0.0 {
		t.Errorf("expected default PresencePenalty 0.0, got %f", cfg.PresencePenalty)
	}
	if cfg.TopK != 0 {
		t.Errorf("expected default TopK 0, got %d", cfg.TopK)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := New()
	cfg.CompartmentID = "test-compartment-id"

	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config to pass validation, got: %v", err)
	}
}

func TestValidate_MissingCompartmentID(t *testing.T) {
	cfg := New()
	// CompartmentID is empty

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing compartmentId")
	}
	if err.Error() != "compartmentId is required and cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_InvalidTemperature(t *testing.T) {
	cfg := New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.Temperature = -1.0

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid temperature")
	}
}

func TestValidate_InvalidTopP(t *testing.T) {
	cfg := New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.TopP = 1.5

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid topP")
	}
}

func TestValidate_InvalidFrequencyPenalty(t *testing.T) {
	cfg := New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.FrequencyPenalty = -3.0

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid frequencyPenalty")
	}
}

func TestValidate_InvalidPresencePenalty(t *testing.T) {
	cfg := New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.PresencePenalty = 3.0

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid presencePenalty")
	}
}

func TestValidate_InvalidMaxTokens(t *testing.T) {
	cfg := New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.MaxTokens = 0

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid maxTokens")
	}
}

func TestValidate_InvalidTopK(t *testing.T) {
	cfg := New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.TopK = -1

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid topK")
	}
}
