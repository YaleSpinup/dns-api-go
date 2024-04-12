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
	"encoding/json"
	"net/http"

	"github.com/YaleSpinup/apierror"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// PingHandler responds to ping requests
func (s *server) PingHandler(w http.ResponseWriter, r *http.Request) {
	w = LogWriter{w}
	log.Debug("Ping/Pong")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

// VersionHandler responds to version requests
func (s *server) VersionHandler(w http.ResponseWriter, r *http.Request) {
	w = LogWriter{w}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	data, err := json.Marshal(s.version)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte{})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// handleError handles standard apierror return codes
func handleError(w http.ResponseWriter, err error) {
	log.Error(err.Error())
	if aerr, ok := errors.Cause(err).(apierror.Error); ok {
		switch aerr.Code {
		case apierror.ErrForbidden:
			w.WriteHeader(http.StatusForbidden)
		case apierror.ErrNotFound:
			w.WriteHeader(http.StatusNotFound)
		case apierror.ErrConflict:
			w.WriteHeader(http.StatusConflict)
		case apierror.ErrBadRequest:
			w.WriteHeader(http.StatusBadRequest)
		case apierror.ErrLimitExceeded:
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write([]byte(aerr.Message))
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
}
