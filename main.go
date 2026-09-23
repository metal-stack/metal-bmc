package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	"github.com/metal-stack/metal-bmc/internal/bmc"
	"github.com/metal-stack/metal-bmc/internal/bmcv2"
	"github.com/metal-stack/metal-bmc/pkg/config"
	metalgo "github.com/metal-stack/metal-go"

	"github.com/metal-stack/metal-bmc/internal/reporter"
	"github.com/metal-stack/v"

	"github.com/kelseyhightower/envconfig"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var cfg config.Config
	if err := envconfig.Process("METAL_BMC", &cfg); err != nil {
		panic(fmt.Errorf("bad configuration: %w", err))
	}

	if err := cfg.Validate(); err != nil {
		panic(fmt.Errorf("bad configuration: %w", err))
	}

	level := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "error":
		level = slog.LevelError
	case "warn":
		level = slog.LevelWarn
	}

	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	log := slog.New(jsonHandler)

	log.Info("running app version", "version", v.V.String())
	log.Info("configuration", "config", cfg)

	v1client, err := metalgo.NewDriver(cfg.MetalAPIURL.String(), "", cfg.MetalAPIHMACKey, metalgo.AuthType("Metal-Edit"))
	if err != nil {
		log.Error("unable to create metal-api client", "error", err)
		panic(err)
	}

	tokenPersister, err := client.NewFilesystemTokenPersister(cfg.TokenFile)
	if err != nil {
		log.Error("error creating token persister", "error", err)
		panic(err)
	}

	token, err := os.ReadFile(cfg.TokenFile)
	if err != nil {
		log.Error("error reading token", "error", err)
		panic(err)
	}

	// keep alives are quite low on metal-apiserver, we need to apply them in order not
	// to receive regular stream timeouts
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 5 * time.Second,
		}).DialContext,
		IdleConnTimeout: 10 * time.Second,
		MaxIdleConns:    1,
	}

	v2client, err := client.New(&client.DialConfig{
		BaseURL:   cfg.MetalAPIServerURL,
		Token:     strings.TrimSpace(string(token)),
		UserAgent: "metal-bmc",
		Log:       log,
		TokenRenewal: &client.TokenRenewal{
			PersistTokenFn: tokenPersister,
		},
		Transport: transport,
	})
	if err != nil {
		log.Error("failed to create metal-apiserver client", "error", err)
		panic(err)
	}

	// Ping apiserver every 5min
	v2client.Ping(ctx, &client.PingConfig{
		ComponentType: apiv2.ComponentType_COMPONENT_TYPE_METAL_BMC,
		StartedAt:     time.Now(),
		Version: apiv2.Version{
			Version:   v.Version,
			Revision:  v.Revision,
			GitSha1:   v.GitSHA1,
			BuildDate: v.BuildDate,
		},
	})

	// BMC Events via NSQ
	b := bmc.New(log, &cfg)

	err = b.InitConsumer()
	if err != nil {
		log.Error("unable to create bmc service", "error", err)
		panic(err)
	}

	// BMC Console access
	console, err := bmc.NewConsole(log, v1client, cfg)
	if err != nil {
		log.Error("unable to create bmc console", "error", err)
		panic(err)
	}
	go func() {
		err := console.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()

	bmcv2.New(log, v2client, &cfg).ProcessCommands(ctx)

	// TODO: implement v2 console, we really want bidi streams now because we do not want to open a second server listener

	// Report IPMI Details
	r, err := reporter.New(log, &cfg, v2client)
	if err != nil {
		log.Error("could not start reporter", "error", err)
		panic(err)
	}

	r.Run(ctx)
}
