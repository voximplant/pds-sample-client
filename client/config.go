package client

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

// Mode selects the gRPC stream method.
type Mode string

const (
	ModePredictive  Mode = "predictive"
	ModeProgressive Mode = "progressive"
)

// SetValue implements cleanenv.Setter.
func (m *Mode) SetValue(s string) error {
	switch Mode(strings.ToLower(strings.TrimSpace(s))) {
	case "", ModePredictive:
		*m = ModePredictive
	case ModeProgressive:
		*m = ModeProgressive
	default:
		return fmt.Errorf("unsupported mode %q", s)
	}
	return nil
}

// PredictiveType selects the PDS dialing algorithm.
type PredictiveType int

const (
	PredictiveDefault PredictiveType = iota
	PredictiveAROptimized
	PredictiveBFOptimized
	PredictiveARSmallGroup
	PredictiveARAutoBalanced
)

// SetValue implements cleanenv.Setter.
func (p *PredictiveType) SetValue(s string) error {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "default":
		*p = PredictiveDefault
	case "bf_optimized", "busy_factor":
		*p = PredictiveBFOptimized
	case "ar_small_group", "small_group":
		*p = PredictiveARSmallGroup
	case "ar_auto_balanced", "auto_balanced":
		*p = PredictiveARAutoBalanced
	case "", "ar_optimized":
		*p = PredictiveAROptimized
	default:
		return fmt.Errorf("unsupported predictive type %q", s)
	}
	return nil
}

// Config holds all runtime settings for the PDS sample client.
// See .env.example for the list of environment variables.
type Config struct {
	Address           string         `env:"PDS_ADDRESS" env-default:"pds.voximplant.com:3005"`
	UseTLS            bool           `env:"PDS_USE_TLS" env-default:"true"`
	AccountID         int32          `env:"PDS_ACCOUNT_ID"`
	APIKey            string         `env:"PDS_API_KEY"`
	RuleID            int32          `env:"PDS_RULE_ID"`
	QueueID           int32          `env:"PDS_QUEUE_ID"`
	ApplicationID     int32          `env:"PDS_APPLICATION_ID"`
	ReferenceIP       string         `env:"PDS_REFERENCE_IP" env-default:"127.0.0.1"`
	SessionID         string         `env:"PDS_SESSION_ID"`
	AvgTimeTalkSec    float64        `env:"PDS_AVERAGE_TALK_TIME" env-default:"80"`
	PercentSuccessful float64        `env:"PDS_SUCCESS_RATE" env-default:"0.4"`
	MaximumErrorRate  float64        `env:"PDS_MAXIMUM_ERROR_RATE" env-default:"0.05"`
	MinimumBusyFactor float64        `env:"PDS_MINIMUM_BUSY_FACTOR" env-default:"0.8"`
	TaskMultiplier    float32        `env:"PDS_TASK_MULTIPLIER" env-default:"1"`
	PredictiveType    PredictiveType `env:"PDS_PREDICTIVE_TYPE" env-default:"ar_optimized"`
	Mode              Mode           `env:"PDS_MODE" env-default:"predictive"`
}

// LoadConfig reads settings from .env (if present) and process environment variables.
// Environment variables override values from the file.
func LoadConfig() (Config, error) {
	var cfg Config

	if _, err := os.Stat(".env"); err == nil {
		if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
			return Config{}, fmt.Errorf("read .env: %w", err)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return Config{}, fmt.Errorf("read env: %w", err)
		}
	} else {
		return Config{}, fmt.Errorf("stat .env: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks that required fields are set and values are in range.
func (c *Config) Validate() error {
	if c.Address == "" {
		return errors.New("address is required")
	}
	if c.AccountID <= 0 {
		return errors.New("account id must be positive")
	}
	if c.APIKey == "" {
		return errors.New("api key is required")
	}
	if c.RuleID <= 0 {
		return errors.New("rule id must be positive")
	}
	if c.QueueID <= 0 {
		return errors.New("queue id must be positive")
	}
	if c.ApplicationID <= 0 {
		return errors.New("application id must be positive")
	}
	if c.ReferenceIP == "" {
		c.ReferenceIP = "127.0.0.1"
	}
	if c.PercentSuccessful <= 0 || c.PercentSuccessful > 1 {
		return errors.New("percent successful must be in (0, 1]")
	}
	if c.MaximumErrorRate <= 0 || c.MaximumErrorRate >= 1 {
		return errors.New("maximum error rate must be in (0, 1)")
	}
	if c.MinimumBusyFactor <= 0 || c.MinimumBusyFactor > 1 {
		return errors.New("minimum busy factor must be in (0, 1]")
	}
	if c.Mode == "" {
		c.Mode = ModePredictive
	}
	switch c.Mode {
	case ModePredictive, ModeProgressive:
	default:
		return fmt.Errorf("unsupported mode %q", c.Mode)
	}
	if c.Mode == ModeProgressive && c.TaskMultiplier <= 0 {
		return errors.New("task multiplier must be positive in progressive mode")
	}
	return nil
}
