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
	"dns-api-go/logger"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"net/http"
	"net/url"

	"golang.org/x/crypto/bcrypt"
)

// TokenMiddleware checks the tokens for non-public URLs
func TokenMiddleware(psk []byte, public map[string]string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Processing token middleware for protected URLs",
			zap.String("method", r.Method),
			zap.String("requestURI", r.RequestURI))

		// Handle CORS preflight checks
		if r.Method == "OPTIONS" {
			logger.Info("Setting CORS preflight options and returning")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "X-Auth-Token")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte{})
			return
		}

		uri, err := url.ParseRequestURI(r.RequestURI)
		if err != nil {
			logger.Error("Unable to parse request URI", zap.Error(err))
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if _, ok := public[uri.Path]; ok {
			logger.Debug("Not authenticating for public URL", zap.String("path", uri.Path))
		} else {
			logger.Debug("Authenticating token for protected URL", zap.String("URL", r.URL.String()))

			htoken := r.Header.Get("X-Auth-Token")
			if err := bcrypt.CompareHashAndPassword([]byte(htoken), psk); err != nil {
				logger.Warn("Unable to authenticate session for URL",
					zap.String("URL", r.URL.String()),
					zap.Error(err))
				w.WriteHeader(http.StatusForbidden)
				return
			}

			logger.Info("Successfully authenticated token for URL", zap.String("URL", r.URL.String()))
		}

		h.ServeHTTP(w, r)
	})
}

func (s *server) AccountValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		account, accountOk := vars["account"]
		if !accountOk {
			logger.Warn("Missing required parameter: account", zap.String("path", r.URL.Path))
			http.Error(w, "Missing required parameter: account", http.StatusBadRequest)
			return
		}

		if s.bluecat.account != account {
			logger.Warn("Invalid account attempt",
				zap.String("providedAccount", account),
				zap.String("expectedAccount", s.bluecat.account))
			http.Error(w, "Invalid account", http.StatusBadRequest)
			return
		}

		logger.Info("Account validated successfully", zap.String("account", account))
		next.ServeHTTP(w, r)
	})
}
