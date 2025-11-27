package tracing

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Provider manages the OpenTelemetry trace provider lifecycle
type Provider struct {
	tp     *sdktrace.TracerProvider
	logger *slog.Logger
}

// TracerConfig holds configuration for trace provider initialization
type TracerConfig struct {
	Enabled     bool
	Exporter    string  // otlp-grpc | otlp-http | stdout | jaeger | zipkin
	Endpoint    string  // Exporter endpoint
	SampleRate  float64 // 0.0-1.0 (default: 1.0)
	ServiceName string  // Service name for traces
	Version     string  // Service version for traces
	UseTLS      bool    // Enable TLS for production (default: false for development)
}

// NewProvider creates and initializes a new OpenTelemetry trace provider
func NewProvider(ctx context.Context, cfg TracerConfig, logger *slog.Logger) (*Provider, error) {
	if !cfg.Enabled {
		logger.Debug("Distributed tracing disabled")
		return &Provider{logger: logger}, nil
	}

	logger.Info("Initializing distributed tracing",
		slog.String("exporter", cfg.Exporter),
		slog.String("endpoint", cfg.Endpoint),
		slog.Float64("sample_rate", cfg.SampleRate),
		slog.String("service", cfg.ServiceName))

	// Create trace exporter
	exporter, err := createExporter(ctx, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create resource with service information
	version := cfg.Version
	if version == "" {
		version = "unknown"
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create sampler based on sample rate
	var sampler sdktrace.Sampler
	if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SampleRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	logger.Info("Distributed tracing initialized successfully")

	return &Provider{
		tp:     tp,
		logger: logger,
	}, nil
}

// createExporter creates the appropriate trace exporter based on configuration
func createExporter(ctx context.Context, cfg TracerConfig, logger *slog.Logger) (sdktrace.SpanExporter, error) {
	switch cfg.Exporter {
	case "otlp-grpc":
		return createOTLPGRPCExporter(ctx, cfg.Endpoint, cfg.UseTLS, logger)
	case "stdout":
		return createStdoutExporter()
	default:
		return nil, fmt.Errorf("unsupported trace exporter: %s (supported: otlp-grpc, stdout)", cfg.Exporter)
	}
}

// createOTLPGRPCExporter creates an OTLP gRPC trace exporter
func createOTLPGRPCExporter(ctx context.Context, endpoint string, useTLS bool, logger *slog.Logger) (sdktrace.SpanExporter, error) {
	logger.Debug("Creating OTLP gRPC exporter",
		slog.String("endpoint", endpoint),
		slog.Bool("tls", useTLS))

	// Create gRPC connection with TLS credentials for production, insecure for development
	var opts []grpc.DialOption
	if useTLS {
		// Use system TLS credentials for production
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
		logger.Info("OTLP gRPC exporter configured with TLS (production mode)")
	} else {
		// Use insecure credentials for development
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		logger.Warn("OTLP gRPC exporter configured without TLS (development mode)")
	}

	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP gRPC exporter: %w", err)
	}

	return exporter, nil
}

// createStdoutExporter creates a stdout trace exporter for development
func createStdoutExporter() (sdktrace.SpanExporter, error) {
	return stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
}

// Tracer returns a tracer for the given component name
func (p *Provider) Tracer(name string) trace.Tracer {
	if p.tp == nil {
		return noop.NewTracerProvider().Tracer(name)
	}
	return p.tp.Tracer(name)
}

// Shutdown gracefully shuts down the trace provider
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.tp == nil {
		return nil
	}

	p.logger.Info("Shutting down distributed tracing")
	if err := p.tp.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown trace provider: %w", err)
	}

	p.logger.Debug("Distributed tracing shutdown complete")
	return nil
}

// Enabled returns whether tracing is enabled
func (p *Provider) Enabled() bool {
	return p.tp != nil
}
