package client

import (
	"errors"
	"fmt"
)

type PredictiveType int

const (
	DefaultPredictiveType               PredictiveType = iota // Default is AbandonRateOptimize
	AbandonRateOptimization                                   // PDS uses abandoned calls control rate for answered calls. Works for a large number of operators (more than 20)..
	BusyFactorOptimization                                    // PDS uses agent busy factor control algorithm. Works for a large number of operators (more than 20).
	SmallGroupAbandonRateOptimization                         // PDS uses abandoned calls control rate for answered calls. Works only when the number of agents is less than 20.
	AutoBalancedAbandonRateOptimization                       // PDS uses abandoned calls control rate for answered calls. Works as a combination of AbandonRateOptimize and SmallGroupAbandonRateOptimize algorithms.
)

var _defaultHost = &HostConf{
	Host:   "pds.voximplant.com",
	Port:   3005,
	UseTls: true,
}

type HostConf struct {
	Host   string
	Port   int
	UseTls bool
}

func (c *HostConf) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type AuthConf struct {
	AccountID int32
	ApiKey    string
}

type PDSConf struct {
	QueueID           int32
	RuleID            int32
	ReferenceIP       string
	AvgTimeTalkSec    float64
	PercentSuccessful float64
	MaximumErrorRate  float64
	MinimumBusyFactor float64
	SessionID         string
	ApplicationID     int32
	TaskMultiplier    float32
	PredictiveType    PredictiveType
}

func (p *PDSConf) Validate() error {
	if p.QueueID <= 0 {
		return errors.New("queueID is required")
	}
	if p.RuleID <= 0 {
		return errors.New("ruleID is required")
	}
	if p.ReferenceIP == "" {
		p.ReferenceIP = "127.0.0.1"
	}
	if p.MaximumErrorRate <= 0.0 || p.MaximumErrorRate >= 1.0 {
		return errors.New("maximumErrorRate should be greater then 0 but less then 1")
	}
	if p.PercentSuccessful <= 0.0 || p.PercentSuccessful > 1.0 {
		return errors.New("percentSuccessful should be greater then 0 but less or equals 1")
	}
	return nil
}

type AgentConfig struct {
	Auth *AuthConf
	Host *HostConf
}

func NewAuth(accountID int32, apiKey string) *AuthConf {
	return &AuthConf{
		AccountID: accountID,
		ApiKey:    apiKey,
	}
}

func DefaultHostConfig() *HostConf {
	return _defaultHost
}

func NewAgentProperties(auth *AuthConf) (*AgentConfig, error) {
	if auth == nil {
		return nil, errors.New("not found required params")
	}
	return &AgentConfig{
		Auth: auth,
		Host: DefaultHostConfig(),
	}, nil
}
