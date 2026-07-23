package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/voximplant/pds-sample-client/protogen"
	"google.golang.org/grpc"
)

const (
	taskBufferSize = 100
	pingInterval   = 30 * time.Second
)

// Task is a single dialing job sent to PDS.
// CustomData is forwarded to the VoxEngine scenario as users_data.
type Task struct {
	UUID       string
	CustomData map[string]any
}

// Agent maintains a bidirectional PDS session.
type Agent struct {
	cfg    Config
	client protogen.PDSClient
	log    *slog.Logger
	tasks  chan Task
}

// NewAgent creates a PDS client bound to an existing gRPC connection.
func NewAgent(conn *grpc.ClientConn, cfg Config, log *slog.Logger) (*Agent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}

	return &Agent{
		cfg:    cfg,
		client: protogen.NewPDSClient(conn),
		log:    log,
		tasks:  make(chan Task, taskBufferSize),
	}, nil
}

// Tasks returns the channel used to enqueue dialing tasks.
func (a *Agent) Tasks() chan<- Task {
	return a.tasks
}

// Run executes the PDS protocol until the context is cancelled or the stream ends.
func (a *Agent) Run(ctx context.Context) error {
	stream, err := a.openStream(ctx)
	if err != nil {
		return err
	}

	if err := stream.Send(a.initMessage()); err != nil {
		return fmt.Errorf("send init: %w", err)
	}

	errCh := make(chan error, 1)
	go a.keepAlive(ctx, stream, errCh)
	go a.receiveLoop(ctx, stream, errCh)

	select {
	case <-ctx.Done():
		_ = stream.CloseSend()
		return ctx.Err()
	case err := <-errCh:
		if err == nil || errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
}

func (a *Agent) openStream(ctx context.Context) (grpc.BidiStreamingClient[protogen.RequestMessage, protogen.ServiceMessage], error) {
	switch a.cfg.Mode {
	case ModeProgressive:
		stream, err := a.client.StartProgressive(ctx)
		if err != nil {
			return nil, fmt.Errorf("open progressive stream: %w", err)
		}
		return stream, nil
	default:
		stream, err := a.client.Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("open predictive stream: %w", err)
		}
		return stream, nil
	}
}

func (a *Agent) initMessage() *protogen.RequestMessage {
	init := &protogen.Init{
		InitStat: &protogen.Statistic{
			AvgTimeTalkSec:    a.cfg.AvgTimeTalkSec,
			PercentSuccessful: a.cfg.PercentSuccessful,
		},
		AccountId:         a.cfg.AccountID,
		ApiKey:            a.cfg.APIKey,
		Rule:              &protogen.Init_RuleId{RuleId: a.cfg.RuleID},
		ReferenceIp:       a.cfg.ReferenceIP,
		QueueId:           a.cfg.QueueID,
		MaximumErrorRate:  a.cfg.MaximumErrorRate,
		MinimumBusyFactor: a.cfg.MinimumBusyFactor,
		SessionId:         a.cfg.SessionID,
		Application:       &protogen.Init_ApplicationId{ApplicationId: a.cfg.ApplicationID},
		AcdVersion:        protogen.Init_V2,
		PredictiveType:    toProtoPredictiveType(a.cfg.PredictiveType),
	}

	if a.cfg.Mode == ModeProgressive {
		init.TaskMultiplier = &protogen.TaskMultiplier{Multiplier: a.cfg.TaskMultiplier}
	}

	return &protogen.RequestMessage{
		Type: protogen.RequestMessage_INIT,
		Init: init,
	}
}

func (a *Agent) keepAlive(
	ctx context.Context,
	stream grpc.BidiStreamingClient[protogen.RequestMessage, protogen.ServiceMessage],
	errCh chan<- error,
) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := stream.Send(&protogen.RequestMessage{Type: protogen.RequestMessage_PING}); err != nil {
				errCh <- fmt.Errorf("send ping: %w", err)
				return
			}
		}
	}
}

func (a *Agent) receiveLoop(
	ctx context.Context,
	stream grpc.BidiStreamingClient[protogen.RequestMessage, protogen.ServiceMessage],
	errCh chan<- error,
) {
	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				errCh <- nil
				return
			}
			errCh <- fmt.Errorf("receive message: %w", err)
			return
		}

		switch msg.GetType() {
		case protogen.ServiceMessage_INIT_RESPONSE:
			a.log.Info("session initialized", "session_id", msg.GetInit().GetSessionId())
		case protogen.ServiceMessage_GET_TASK:
			if err := a.sendRequestedTasks(ctx, stream, int(msg.GetRequest().GetCount())); err != nil {
				errCh <- err
				return
			}
		case protogen.ServiceMessage_TASK_EVENT:
			event := msg.GetEvent()
			a.log.Info("task event",
				"task_uuid", event.GetTaskUUID(),
				"type", event.GetType().String(),
			)
		case protogen.ServiceMessage_PONG:
			a.log.Debug("pong received")
		default:
			a.log.Warn("unknown service message", "type", msg.GetType().String())
		}
	}
}

func (a *Agent) sendRequestedTasks(
	ctx context.Context,
	stream grpc.BidiStreamingClient[protogen.RequestMessage, protogen.ServiceMessage],
	count int,
) error {
	if count <= 0 {
		return nil
	}

	a.log.Info("tasks requested", "count", count)

	for remaining := count; remaining > 0; remaining-- {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case task, ok := <-a.tasks:
			if !ok {
				return errors.New("task channel closed before all requested tasks were sent")
			}

			payload, err := json.Marshal(task.CustomData)
			if err != nil {
				return fmt.Errorf("marshal task custom data: %w", err)
			}

			taskUUID := task.UUID
			if taskUUID == "" {
				taskUUID = uuid.NewString()
			}

			if err := stream.Send(&protogen.RequestMessage{
				Type: protogen.RequestMessage_PUT_TASK,
				Task: &protogen.PutTask{
					CustomData: payload,
					TaskUUID:   taskUUID,
				},
			}); err != nil {
				return fmt.Errorf("send task %s: %w", taskUUID, err)
			}

			a.log.Debug("task sent", "task_uuid", taskUUID)
		}
	}

	return nil
}

func toProtoPredictiveType(value PredictiveType) protogen.Init_PredictiveType {
	switch value {
	case PredictiveAROptimized:
		return protogen.Init_AR_OPTIMIZED
	case PredictiveBFOptimized:
		return protogen.Init_BF_OPTIMIZED
	case PredictiveARSmallGroup:
		return protogen.Init_AR_SMALL_GROUP
	case PredictiveARAutoBalanced:
		return protogen.Init_AR_AUTO_BALANCED
	default:
		return protogen.Init_DEFAULT
	}
}
