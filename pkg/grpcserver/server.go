package grpcserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"cardwar/pkg/etcdutil"

	grpclog "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// Config holds the configuration for creating a gRPC server.
type Config struct {
	Name          string   // service name, e.g. "user-service"
	Addr          string   // listen address, e.g. ":50051"
	ID            string   // instance ID; auto-generated if empty
	EtcdEndpoints []string // etcd cluster endpoints, empty = no registration
}

// Server wraps a *grpc.Server with health checking and optional etcd registration.
type Server struct {
	cfg        Config
	GRPC       *grpc.Server
	health     *health.Server
	etcdClient *etcdutil.Client
	etcdCancel func()
}

// ID returns the server's instance ID.
func (s *Server) ID() string { return s.cfg.ID }

// New creates a gRPC server. etcd self-registration is automatic when EtcdEndpoints
// is non-empty. If ID is empty, a random one is generated.
func New(cfg Config) (*Server, error) {
	if cfg.ID == "" {
		cfg.ID = cfg.Name + "-" + randomHex(4)
	}

	recoveryOpts := []grpcrecovery.Option{
		grpcrecovery.WithRecoveryHandler(func(p any) error {
			log.Printf("grpc: panic recovered in %s: %v", cfg.Name, p)
			return status.Errorf(codes.Internal, "internal error")
		}),
	}

	loggingOpts := []grpclog.Option{
		grpclog.WithLogOnEvents(grpclog.FinishCall),
	}

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcrecovery.UnaryServerInterceptor(recoveryOpts...),
			grpclog.UnaryServerInterceptor(interceptorLogger(), loggingOpts...),
		),
		grpc.ChainStreamInterceptor(
			grpcrecovery.StreamServerInterceptor(recoveryOpts...),
			grpclog.StreamServerInterceptor(interceptorLogger(), loggingOpts...),
		),
	)

	hs := health.NewServer()
	healthpb.RegisterHealthServer(srv, hs)

	s := &Server{
		cfg:    cfg,
		GRPC:   srv,
		health: hs,
	}

	if len(cfg.EtcdEndpoints) > 0 {
		etcdCfg := etcdutil.NewConfig(cfg.EtcdEndpoints)
		cli, err := etcdutil.NewClient(etcdCfg)
		if err != nil {
			return nil, fmt.Errorf("grpcserver: connect etcd: %w", err)
		}
		s.etcdClient = cli

		key := etcdutil.ServiceKey(cfg.Name, cfg.ID)
		cancel, err := etcdutil.Register(cli, key, cfg.Addr)
		if err != nil {
			cli.Close()
			return nil, fmt.Errorf("grpcserver: register in etcd: %w", err)
		}
		s.etcdCancel = cancel
	}

	return s, nil
}

// Serve starts listening and serving gRPC requests.
func (s *Server) Serve() error {
	lis, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.Addr, err)
	}

	s.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	log.Printf("grpc: %s/%s listening on %s", s.cfg.Name, s.cfg.ID, s.cfg.Addr)
	return s.GRPC.Serve(lis)
}

// GracefulStop unregisters from etcd and gracefully stops the gRPC server.
func (s *Server) GracefulStop() {
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	if s.etcdCancel != nil {
		s.etcdCancel()
	}
	if s.etcdClient != nil {
		s.etcdClient.Close()
	}
	s.GRPC.GracefulStop()
}

// WaitForShutdown blocks until SIGINT or SIGTERM, then calls GracefulStop.
func (s *Server) WaitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Printf("grpc: %s/%s shutting down...", s.cfg.Name, s.cfg.ID)
	s.GracefulStop()
}

func interceptorLogger() grpclog.Logger {
	return grpclog.LoggerFunc(func(ctx context.Context, lvl grpclog.Level, msg string, fields ...any) {
		log.Printf("[%v] %s %v", lvl, msg, fields)
	})
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
