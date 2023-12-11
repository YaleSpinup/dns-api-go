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
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"os"
	"time"

	"dns-api-go/common"
	"dns-api-go/iam"
	"dns-api-go/session"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"

	log "github.com/sirupsen/logrus"
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

type server struct {
	router       *mux.Router
	version      *apiVersion
	context      context.Context
	session      session.Session
	sessionCache *cache.Cache
	orgPolicy    string
	org          string
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
		router:       mux.NewRouter(),
		context:      ctx,
		org:          config.Org,
		sessionCache: cache.New(600*time.Second, 900*time.Second),
	}

	s.version = &apiVersion{
		Version:    config.Version.Version,
		GitHash:    config.Version.GitHash,
		BuildStamp: config.Version.BuildStamp,
	}

	orgPolicy, err := orgTagAccessPolicy(config.Org)
	if err != nil {
		return err
	}
	s.orgPolicy = orgPolicy

	// Create a new session used for authentication and assuming cross account roles
	log.Debugf("Creating new session with key '%s' in region '%s'", config.Account.Akid, config.Account.Region)
	s.session = session.New(
		session.WithCredentials(config.Account.Akid, config.Account.Secret, ""),
		session.WithRegion(config.Account.Region),
		session.WithExternalID(config.Account.ExternalID),
		session.WithExternalRoleName(config.Account.Role),
	)

	publicURLs := map[string]string{
		"/v1/test/ping":    "public",
		"/v1/test/version": "public",
		"/v1/test/metrics": "public",
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

	log.Infof("Starting listener on %s", config.ListenAddress)
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
		log.Errorf("Write failed: %v", err)
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
		log.Errorf("executing rollback of %d tasks", len(tasks))
		for i := len(tasks) - 1; i >= 0; i-- {
			f := tasks[i]
			if funcerr := f(timeout); funcerr != nil {
				log.Errorf("rollback task error: %s, continuing rollback", funcerr)
			}
			log.Infof("executed rollback task %d of %d", len(tasks)-i, len(tasks))
		}
		done <- "success"
	}()

	// wait for a done context
	select {
	case <-timeout.Done():
		log.Error("timeout waiting for successful rollback")
	case <-done:
		log.Info("successfully rolled back")
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

// orgTagAccessPolicy generates the org tag conditional policy to be passed inline when assuming a role
func orgTagAccessPolicy(org string) (string, error) {
	log.Debugf("generating org policy document")

	policy := iam.PolicyDocument{
		Version: "2012-10-17",
		Statement: []iam.StatementEntry{
			{
				Effect:   "Allow",
				Action:   []string{"*"},
				Resource: "*",
				Condition: iam.Condition{
					"StringEquals": iam.ConditionStatement{
						"aws:ResourceTag/spinup:org": org,
					},
				},
			},
		},
	}

	j, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}

	return string(j), nil
}
