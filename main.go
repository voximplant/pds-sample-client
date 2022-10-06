package main

import (
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/voximplant/pds-sample-client/client"
	"time"
)

func main() {
	prop, err := client.NewAgentProperties(client.NewAuth(1, "1234567890"))
	if err != nil {
		panic(err)
	}
	conn, err := client.NewConn(prop.Host)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	agent, err := client.NewAgent(conn, prop.Auth, &client.PDSConf{
		RuleID:            1,
		QueueID:           1,
		ReferenceIP:       "127.0.0.1",
		AvgTimeTalkSec:    80.0,
		PercentSuccessful: 0.4,
		MaximumErrorRate:  0.05,
		SessionID:         uuid.NewV4().String(),
	})
	if err != nil {
		panic(err)
	}

	taskChan := agent.GetTaskChannel()
	go func() {
		defer close(taskChan)
		for {
			// send task to agent
			tmpTask := map[string]interface{}{
				"phone_number": "1234567",
			}
			taskChan <- tmpTask
		}
	}()

	for repeat := 5; repeat > 0; repeat-- {
		err = agent.Start()
		if err != nil {
			time.Sleep(2 * time.Second)
			fmt.Println(err)
		}
	}
}
