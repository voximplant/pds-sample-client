package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log"

	uuid "github.com/satori/go.uuid"
	"github.com/voximplant/pds-sample-client/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Buffer size for preloaded task
const _bufferSize = 100

type PDSAgent interface {
	Start() error
	GetTaskChannel() chan<- map[string]interface{}
	ChangeErrorRate(value float64) error
}

type agent struct {
	RcTask chan map[string]interface{}
	// inner entities
	authConf  *AuthConf
	pdsConf   *PDSConf
	client    service.PDSClient
	rcErrRate chan float64
	done      chan interface{}
}

func NewConn(hostCfg *HostConf) (*grpc.ClientConn, error) {
	var additionalDealOpt = make([]grpc.DialOption, 0)
	if !hostCfg.UseTls {
		additionalDealOpt = append(additionalDealOpt, grpc.WithInsecure())
	} else {
		config := &tls.Config{
			InsecureSkipVerify: false,
		}
		additionalDealOpt = append(additionalDealOpt, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	}
	conn, err := grpc.Dial(hostCfg.GetAddr(), additionalDealOpt...)
	if err != nil {
		return nil, err
	}
	return conn, err
}

func NewAgent(conn *grpc.ClientConn, authConf *AuthConf, pdsConf *PDSConf) (PDSAgent, error) {
	if pdsConf == nil {
		return nil, errors.New("invalid pdsConf argument")
	}
	if err := pdsConf.Validate(); err != nil {
		return nil, err
	}
	c := service.NewPDSClient(conn)

	res := &agent{
		client:    c,
		pdsConf:   pdsConf,
		authConf:  authConf,
		RcTask:    make(chan map[string]interface{}, _bufferSize),
		rcErrRate: make(chan float64),
		done:      make(chan interface{}),
	}
	return res, nil
}

func (c *agent) GetTaskChannel() chan<- map[string]interface{} {
	return c.RcTask
}

func (c *agent) ChangeErrorRate(value float64) error {
	if value <= 0.0 || value >= 1.0 {
		return errors.New("error rate should be greater then 0 but less then 1")
	}
	if value > 0.5 {
		log.Println("WARNING: error rate is very high:", value)
	}
	c.rcErrRate <- value
	return nil
}

func (c *agent) Stop() {
	close(c.done)
}

func (c *agent) Start() error {
	initConf := service.RequestMessage{
		Type: service.RequestMessage_INIT,
		Init: &service.Init{
			InitStat: &service.Statistic{
				AvgTimeTalkSec:    c.pdsConf.AvgTimeTalkSec,
				PercentSuccessful: c.pdsConf.PercentSuccessful,
			},
			AccountId:        c.authConf.AccountID,
			ApiKey:           c.authConf.ApiKey,
			Rule:             &service.Init_RuleId{RuleId: c.pdsConf.RuleID},
			ReferenceIp:      c.pdsConf.ReferenceIP,
			QueueId:          c.pdsConf.QueueID,
			MaximumErrorRate: c.pdsConf.MaximumErrorRate,
			SessionId:        c.pdsConf.SessionID,
			Application:      &service.Init_ApplicationId{ApplicationId: c.pdsConf.ApplicationID},
			AcdVersion:       service.Init_SQ,
		},
	}

	cntx := context.Background()
	stream, err := c.client.Start(cntx)
	if err != nil {
		return err
	}
	err = stream.Send(&initConf)
	if err != nil {
		return err
	}

	waitc := make(chan error)
	go func() {
		defer close(waitc)
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				waitc <- err
				return
			}
			log.Println("Receive message:", in)
			switch in.Type {
			case service.ServiceMessage_INIT_RESPONSE:
				log.Println("success init ...")
			case service.ServiceMessage_GET_TASK:
				log.Println("get tasks ... ", in.Request.Count)
				toConsume := in.Request.Count

				if toConsume == 0 {
					continue
				}
				for customData := range c.RcTask {
					toConsume--
					b, _ := json.Marshal(customData)
					s := string(b)

					cd := service.PutTask{
						CustomData: []byte(s),
						TaskUUID:   uuid.NewV4().String(),
					}

					err := stream.Send(&service.RequestMessage{
						Type: service.RequestMessage_PUT_TASK,
						Task: &cd,
					})
					if err != nil {
						waitc <- err
						return
					}
					if toConsume == 0 {
						break
					}
				}
			}
		}
	}()
	select {
	case err := <-waitc:
		return err
	case <-c.done:
		stream.CloseSend()
	}
	return nil
}
