package main

import (
	"context"
	"fmt"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/voximplant/pds-sample-client/client"
)

func main() {
	//TODO: put account id and API key below
	prop, err := client.NewAgentProperties(client.NewAuth(1, "api-key"))
	if err != nil {
		panic(err)
	}
	conn, err := client.NewConn(prop.Host)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	pdsConfig := client.PDSConf{
		RuleID:            1, //TODO: Put your rule id here
		QueueID:           1, //TODO: Put SmartQueue queue id here
		ReferenceIP:       "69.167.178.4",
		AvgTimeTalkSec:    80.0,
		PercentSuccessful: 0.4,
		MaximumErrorRate:  0.05,                           // TODO: config for Abandonment rate optimized solutions
		MinimumBusyFactor: 0.8,                            // TODO: config for busy factor optimized solutions
		PredictiveType:    client.AbandonRateOptimization, // TODO: change it to switch predictive algorithm
		SessionID:         uuid.NewV4().String(),
		ApplicationID:     1, //TODO: Put your application id here
	}
	//TODO: Uncommend following line to enable progressive mode instead of predictive.
	//pdsConfig.TaskMultiplier = 1

	agent, err := client.NewAgent(conn, prop.Auth, &pdsConfig)
	if err != nil {
		panic(err)
	}

	taskChan := agent.GetTaskChannel()
	go func() {
		defer close(taskChan)
		for {
			// send task to agent
			//TODO: send actual call list data to service
			tmpTask := map[string]interface{}{
				"phone_number": "1234567",
			}
			taskChan <- tmpTask
		}
	}()

	for repeat := 5; repeat > 0; repeat-- {
		err = runAgent(context.Background(), agent)
		if err != nil {
			time.Sleep(2 * time.Second)
			fmt.Println(err)
		}
	}
}

func runAgent(ctx context.Context, agent client.PDSAgent) error {
	cc, cancel := context.WithCancel(ctx)
	defer cancel()
	return agent.Start(cc)
}
