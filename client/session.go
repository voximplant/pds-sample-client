package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/voximplant/pds-sample-client/api"
	"google.golang.org/grpc"
)

const (
	taskBufferSize = 100
	pingInterval   = 30 * time.Second
)

// Task is a dialing job waiting to be sent as PUT_TASK.
// CustomData is JSON-encoded and forwarded to the VoxEngine scenario.
type Task struct {
	UUID       string
	CustomData map[string]any
}

// Session maintains one bidirectional PDS stream.
type Session struct {
	cfg    Config
	client api.PDSClient
	log    *slog.Logger
	tasks  chan Task

	mu        sync.RWMutex
	sessionID string
	sendMu    sync.Mutex // gRPC BidiStreamingClient.Send is not concurrency-safe
}

// NewSession binds a PDS client to an existing gRPC connection.
func NewSession(conn *grpc.ClientConn, cfg Config, log *slog.Logger) (*Session, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}

	return &Session{
		cfg:       cfg,
		client:    api.NewPDSClient(conn),
		log:       log,
		tasks:     make(chan Task, taskBufferSize),
		sessionID: cfg.SessionID,
	}, nil
}

// Tasks returns the channel used to enqueue dialing jobs.
func (s *Session) Tasks() chan<- Task {
	return s.tasks
}

// SessionID returns the current session identifier (updated after INIT_RESPONSE).
func (s *Session) SessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionID
}

func (s *Session) setSessionID(id string) {
	s.mu.Lock()
	s.sessionID = id
	s.mu.Unlock()
}

// Run opens a stream, sends INIT, then handles GET_TASK / TASK_EVENT / PONG
// until the context is cancelled or the stream ends.
func (s *Session) Run(ctx context.Context) error {
	stream, err := s.openStream(ctx)
	if err != nil {
		return err
	}

	if err := s.send(stream, s.initMessage()); err != nil {
		return fmt.Errorf("send init: %w", err)
	}

	errCh := make(chan error, 1)
	go s.keepAlive(ctx, stream, errCh)
	go s.receiveLoop(ctx, stream, errCh)

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

func (s *Session) openStream(ctx context.Context) (grpc.BidiStreamingClient[api.RequestMessage, api.ServiceMessage], error) {
	switch s.cfg.Mode {
	case ModeProgressive:
		stream, err := s.client.StartProgressive(ctx)
		if err != nil {
			return nil, fmt.Errorf("open progressive stream: %w", err)
		}
		return stream, nil
	default:
		stream, err := s.client.Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("open predictive stream: %w", err)
		}
		return stream, nil
	}
}

func (s *Session) initMessage() *api.RequestMessage {
	init := &api.Init{
		InitStat: &api.Statistic{
			AvgTimeTalkSec:    s.cfg.AvgTimeTalkSec,
			PercentSuccessful: s.cfg.PercentSuccessful,
		},
		AccountId:         s.cfg.AccountID,
		ApiKey:            s.cfg.APIKey,
		Rule:              &api.Init_RuleId{RuleId: s.cfg.RuleID},
		ReferenceIp:       s.cfg.ReferenceIP,
		QueueId:           s.cfg.QueueID,
		MaximumErrorRate:  s.cfg.MaximumErrorRate,
		MinimumBusyFactor: s.cfg.MinimumBusyFactor,
		SessionId:         s.SessionID(),
		Application:       &api.Init_ApplicationId{ApplicationId: s.cfg.ApplicationID},
		AcdVersion:        api.Init_V2,
		PredictiveType:    toProtoPredictiveType(s.cfg.PredictiveType),
		ServerLocation:    s.cfg.ServerLocation,
		Priority:          s.cfg.Priority,
		MaxSimultaneous:   s.cfg.MaxSimultaneous,
	}

	if s.cfg.Mode == ModeProgressive {
		init.TaskMultiplier = &api.TaskMultiplier{Multiplier: s.cfg.TaskMultiplier}
	}

	return &api.RequestMessage{
		Type: api.RequestMessage_INIT,
		Init: init,
	}
}

func (s *Session) keepAlive(
	ctx context.Context,
	stream grpc.BidiStreamingClient[api.RequestMessage, api.ServiceMessage],
	errCh chan<- error,
) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.send(stream, &api.RequestMessage{Type: api.RequestMessage_PING}); err != nil {
				errCh <- fmt.Errorf("send ping: %w", err)
				return
			}
		}
	}
}

func (s *Session) send(
	stream grpc.BidiStreamingClient[api.RequestMessage, api.ServiceMessage],
	msg *api.RequestMessage,
) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return stream.Send(msg)
}

func (s *Session) receiveLoop(
	ctx context.Context,
	stream grpc.BidiStreamingClient[api.RequestMessage, api.ServiceMessage],
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
		case api.ServiceMessage_INIT_RESPONSE:
			sessionID := msg.GetInit().GetSessionId()
			if sessionID != "" {
				s.setSessionID(sessionID)
			}
			s.log.Info("session initialized", "session_id", sessionID)

		case api.ServiceMessage_GET_TASK:
			req := msg.GetRequest()
			s.log.Info("tasks requested",
				"count", req.GetCount(),
				"user_ids", req.GetUserIds(),
			)
			if err := s.sendRequestedTasks(ctx, stream, int(req.GetCount())); err != nil {
				errCh <- err
				return
			}

		case api.ServiceMessage_TASK_EVENT:
			event := msg.GetEvent()
			s.log.Info("task event",
				"task_uuid", event.GetTaskUUID(),
				"type", event.GetType().String(),
				"started_at", event.GetStartedAt(),
				"result", event.GetResult(),
				"ms_check_url", event.GetMsCheckUrl(),
				"ms_access_url", event.GetMsAccessUrl(),
				"media_session_access_secure_url", event.GetMediaSessionAccessSecureUrl(),
				"call_session_history_id", event.GetCallSessionHistoryId(),
			)

		case api.ServiceMessage_PONG:
			s.log.Debug("pong received")

		default:
			s.log.Warn("unknown service message", "type", msg.GetType().String())
		}
	}
}

// sendRequestedTasks must send exactly count PUT_TASK messages.
// Sending fewer or more tasks causes the server to close the stream.
func (s *Session) sendRequestedTasks(
	ctx context.Context,
	stream grpc.BidiStreamingClient[api.RequestMessage, api.ServiceMessage],
	count int,
) error {
	if count <= 0 {
		return nil
	}

	for remaining := count; remaining > 0; remaining-- {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case task, ok := <-s.tasks:
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

			if err := s.send(stream, &api.RequestMessage{
				Type: api.RequestMessage_PUT_TASK,
				Task: &api.PutTask{
					CustomData: payload,
					TaskUUID:   taskUUID,
				},
			}); err != nil {
				return fmt.Errorf("send task %s: %w", taskUUID, err)
			}

			s.log.Debug("task sent", "task_uuid", taskUUID)
		}
	}

	return nil
}

func toProtoPredictiveType(value PredictiveType) api.Init_PredictiveType {
	switch value {
	case PredictiveAROptimized:
		return api.Init_AR_OPTIMIZED
	case PredictiveBFOptimized:
		return api.Init_BF_OPTIMIZED
	case PredictiveARSmallGroup:
		return api.Init_AR_SMALL_GROUP
	case PredictiveARAutoBalanced:
		return api.Init_AR_AUTO_BALANCED
	default:
		return api.Init_DEFAULT
	}
}
