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
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

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

		// Check if the path is public (either exact match or prefix match)
		isPublic := false
		if _, ok := public[uri.Path]; ok {
			isPublic = true
		} else {
			// Check for prefix matches (like /swagger/)
			for publicPath := range public {
				if strings.HasPrefix(uri.Path, publicPath) {
					isPublic = true
					break
				}
			}
		}

		if isPublic {
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

// AccountValidationMiddleware is a middleware function that validates the account parameter
// in the request URL. If the account is invalid, it returns a 400 Bad Request response.
// Otherwise, it allows the request to proceed to the next handler.
func (s *server) AccountValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the account parameter from the URL variables
		vars := mux.Vars(r)
		account := vars["account"]

		// Check if the provided account matches the expected account
		if s.bluecat.account != account {
			logger.Warn("Invalid account attempt",
				zap.String("providedAccount", account),
				zap.String("expectedAccount", s.bluecat.account))
			http.Error(w, "Invalid account", http.StatusBadRequest)
			return
		}

		// Proceed to the next handler since the account is valid
		logger.Info("Account validated successfully", zap.String("account", account))
		next.ServeHTTP(w, r)
	})
}
