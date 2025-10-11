package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/benvon/thermostat-telemetry-reader/internal/core"
	"github.com/benvon/thermostat-telemetry-reader/internal/providers/ecobee"
	"github.com/benvon/thermostat-telemetry-reader/internal/sinks/elasticsearch"
	"github.com/benvon/thermostat-telemetry-reader/pkg/config"
	"github.com/benvon/thermostat-telemetry-reader/pkg/model"
)

var (
	configFile = flag.String("config", "config.yaml", "Path to configuration file")
	version    = flag.Bool("version", false, "Show version information")
)

const (
	appName    = "thermostat-telemetry-reader"
	appVersion = "1.0.0"
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("%s version %s\n", appName, appVersion)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Set up logging
	logger := setupLogger(cfg.TTR.LogLevel)
	logger.Info("Starting thermostat telemetry reader",
		"version", appVersion,
		"config_file", *configFile)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("Received signal, shutting down gracefully", "signal", sig)
		cancel()
	}()

	// Initialize components
	app, err := initializeApp(ctx, cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize application", "error", err)
		os.Exit(1)
	}

	// Start health and metrics servers
	if err := startHealthServers(ctx, app, cfg, logger); err != nil {
		logger.Error("Failed to start health servers", "error", err)
		os.Exit(1)
	}

	// Start the main scheduler
	logger.Info("Starting scheduler")
	if err := app.Scheduler.Start(ctx); err != nil && err != context.Canceled {
		logger.Error("Scheduler failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Application stopped")
}

// Application holds all the application components
type Application struct {
	Config        *config.Config
	Providers     []model.Provider
	Sinks         []model.Sink
	Normalizer    *core.Normalizer
	Scheduler     *core.Scheduler
	HealthChecker *core.HealthChecker
	Metrics       *core.MetricsCollector
	Logger        *slog.Logger
}

// initializeApp initializes all application components
func initializeApp(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*Application, error) {
	app := &Application{
		Config: cfg,
		Logger: logger,
	}

	// Initialize providers
	providers, err := initializeProviders(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("initializing providers: %w", err)
	}
	app.Providers = providers

	// Initialize sinks
	sinks, err := initializeSinks(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("initializing sinks: %w", err)
	}
	app.Sinks = sinks

	// Initialize normalizer
	normalizer, err := core.NewNormalizer(cfg.TTR.Timezone)
	if err != nil {
		return nil, fmt.Errorf("initializing normalizer: %w", err)
	}
	app.Normalizer = normalizer

	// Initialize offset store (using in-memory for now)
	offsetStore := core.NewMemoryOffsetStore()

	// Initialize scheduler
	scheduler := core.NewScheduler(
		providers,
		sinks,
		normalizer,
		offsetStore,
		cfg.TTR.PollInterval,
		cfg.TTR.BackfillWindow,
		logger,
	)
	app.Scheduler = scheduler

	// Initialize health checker
	healthChecker := core.NewHealthChecker(providers, sinks)
	app.HealthChecker = healthChecker

	// Initialize metrics collector
	metrics := core.NewMetricsCollector()
	app.Metrics = metrics

	return app, nil
}

// initializeProviders initializes all configured providers
func initializeProviders(cfg *config.Config, logger *slog.Logger) ([]model.Provider, error) {
	var providers []model.Provider

	enabledProviders := cfg.GetEnabledProviders()
	for _, providerConfig := range enabledProviders {
		switch providerConfig.Name {
		case "ecobee":
			provider, err := initializeEcobeeProvider(providerConfig, logger)
			if err != nil {
				return nil, fmt.Errorf("initializing ecobee provider: %w", err)
			}
			providers = append(providers, provider)
		default:
			logger.Warn("Unknown provider type", "provider", providerConfig.Name)
		}
	}

	return providers, nil
}

// initializeEcobeeProvider initializes the Ecobee provider
func initializeEcobeeProvider(providerConfig config.ProviderConfig, logger *slog.Logger) (model.Provider, error) {
	clientID, ok := providerConfig.Settings["client_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid client_id in ecobee provider config")
	}

	refreshToken, ok := providerConfig.Settings["refresh_token"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid refresh_token in ecobee provider config")
	}

	logger.Info("Initializing Ecobee provider", "client_id", clientID)
	return ecobee.NewProvider(clientID, refreshToken), nil
}

// initializeSinks initializes all configured sinks
func initializeSinks(cfg *config.Config, logger *slog.Logger) ([]model.Sink, error) {
	var sinks []model.Sink

	enabledSinks := cfg.GetEnabledSinks()
	for _, sinkConfig := range enabledSinks {
		switch sinkConfig.Name {
		case "elasticsearch":
			sink, err := initializeElasticsearchSink(sinkConfig, logger)
			if err != nil {
				return nil, fmt.Errorf("initializing elasticsearch sink: %w", err)
			}
			sinks = append(sinks, sink)
		default:
			logger.Warn("Unknown sink type", "sink", sinkConfig.Name)
		}
	}

	return sinks, nil
}

// initializeElasticsearchSink initializes the Elasticsearch sink
func initializeElasticsearchSink(sinkConfig config.SinkConfig, logger *slog.Logger) (model.Sink, error) {
	url, ok := sinkConfig.Settings["url"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid url in elasticsearch sink config")
	}

	apiKey, _ := sinkConfig.Settings["api_key"].(string)
	indexPrefix, ok := sinkConfig.Settings["index_prefix"].(string)
	if !ok {
		indexPrefix = "ttr"
	}

	createTemplates, ok := sinkConfig.Settings["create_templates"].(bool)
	if !ok {
		createTemplates = true
	}

	logger.Info("Initializing Elasticsearch sink",
		"url", url,
		"index_prefix", indexPrefix,
		"create_templates", createTemplates)

	return elasticsearch.NewSink(url, apiKey, indexPrefix, createTemplates), nil
}

// startHealthServers starts the health and metrics HTTP servers
func startHealthServers(ctx context.Context, app *Application, cfg *config.Config, logger *slog.Logger) error {
	// Start health server
	healthMux := http.NewServeMux()
	healthMux.Handle("/healthz", app.HealthChecker.ServeHealth())
	healthMux.Handle("/metrics", app.Metrics.ServeMetrics())

	healthServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.TTR.HealthPort),
		Handler: healthMux,
	}

	go func() {
		logger.Info("Starting health server", "port", cfg.TTR.HealthPort)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Health server failed", "error", err)
		}
	}()

	// Start metrics server
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", app.Metrics.ServeMetrics())

	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.TTR.MetricsPort),
		Handler: metricsMux,
	}

	go func() {
		logger.Info("Starting metrics server", "port", cfg.TTR.MetricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Metrics server failed", "error", err)
		}
	}()

	// Graceful shutdown for servers
	go func() {
		<-ctx.Done()
		logger.Info("Shutting down HTTP servers")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer shutdownCancel()

		if err := healthServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("Failed to shutdown health server", "error", err)
		}

		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("Failed to shutdown metrics server", "error", err)
		}
	}()

	return nil
}

// setupLogger configures structured logging
func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler)
}
