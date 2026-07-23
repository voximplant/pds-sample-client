package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/voximplant/pds-sample-client/client"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := client.LoadConfig()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	if cfg.SessionID == "" {
		cfg.SessionID = uuid.NewString()
	}

	conn, err := client.Dial(cfg)
	if err != nil {
		logger.Error("failed to connect", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	agent, err := client.NewAgent(conn, cfg, logger)
	if err != nil {
		logger.Error("failed to create agent", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go feedSampleTasks(ctx, agent.Tasks(), logger)

	for attempt := 1; ; attempt++ {
		logger.Info("starting pds session", "attempt", attempt, "mode", cfg.Mode)

		err = agent.Run(ctx)
		if ctx.Err() != nil {
			logger.Info("shutdown requested")
			return
		}
		if err != nil {
			logger.Error("pds session ended with error", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		logger.Warn("pds session closed by server, reconnecting")
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// feedSampleTasks demonstrates how to enqueue call-list records for PDS.
// Replace this with your CRM / database / queue integration.
func feedSampleTasks(ctx context.Context, tasks chan<- client.Task, log *slog.Logger) {
	defer close(tasks)

	for i := 1; ; i++ {
		task := client.Task{
			CustomData: map[string]any{
				"phone_number": "1234567890",
				"customer_id":  i,
			},
		}

		select {
		case <-ctx.Done():
			return
		case tasks <- task:
			log.Debug("task queued", "customer_id", i)
		}
	}
}
