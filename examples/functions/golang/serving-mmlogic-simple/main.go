/*
This application handles all the startup and connection scaffolding for
running a gRPC server serving the APIService as defined in
${OM_ROOT}/internal/pb/function.pb.go

NOTE: This code will likely all get moved refactoring is done and the code
for spinning up a gRPC API is DRYed out.

All the actual important bits are in the API Server source code: apisrv/

Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/GoogleCloudPlatform/open-match/config"
	"github.com/GoogleCloudPlatform/open-match/examples/functions/golang/serving-mmlogic-simple/apisrv"
	"github.com/GoogleCloudPlatform/open-match/internal/metrics"
	api "github.com/GoogleCloudPlatform/open-match/internal/pb"
	redisHelpers "github.com/GoogleCloudPlatform/open-match/internal/statestorage/redis"
	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	// Logrus structured logging setup
	fnLogFields = log.Fields{
		"app":       "openmatch",
		"component": "function_service",
	}
	fnLog = log.WithFields(fnLogFields)

	// Viper config management setup
	cfg = viper.New()
	err = errors.New("")
)

func init() {
	// Add a hook to the logger to auto-count log lines for metrics output thru OpenCensus
	log.AddHook(metrics.NewMmfHook(apisrv.FnLogLines, apisrv.KeySeverity, apisrv.KeyFnName))

	// Viper config management initialization
	cfg, err = config.Read()
	if err != nil {
		fnLog.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Unable to load config file")
	}

	if cfg.IsSet("debug") && cfg.GetBool("debug") {
		fnLog.Warn("Debug logging configured. Not recommended for production!")
	}

	// Configure OpenCensus exporter to Prometheus
	// metrics.ConfigureOpenCensusPrometheusExporter expects that every OpenCensus view you
	// want to register is in an array, so append any views you want from other
	// packages to a single array here.
	ocServerViews := apisrv.DefaultFunctionViews
	ocServerViews = append(ocServerViews, ocgrpc.DefaultServerViews...)
	ocServerViews = append(ocServerViews, config.CfgVarCountView) // config loader view.
	// Waiting on https://github.com/opencensus-integrations/redigo/pull/1
	// ocServerViews = append(ocServerViews, redis.ObservabilityMetricViews...) // redis OpenCensus views.
	fnLog.WithFields(log.Fields{"viewscount": len(ocServerViews)}).Info("Loaded OpenCensus views")
	metrics.ConfigureOpenCensusPrometheusExporter(cfg, ocServerViews)
}

func main() {
	// Attempt to connect to MMLogic API. Assumes that this FunctionServer is
	// running in the same k8s namespace as the MMLogic Service. Since MMLogic
	// API is optional, all this code should be written to execute successfully
	// and log  warnings if the MMLogic API is not running.
	// TODO: probably put this in a helper function ala redisHelpers.ConnectionPool(cfg)
	var client api.MmLogicClient
	ip, err := net.LookupHost(cfg.GetString("api.mmlogic.hostname"))
	if err != nil || len(ip) == 0 {
		fnLog.Warning("Couldn't get IP for MMLogic API from environment! Have you started the MMLogic API yet?")
	} else {
		port := cfg.GetString("api.mmlogic.port")
		if len(port) == 0 {
			fnLog.Warning("Couldn't get port for MMLogic API from environment! Have you started the MMLogic API yet?")
		} else {

			//Connect
			conn, err := grpc.Dial(fmt.Sprintf("%v:%v", ip, port), grpc.WithInsecure())
			if err != nil {
				fnLog.Warning("failed to connect: ", err.Error())
			} else {
				//func(t time.Time) *time.Time { return &t }(time.Now())
				client = api.NewMmLogicClient(conn)
				fnLog.Info("Connected to MMLogic API")
			}
		}
	}

	// Connect to redis
	pool, err := redisHelpers.ConnectionPool(cfg)
	defer pool.Close()

	// Instantiate the gRPC server with the connections we've made
	fnLog.Info("Attempting to start gRPC server")
	srv := apisrv.New(cfg, pool, client)

	// Run the gRPC server
	err = srv.Open()
	if err != nil {
		fnLog.WithFields(log.Fields{"error": err.Error()}).Fatal("Failed to start gRPC server")
	}

	// Exit when we see a signal
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
	fnLog.Info("Shutting down gRPC server")
}