// SPDX-License-Identifier: Apache-2.0
//
// Copyright Jan-Otto Kröpke
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/jkroepke/access-log-exporter/internal/collector"
	"github.com/jkroepke/access-log-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	versionCollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
)

type ReturnCode = int

const (
	// ReturnCodeNoError indicates that the program should continue running.
	ReturnCodeNoError ReturnCode = -2
	// ReturnCodeReload indicates that the configuration should be reloaded.
	ReturnCodeReload ReturnCode = -1
	// ReturnCodeOK indicates a successful execution of the program.
	ReturnCodeOK ReturnCode = 0
	// ReturnCodeError indicates an error during execution.
	ReturnCodeError ReturnCode = 1
)

var ErrReload = errors.New("reload")

func main() {
	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGUSR1)

	os.Exit(execute(os.Args, os.Stdout, termCh)) //nolint:forbidigo // entry point
}

// execute is the main entry point for the daemon.
func execute(args []string, stdout io.Writer, termCh <-chan os.Signal) int {
	ctx := context.Background()

	for {
		if returnCode := run(ctx, args, stdout, termCh); returnCode != ReturnCodeReload {
			return returnCode
		}
	}
}

// run runs the main program logic of the daemon.
func run(ctx context.Context, args []string, stdout io.Writer, termCh <-chan os.Signal) ReturnCode {
	conf, logger, rc := initializeConfigAndLogger(args, stdout)
	if rc != ReturnCodeNoError {
		return rc
	}

	// initialize the root context with a cancel function
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	logger.LogAttrs(ctx, slog.LevelDebug, "config", slog.String("config", conf.String()))

	wg := &sync.WaitGroup{}
	defer wg.Wait()

	stopChan := make(chan bool, 1)

	// Start debug listener if enabled
	if conf.Debug.Pprof {
		startDebugListener(ctx, cancel, wg, logger, conf)
	}

	prometheusCollector, err := collector.New(ctx, logger, conf)
	if err != nil {
		logger.LogAttrs(ctx, slog.LevelError, "error creating collector", slog.Any("error", err))
		return ReturnCodeError
	}

	if conf.VerifyConfig {
		return ReturnCodeOK
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewBuildInfoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		versionCollector.NewCollector("prometheus_nginxlog_exporter"),
		prometheusCollector,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.Handle("GET /metrics", promhttp.InstrumentMetricHandler(reg, promhttp.HandlerFor(
		prometheus.Gatherers{reg},
		promhttp.HandlerOpts{
			ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
			ErrorHandling:     promhttp.ContinueOnError,
			Registry:          reg,
			EnableOpenMetrics: true,
		},
	)))

	server := &http.Server{
		Addr:              conf.Debug.ListenAddress,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       3 * time.Second,
		WriteTimeout:      1 * time.Minute,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		Handler:           mux,
	}

	defer func() {
		defer wg.Done()
		wg.Add(1)

		prometheusCollector.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		server.RegisterOnShutdown(cancel)
		if err := server.Shutdown(ctx); err != nil {
			logger.LogAttrs(ctx, slog.LevelError, "error shutting down server", slog.Any("error", err))
		} else {
			logger.LogAttrs(ctx, slog.LevelInfo, "server shutdown gracefully")
		}
	}()

	webConfig := &web.FlagConfig{
		WebListenAddresses: &([]string{conf.Web.ListenAddress}),
		WebConfigFile:      &conf.Web.ConfigFile,
	}

	go func() {
		defer wg.Done()
		wg.Add(1)

		if err := web.ListenAndServe(server, webConfig, logger); err != nil && !errors.Is(err, http.ErrServerClosed) {
			cancel(err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			stopChan <- true
			err := context.Cause(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return ReturnCodeOK
				}

				if errors.Is(err, ErrReload) {
					return ReturnCodeReload
				}

				logger.ErrorContext(ctx, err.Error())

				return ReturnCodeError
			}

			return ReturnCodeOK
		case sig := <-termCh:
			logger.LogAttrs(ctx, slog.LevelInfo, "receiving signal: "+sig.String())

			switch sig {
			case syscall.SIGHUP:
				logger.LogAttrs(ctx, slog.LevelInfo, "reloading configuration")
				cancel(ErrReload)
			default:
				cancel(nil)
			}
		}
	}
}

// initializeConfigAndLogger handles configuration parsing and logger setup.
func initializeConfigAndLogger(args []string, stdout io.Writer) (config.Config, *slog.Logger, ReturnCode) {
	conf, err := setupConfiguration(args, stdout)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return config.Config{}, nil, ReturnCodeOK
		}

		if errors.Is(err, config.ErrVersion) {
			printVersion(stdout)

			return config.Config{}, nil, ReturnCodeOK
		}

		_, _ = fmt.Fprintln(stdout, err.Error())

		return config.Config{}, nil, ReturnCodeError
	}

	logger, err := setupLogger(conf, stdout)
	if err != nil {
		_, _ = fmt.Fprintln(stdout, fmt.Errorf("error setupConfiguration logging: %w", err).Error())

		return config.Config{}, nil, ReturnCodeError
	}

	return conf, logger, ReturnCodeNoError
}

// setupConfiguration parses the command line arguments and loads the configuration.
func setupConfiguration(args []string, logWriter io.Writer) (config.Config, error) {
	conf, err := config.New(args, logWriter)
	if err != nil {
		return config.Config{}, fmt.Errorf("configuration error: %w", err)
	}

	if err = config.Validate(conf); err != nil {
		return config.Config{}, fmt.Errorf("configuration validation error: %w", err)
	}

	return conf, nil
}

func printVersion(writer io.Writer) {
	//goland:noinspection GoBoolExpressions
	if version.Version == "dev" {
		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			_, _ = fmt.Fprintf(writer, "version: %s\ngo: %s\n", buildInfo.Main.Version, buildInfo.GoVersion)

			return
		}
	}

	_, _ = fmt.Fprintln(writer, version.Print("prometheus_nginxlog_exporter"))
}

// setupLogger initializes the logger based on the configuration.
func setupLogger(conf config.Config, writer io.Writer) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{
		AddSource: false,
		Level:     conf.Log.Level,
	}

	switch conf.Log.Format {
	case "json":
		return slog.New(slog.NewJSONHandler(writer, opts)), nil
	case "console":
		return slog.New(slog.NewTextHandler(writer, opts)), nil
	default:
		return nil, fmt.Errorf("unknown log format: %s", conf.Log.Format)
	}
}

// startDebugListener starts the debug/pprof HTTP server in a goroutine.
func startDebugListener(ctx context.Context, cancel context.CancelCauseFunc, wg *sync.WaitGroup, logger *slog.Logger, conf config.Config) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		cancel(setupDebugListener(ctx, logger, conf))
	}()
}

// setupDebugListener sets up an HTTP server for debugging purposes, including pprof endpoints.
func setupDebugListener(ctx context.Context, logger *slog.Logger, conf config.Config) error {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.RedirectHandler("/debug/pprof/", http.StatusTemporaryRedirect))
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)

	server := &http.Server{
		Addr:              conf.Debug.ListenAddress,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       3 * time.Second,
		WriteTimeout:      1 * time.Minute,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		Handler:           mux,
	}

	go func() {
		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		server.RegisterOnShutdown(cancel)

		_ = server.Shutdown(ctx)
	}()

	err := server.ListenAndServe()
	if err != nil {
		return fmt.Errorf("error debug http listener: %w", err)
	}

	return nil
}
