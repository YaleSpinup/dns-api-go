/*
Copyright © 2023 Yale University

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package api

import (
	"context"
	"dns-api-go/internal/common"
	"dns-api-go/internal/services"
	"dns-api-go/logger"
	"errors"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// apiVersion is the API version
type apiVersion struct {
	// The version of the API
	Version string `json:"version"`
	// The git hash of the API
	GitHash string `json:"githash"`
	// The build timestamp of the API
	BuildStamp string `json:"buildstamp"`
}

type proxyBackend struct {
	baseUrl string
	token   string
	prefix  string
}

type bluecat struct {
	account   string
	baseUrl   string
	user      string
	password  string
	token     string
	tokenLock sync.Mutex
}

type Services struct {
	BaseService *services.BaseService
}

type server struct {
	router   *mux.Router
	version  *apiVersion
	context  context.Context
	backend  *proxyBackend
	bluecat  *bluecat
	org      string
	services Services
}

// NewServer creates a new server and starts it
func NewServer(config common.Config) error {
	// setup server context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if config.Org == "" {
		return errors.New("'org' cannot be empty in the configuration")
	}

	s := server{
		router:  mux.NewRouter(),
		context: ctx,
		org:     config.Org,
	}

	s.version = &apiVersion{
		Version:    config.Version.Version,
		GitHash:    config.Version.GitHash,
		BuildStamp: config.Version.BuildStamp,
	}

	if b := config.Bluecat; b != nil {
		logger.Debug("configuring bluecat", zap.String("baseUrl", b.BaseUrl))
		s.bluecat = &bluecat{
			account:  b.Account,
			baseUrl:  b.BaseUrl,
			user:     b.Username,
			password: b.Password,
		}
	}

	// Define services that interact with Bluecat entities
	baseService := services.NewBaseService(&s)
	s.services = Services{
		BaseService: baseService,
	}

	if b := config.ProxyBackend; b != nil {
		logger.Debug("configuring proxy backend", zap.String("baseUrl", b.BaseUrl))
		s.backend = &proxyBackend{
			baseUrl: b.BaseUrl,
			token:   b.Token,
			prefix:  b.BackendPrefix,
		}
	}

	publicURLs := map[string]string{
		"/v2/dns/ping":    "public",
		"/v2/dns/version": "public",
		"/v2/dns/metrics": "public",
	}

	// load routes
	s.routes()

	if config.ListenAddress == "" {
		config.ListenAddress = ":8080"
	}
	handler := handlers.RecoveryHandler()(handlers.LoggingHandler(os.Stdout, TokenMiddleware([]byte(config.Token), publicURLs, s.router)))
	srv := &http.Server{
		Handler:      handler,
		Addr:         config.ListenAddress,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	logger.Info("Starting listener", zap.String("address", config.ListenAddress))
	if err := srv.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

// LogWriter is an http.ResponseWriter
type LogWriter struct {
	http.ResponseWriter
}

// Write log message if http response writer returns an error
func (w LogWriter) Write(p []byte) (n int, err error) {
	n, err = w.ResponseWriter.Write(p)
	if err != nil {
		logger.Error("Write failed", zap.Error(err))
	}
	return
}

type rollbackFunc func(ctx context.Context) error

// rollBack executes functions from a stack of rollback functions
func rollBack(t *[]rollbackFunc) {
	if t == nil {
		return
	}

	timeout, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	done := make(chan string, 1)
	go func() {
		tasks := *t
		logger.Error("executing rollback of tasks", zap.Int("taskCount", len(tasks)))
		for i := len(tasks) - 1; i >= 0; i-- {
			f := tasks[i]
			if funcerr := f(timeout); funcerr != nil {
				logger.Error("rollback task error, continuing rollback", zap.Error(funcerr))
			}
			logger.Info("executed rollback task", zap.Int("currentTask", len(tasks)-i), zap.Int("totalTasks", len(tasks)))
		}
		done <- "success"
	}()

	// wait for a done context
	select {
	case <-timeout.Done():
		logger.Error("timeout waiting for successful rollback")
	case <-done:
		logger.Info("successfully rolled back")
	}
}

type stop struct {
	error
}

// retry was originally borrowed from https://upgear.io/blog/simple-golang-retry-function/
// it will retry the given code _f_ for the specified number of _attempts_
// sleeping after each attempt, starting with _sleep_ time and doubling it
// for the first _doubling_ times, then keeping it constant (+ jitter)
func retry(attempts int, doubling int, sleep time.Duration, f func() error) error {
	if err := f(); err != nil {
		if s, ok := err.(stop); ok {
			// return the original error for later checking
			return s.error
		}

		if attempts--; attempts > 0 {
			// add some randomness to prevent creating a Thundering Herd
			jitter := time.Duration(rand.Int63n(int64(sleep)))
			sleep = sleep + jitter/2

			time.Sleep(sleep)
			if doubling--; doubling > 0 {
				return retry(attempts, doubling, 2*sleep, f)
			}
			// stop doubling the sleep interval after some point to prevent it from getting too big
			return retry(attempts, 0, sleep, f)
		}
		return err
	}

	return nil
}
