/*
Copyright 2026 eFellows Ltd.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/

package bigip

// Unit tests for the HA resources. They run the CRUD functions against a fake
// iControl REST server, so they need neither TF_ACC nor a BIG-IP.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	bigip "github.com/efellowsbg/go-bigip"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type haFakeCall struct {
	Method string
	Path   string
	Body   map[string]interface{}
}

type haFakeBigIP struct {
	t      *testing.T
	mu     sync.Mutex
	calls  []haFakeCall
	routes map[string]func(body map[string]interface{}) (int, interface{})
	server *httptest.Server
}

func newHAFakeBigIP(t *testing.T) *haFakeBigIP {
	f := &haFakeBigIP{t: t, routes: map[string]func(map[string]interface{}) (int, interface{}){}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		path := strings.TrimPrefix(r.URL.Path, "/mgmt/tm/")
		f.mu.Lock()
		f.calls = append(f.calls, haFakeCall{Method: r.Method, Path: path, Body: body})
		h, ok := f.routes[r.Method+" "+path]
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			w.WriteHeader(404)
			_, _ = fmt.Fprintf(w, `{"code":404,"message":"01020036:3: The requested object (%s) was not found."}`, path)
			return
		}
		status, resp := h(body)
		w.WriteHeader(status)
		if resp != nil {
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *haFakeBigIP) on(method, path string, h func(body map[string]interface{}) (int, interface{})) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[method+" "+path] = h
}

func (f *haFakeBigIP) off(method, path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.routes, method+" "+path)
}

func (f *haFakeBigIP) lastCall(method, path string) *haFakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.calls) - 1; i >= 0; i-- {
		if f.calls[i].Method == method && f.calls[i].Path == path {
			c := f.calls[i]
			return &c
		}
	}
	return nil
}

func (f *haFakeBigIP) client() *bigip.BigIP {
	return &bigip.BigIP{
		Host:          f.server.URL,
		User:          "admin",
		Password:      "secret",
		Transport:     &http.Transport{},
		ConfigOptions: &bigip.ConfigOptions{APICallTimeout: 5 * time.Second, APICallRetries: 1},
	}
}

func haFastPolling(t *testing.T) {
	old := haPollInterval
	haPollInterval = 10 * time.Millisecond
	t.Cleanup(func() { haPollInterval = old })
}

func haStats(fields map[string]string) map[string]interface{} {
	entries := map[string]interface{}{}
	for k, v := range fields {
		entries[k] = map[string]interface{}{"description": v}
	}
	return map[string]interface{}{"entries": map[string]interface{}{
		"https://localhost/mgmt/tm/cm/x/0": map[string]interface{}{"nestedStats": map[string]interface{}{"entries": entries}},
	}}
}

func ok(v interface{}) func(map[string]interface{}) (int, interface{}) {
	return func(map[string]interface{}) (int, interface{}) { return 200, v }
}

// ---------------------------------------------------------------- helpers

func TestHAHelpers(t *testing.T) {
	if got := haURIName("Common", "bigip1.a1.at"); got != "~Common~bigip1.a1.at" {
		t.Errorf("haURIName = %q", got)
	}
	if got := haURIName("Common", "/Part/tg"); got != "~Part~tg" {
		t.Errorf("haURIName full path = %q", got)
	}
	conf := map[string]bool{"bigip1": true, "/Common/bigip2": true}
	if got := haMatchNameForm("/Common/bigip1", conf); got != "bigip1" {
		t.Errorf("haMatchNameForm short = %q", got)
	}
	if got := haMatchNameForm("/Common/bigip2", conf); got != "/Common/bigip2" {
		t.Errorf("haMatchNameForm full = %q", got)
	}
	if haToInt("60") != 60 || haToInt(float64(5)) != 5 || !haToBool("true") || haToBool("false") || !haToBool(true) {
		t.Error("haToInt/haToBool conversion failed")
	}
	if code := haStatusFromError(nil, errors.New("HTTP 404 :: not here")); code != 404 {
		t.Errorf("haStatusFromError plain = %d", code)
	}
	if code := haStatusFromError([]byte(`{"code":409,"message":"exists"}`), errors.New("exists")); code != 409 {
		t.Errorf("haStatusFromError json = %d", code)
	}
}

// ---------------------------------------------------------------- device trust

func TestHADeviceTrustLifecycle(t *testing.T) {
	haFastPolling(t)
	f := newHAFakeBigIP(t)
	var mu sync.Mutex
	trusted := false
	f.on("GET", "cm/trust-domain/~Common~Root", func(map[string]interface{}) (int, interface{}) {
		mu.Lock()
		defer mu.Unlock()
		devs := []string{"/Common/bigip1"}
		if trusted {
			devs = append(devs, "/Common/bigip2")
		}
		return 200, map[string]interface{}{"caDevices": devs}
	})
	f.on("POST", "cm/add-to-trust", func(map[string]interface{}) (int, interface{}) {
		mu.Lock()
		trusted = true
		mu.Unlock()
		return 200, map[string]interface{}{}
	})
	f.on("POST", "cm/remove-from-trust", func(map[string]interface{}) (int, interface{}) {
		mu.Lock()
		trusted = false
		mu.Unlock()
		return 200, map[string]interface{}{}
	})

	r := resourceBigipCmDeviceTrust()
	d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{
		"peer_name":    "bigip2",
		"peer_address": "192.0.2.2",
		"username":     "admin",
		"password":     "pw",
	})
	if diags := r.CreateContext(context.Background(), d, f.client()); diags.HasError() {
		t.Fatalf("create: %v", diags)
	}
	if d.Id() != "bigip2" || !d.Get("ca_device").(bool) {
		t.Fatalf("unexpected state id=%q ca=%v", d.Id(), d.Get("ca_device"))
	}
	call := f.lastCall("POST", "cm/add-to-trust")
	want := map[string]interface{}{"command": "run", "name": "Root", "caDevice": true, "device": "192.0.2.2", "deviceName": "bigip2", "username": "admin", "password": "pw"}
	for k, v := range want {
		if call.Body[k] != v {
			t.Errorf("add-to-trust %s = %v, want %v", k, call.Body[k], v)
		}
	}

	if diags := r.DeleteContext(context.Background(), d, f.client()); diags.HasError() {
		t.Fatalf("delete: %v", diags)
	}
	if call := f.lastCall("POST", "cm/remove-from-trust"); call == nil || call.Body["deviceName"] != "bigip2" {
		t.Fatalf("remove-from-trust not called correctly: %+v", call)
	}

	// Read after the peer disappeared removes the resource from state.
	d.SetId("bigip2")
	if diags := r.ReadContext(context.Background(), d, f.client()); diags.HasError() || d.Id() != "" {
		t.Fatalf("read of missing peer: id=%q diags=%v", d.Id(), diags)
	}
}

// ---------------------------------------------------------------- device HA

func TestHADeviceHaLifecycle(t *testing.T) {
	f := newHAFakeBigIP(t)
	dev := map[string]interface{}{"name": "bigip1", "configsyncIp": "none", "mirrorIp": "any6", "mirrorSecondaryIp": "any6"}
	var mu sync.Mutex
	f.on("GET", "cm/device/~Common~bigip1", func(map[string]interface{}) (int, interface{}) {
		mu.Lock()
		defer mu.Unlock()
		return 200, dev
	})
	f.on("PATCH", "cm/device/~Common~bigip1", func(b map[string]interface{}) (int, interface{}) {
		mu.Lock()
		defer mu.Unlock()
		for k, v := range b {
			if k == "unicastAddress" && v == "none" {
				delete(dev, k)
				continue
			}
			dev[k] = v
		}
		return 200, dev
	})

	r := resourceBigipCmDeviceHa()
	d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{
		"name":          "bigip1",
		"configsync_ip": "10.0.0.1",
		"mirror_ip":     "10.0.0.1",
		"unicast_address": []interface{}{
			map[string]interface{}{"ip": "10.0.0.1"},
			map[string]interface{}{"ip": "management-ip", "port": 1026},
		},
	})
	if diags := r.CreateContext(context.Background(), d, f.client()); diags.HasError() {
		t.Fatalf("create: %v", diags)
	}
	patch := f.lastCall("PATCH", "cm/device/~Common~bigip1").Body
	if patch["configsyncIp"] != "10.0.0.1" || patch["mirrorIp"] != "10.0.0.1" {
		t.Errorf("unexpected PATCH body %v", patch)
	}
	if _, sent := patch["mirrorSecondaryIp"]; sent {
		t.Errorf("unset mirror_secondary_ip must not be sent: %v", patch)
	}
	ua := patch["unicastAddress"].([]interface{})
	first, second := ua[0].(map[string]interface{}), ua[1].(map[string]interface{})
	if first["ip"] != "10.0.0.1" || first["effectiveIp"] != "10.0.0.1" || first["port"].(float64) != 1026 {
		t.Errorf("unexpected unicast[0] %v", first)
	}
	if _, has := second["effectiveIp"]; has || second["ip"] != "management-ip" {
		t.Errorf("management-ip entry must not carry effectiveIp: %v", second)
	}
	if got := d.Get("unicast_address.1.ip"); got != "management-ip" {
		t.Errorf("state unicast_address.1.ip = %v", got)
	}
	if got := d.Get("mirror_secondary_ip"); got != "any6" {
		t.Errorf("computed mirror_secondary_ip = %v", got)
	}

	if diags := r.DeleteContext(context.Background(), d, f.client()); diags.HasError() {
		t.Fatalf("delete: %v", diags)
	}
	reset := f.lastCall("PATCH", "cm/device/~Common~bigip1").Body
	if reset["configsyncIp"] != "none" || reset["mirrorIp"] != "any6" || reset["unicastAddress"] != "none" {
		t.Errorf("unexpected reset body %v", reset)
	}

	// Creating on a device that does not exist fails with a clear message.
	d2 := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"name": "nope", "configsync_ip": "10.0.0.1"})
	diags := r.CreateContext(context.Background(), d2, f.client())
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "does not exist") {
		t.Errorf("expected 'does not exist' error, got %v", diags)
	}
}

// ---------------------------------------------------------------- traffic group

func TestHATrafficGroupLifecycle(t *testing.T) {
	f := newHAFakeBigIP(t)
	var mu sync.Mutex
	var tg map[string]interface{}
	uri := "cm/traffic-group/~Common~traffic-group-2"
	get := func(map[string]interface{}) (int, interface{}) {
		mu.Lock()
		defer mu.Unlock()
		return 200, tg
	}
	f.on("POST", "cm/traffic-group", func(b map[string]interface{}) (int, interface{}) {
		mu.Lock()
		defer mu.Unlock()
		tg = map[string]interface{}{"autoFailbackTime": 60, "haLoadFactor": 1, "mac": "none", "failoverMethod": "ha-order"}
		for k, v := range b {
			tg[k] = v
		}
		// BIG-IP returns device names as full paths.
		if order, ok := tg["haOrder"].([]interface{}); ok {
			tg["haOrder"] = order
		}
		f.on("GET", uri, get)
		return 200, tg
	})
	f.on("PATCH", uri, func(b map[string]interface{}) (int, interface{}) {
		mu.Lock()
		defer mu.Unlock()
		for k, v := range b {
			tg[k] = v
		}
		return 200, tg
	})
	f.on("DELETE", uri, func(map[string]interface{}) (int, interface{}) {
		f.off("GET", uri)
		return 200, nil
	})

	r := resourceBigipCmTrafficGroup()
	d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{
		"name":                  "traffic-group-2",
		"ha_order":              []interface{}{"bigip2", "bigip1"},
		"auto_failback_enabled": true,
		"auto_failback_time":    30,
	})
	if diags := r.CreateContext(context.Background(), d, f.client()); diags.HasError() {
		t.Fatalf("create: %v", diags)
	}
	post := f.lastCall("POST", "cm/traffic-group").Body
	if post["name"] != "traffic-group-2" || post["partition"] != "Common" || post["autoFailbackEnabled"] != "true" || post["autoFailbackTime"].(float64) != 30 {
		t.Errorf("unexpected POST body %v", post)
	}
	if order := post["haOrder"].([]interface{}); order[0] != "/Common/bigip2" || order[1] != "/Common/bigip1" {
		t.Errorf("ha_order must be sent as full paths: %v", order)
	}
	if d.Id() != "/Common/traffic-group-2" || d.Get("full_path") != "/Common/traffic-group-2" {
		t.Errorf("unexpected id %q", d.Id())
	}
	if got := d.Get("ha_order.0"); got != "bigip2" {
		t.Errorf("ha_order must keep the configured short form, got %v", got)
	}
	if !d.Get("auto_failback_enabled").(bool) || d.Get("mac") != "none" || d.Get("failover_method") != "ha-order" {
		t.Errorf("unexpected state after read: %v", d.State())
	}

	if diags := r.DeleteContext(context.Background(), d, f.client()); diags.HasError() {
		t.Fatalf("delete: %v", diags)
	}
	d.SetId("/Common/traffic-group-2")
	if diags := r.ReadContext(context.Background(), d, f.client()); diags.HasError() || d.Id() != "" {
		t.Fatalf("read after delete: id=%q diags=%v", d.Id(), diags)
	}
}

// ---------------------------------------------------------------- config sync

func TestHAConfigSync(t *testing.T) {
	haFastPolling(t)
	f := newHAFakeBigIP(t)
	var mu sync.Mutex
	polls := 0
	f.on("POST", "cm", ok(map[string]interface{}{}))
	f.on("GET", "cm/sync-status", func(map[string]interface{}) (int, interface{}) {
		mu.Lock()
		defer mu.Unlock()
		polls++
		if polls < 3 {
			return 200, haStats(map[string]string{"status": "Syncing", "summary": "busy"})
		}
		return 200, haStats(map[string]string{"status": "In Sync", "summary": "All devices in the device group are in sync"})
	})

	r := resourceBigipCmConfigSync()
	d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"device_group": "dg-gtp"})
	if diags := r.CreateContext(context.Background(), d, f.client()); diags.HasError() {
		t.Fatalf("create: %v", diags)
	}
	body := f.lastCall("POST", "cm").Body
	if body["command"] != "run" || body["utilCmdArgs"] != "config-sync to-group dg-gtp" {
		t.Errorf("unexpected sync body %v", body)
	}
	if d.Get("sync_status") != "In Sync" || polls < 3 {
		t.Errorf("did not wait for In Sync: status=%v polls=%d", d.Get("sync_status"), polls)
	}

	if cmd, _ := haSyncCommand("dg", "to-group", true); cmd != "config-sync force-full-load-push to-group dg" {
		t.Errorf("full-load command = %q", cmd)
	}
	if _, err := haSyncCommand("dg", "from-group", true); err == nil {
		t.Error("force_full_load_push with from-group must fail")
	}

	// A sync failure is reported instead of waiting for the timeout.
	f.on("GET", "cm/sync-status", ok(haStats(map[string]string{"status": "Sync Failure", "summary": "boom"})))
	d2 := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"device_group": "dg-gtp"})
	diags := r.CreateContext(context.Background(), d2, f.client())
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "Sync Failure") {
		t.Errorf("expected Sync Failure error, got %v", diags)
	}
}

// ---------------------------------------------------------------- failover

func TestHASysFailover(t *testing.T) {
	haFastPolling(t)
	f := newHAFakeBigIP(t)
	var mu sync.Mutex
	state := "ACTIVE"
	f.on("POST", "sys/failover", func(b map[string]interface{}) (int, interface{}) {
		mu.Lock()
		defer mu.Unlock()
		if b["standby"] == true {
			state = "STANDBY"
		}
		return 200, map[string]interface{}{}
	})
	f.on("GET", "cm/failover-status", func(map[string]interface{}) (int, interface{}) {
		mu.Lock()
		defer mu.Unlock()
		return 200, haStats(map[string]string{"status": state, "color": "green"})
	})

	r := resourceBigipSysFailover()
	d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"action": "standby"})
	if diags := r.CreateContext(context.Background(), d, f.client()); diags.HasError() {
		t.Fatalf("create: %v", diags)
	}
	if body := f.lastCall("POST", "sys/failover").Body; body["command"] != "run" || body["standby"] != true {
		t.Errorf("unexpected failover body %v", body)
	}
	if d.Get("failover_state") != "STANDBY" {
		t.Errorf("failover_state = %v", d.Get("failover_state"))
	}

	d2 := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"action": "standby", "traffic_group": "traffic-group-1"})
	if diags := r.CreateContext(context.Background(), d2, f.client()); diags.HasError() {
		t.Fatalf("create with traffic group: %v", diags)
	}
	if body := f.lastCall("POST", "sys/failover").Body; body["trafficGroup"] != "traffic-group-1" {
		t.Errorf("trafficGroup not sent: %v", body)
	}

	d3 := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{"action": "offline", "traffic_group": "traffic-group-1"})
	if diags := r.CreateContext(context.Background(), d3, f.client()); !diags.HasError() {
		t.Error("traffic_group with action offline must fail")
	}
}

// ---------------------------------------------------------------- provider schema

func TestHAResourcesRegistered(t *testing.T) {
	p := Provider()
	for _, name := range []string{"bigip_cm_device_trust", "bigip_cm_device_ha", "bigip_cm_traffic_group", "bigip_cm_config_sync", "bigip_sys_failover"} {
		if _, ok := p.ResourcesMap[name]; !ok {
			t.Errorf("%s not registered in provider", name)
		}
	}
	if err := p.InternalValidate(); err != nil {
		t.Fatalf("provider schema invalid: %v", err)
	}
}

// ---------------------------------------------------------------- wait helper

func TestHAWaitFor(t *testing.T) {
	haFastPolling(t)
	ctx := context.Background()

	// A stop error ends the wait at once, well before the timeout.
	start := time.Now()
	err := haWaitFor(ctx, time.Minute, "x", func() (bool, error) { return false, haStop(errors.New("fatal")) })
	if err == nil || !strings.Contains(err.Error(), "fatal") || time.Since(start) > 5*time.Second {
		t.Errorf("stop error not honoured: %v after %s", err, time.Since(start))
	}

	// Transient errors are retried and reported on timeout.
	err = haWaitFor(ctx, 50*time.Millisecond, "x", func() (bool, error) { return false, errors.New("flaky") })
	if err == nil || !strings.Contains(err.Error(), "timed out") || !strings.Contains(err.Error(), "flaky") {
		t.Errorf("unexpected timeout error: %v", err)
	}

	// Success after a few transient failures.
	n := 0
	err = haWaitFor(ctx, time.Minute, "x", func() (bool, error) {
		n++
		if n < 3 {
			return false, errors.New("flaky")
		}
		return true, nil
	})
	if err != nil || n != 3 {
		t.Errorf("expected success after 3 polls, got err=%v n=%d", err, n)
	}
}
