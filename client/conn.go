package client

import (
	"crypto/tls"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Dial opens a gRPC connection to the PDS endpoint from Config.
func Dial(cfg Config) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if cfg.UseTLS {
		opts = append(opts,
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                20 * time.Second,
				Timeout:             3 * time.Second,
				PermitWithoutStream: true,
			}),
		)
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", cfg.Address, err)
	}
	return conn, nil
}
