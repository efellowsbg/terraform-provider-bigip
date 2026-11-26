/*
Copyright 2026 eFellows Ltd.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/

package bigip

// Shared helpers for the HA / device-cluster resources (device trust, device HA
// addresses, traffic groups, config sync, failover). These resources talk to
// iControl REST directly through client.APICall so that no change to the
// go-bigip client library is required.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	bigip "github.com/efellowsbg/go-bigip"
)

// haAPIError wraps an iControl REST error and keeps the HTTP status code that
// BIG-IP reports in the JSON error body (go-bigip drops it).
type haAPIError struct {
	Code int
	Err  error
}

func (e *haAPIError) Error() string { return e.Err.Error() }
func (e *haAPIError) Unwrap() error { return e.Err }

// isHANotFound reports whether err is an iControl REST "404 not found".
func isHANotFound(err error) bool {
	var apiErr *haAPIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == 404
	}
	return false
}

// haRequest performs an iControl REST call. uri is relative to /mgmt/tm/
// (for example "cm/traffic-group/~Common~tg1"). body is JSON-encoded when not nil.
func haRequest(client *bigip.BigIP, method, uri string, body interface{}) ([]byte, error) {
	payload := ""
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body for %s %s: %w", method, uri, err)
		}
		payload = string(b)
	}
	resp, err := client.APICall(&bigip.APIRequest{
		Method:      method,
		URL:         uri,
		Body:        payload,
		ContentType: "application/json",
	})
	if err != nil {
		return resp, &haAPIError{Code: haStatusFromError(resp, err), Err: err}
	}
	return resp, nil
}

// haGet performs a GET and decodes the JSON response into out.
func haGet(client *bigip.BigIP, uri string, out interface{}) error {
	resp, err := haRequest(client, "GET", uri, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(resp, out); err != nil {
		return fmt.Errorf("decoding response of GET %s: %w", uri, err)
	}
	return nil
}

func haStatusFromError(resp []byte, err error) int {
	var reqErr struct {
		Code int `json:"code"`
	}
	if len(resp) > 0 && json.Unmarshal(resp, &reqErr) == nil && reqErr.Code != 0 {
		return reqErr.Code
	}
	// Non-JSON errors are formatted by go-bigip as "HTTP <code> :: <body>".
	msg := err.Error()
	if strings.HasPrefix(msg, "HTTP ") {
		if code, convErr := strconv.Atoi(strings.Fields(msg)[1]); convErr == nil {
			return code
		}
	}
	return 0
}

// haFullPath returns "/<partition>/<name>" unless name is already a full path.
func haFullPath(partition, name string) string {
	if strings.HasPrefix(name, "/") {
		return name
	}
	if partition == "" {
		partition = "Common"
	}
	return "/" + partition + "/" + name
}

// haURIName converts "/Common/name" (or a bare name) into the "~Common~name"
// form used in iControl REST URLs.
func haURIName(partition, name string) string {
	return strings.ReplaceAll(haFullPath(partition, name), "/", "~")
}

// haShortName strips the partition from a full path ("/Common/x" -> "x").
func haShortName(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// haMatchNameForm returns value in the same form (full path or bare name) the
// user configured, so "/Common/bigip1" read back from BIG-IP does not produce a
// diff against a configured "bigip1".
func haMatchNameForm(value string, configured map[string]bool) string {
	if configured[value] {
		return value
	}
	if short := haShortName(value); configured[short] {
		return short
	}
	return value
}

// haToInt converts JSON numbers or numeric strings to int.
func haToInt(v interface{}) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		i, _ := strconv.Atoi(t)
		return i
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	}
	return 0
}

// haToBool converts the many boolean spellings iControl REST uses to bool.
func haToBool(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(t) {
		case "true", "yes", "enabled":
			return true
		}
	}
	return false
}

// haPollInterval is how often the HA resources poll BIG-IP while waiting for
// an asynchronous operation (trust establishment, config sync, failover).
// Unit tests shorten it.
var haPollInterval = 3 * time.Second

// haStopError marks an error returned by a haWaitFor check as final: waiting
// stops immediately instead of retrying until the timeout.
type haStopError struct{ err error }

func (e *haStopError) Error() string { return e.err.Error() }
func (e *haStopError) Unwrap() error { return e.err }

func haStop(err error) error { return &haStopError{err: err} }

// haWaitFor calls check every haPollInterval until it returns true, the context
// is cancelled, or timeout elapses. Errors from check are treated as transient
// (retried) unless wrapped with haStop.
func haWaitFor(ctx context.Context, timeout time.Duration, what string, check func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		done, err := check()
		if done && err == nil {
			return nil
		}
		var stop *haStopError
		if errors.As(err, &stop) {
			return fmt.Errorf("waiting for %s: %w", what, stop.err)
		}
		lastErr = err
		if time.Now().After(deadline) {
			if lastErr != nil {
				return fmt.Errorf("timed out after %s waiting for %s: %w", timeout, what, lastErr)
			}
			return fmt.Errorf("timed out after %s waiting for %s", timeout, what)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for %s: %w", what, ctx.Err())
		case <-time.After(haPollInterval):
		}
	}
}

// haStatsDescriptions flattens an iControl REST stats response
// ({"entries":{"<url>":{"nestedStats":{"entries":{"status":{"description":"In Sync"}}}}}})
// into a map of the first entry's field -> description/value.
func haStatsDescriptions(resp []byte) (map[string]string, error) {
	var stats struct {
		Entries map[string]struct {
			NestedStats struct {
				Entries map[string]struct {
					Description *string  `json:"description"`
					Value       *float64 `json:"value"`
				} `json:"entries"`
			} `json:"nestedStats"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(resp, &stats); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, entry := range stats.Entries {
		for k, v := range entry.NestedStats.Entries {
			switch {
			case v.Description != nil:
				out[k] = *v.Description
			case v.Value != nil:
				out[k] = strconv.FormatFloat(*v.Value, 'f', -1, 64)
			}
		}
		break // sync-status and failover-status have a single top-level entry
	}
	return out, nil
}
