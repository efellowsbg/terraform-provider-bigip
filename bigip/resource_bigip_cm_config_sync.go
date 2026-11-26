/*
Copyright 2026 eFellows Ltd.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/

package bigip

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	bigip "github.com/efellowsbg/go-bigip"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const (
	uriCm               = "cm"
	uriCmSyncStatus     = "cm/sync-status"
	haSyncStatusInSync  = "In Sync"
	haSyncStatusStandby = "Standalone"
)

// haSyncFailureStatuses are sync-status values after which waiting is pointless.
var haSyncFailureStatuses = []string{"Sync Failure"}

// haSyncRetryAfter is how long to wait for a group to reach "In Sync" before
// re-sending the sync command. BIG-IP silently skips a sync requested less than
// 30 seconds after the previous one (e.g. right after the device group or trust
// was created), so a single command can be lost.
var haSyncRetryAfter = 35 * time.Second

// resourceBigipCmConfigSync is an action resource: creating it (or changing
// triggers) runs `tmsh run cm config-sync <direction> <device_group>` and
// optionally waits until that device group is in sync. Other groups (e.g. the
// built-in datasync-* groups) do not affect the wait. Destroying it does
// nothing on the BIG-IP.
func resourceBigipCmConfigSync() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBigipCmConfigSyncCreate,
		ReadContext:   resourceBigipCmConfigSyncRead,
		DeleteContext: resourceBigipCmConfigSyncDelete,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"device_group": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Sync-failover or sync-only device group to synchronise",
			},
			"direction": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "to-group",
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"to-group", "from-group"}, false),
				Description:  "to-group pushes this device's configuration to the group; from-group pulls the group's configuration onto this device",
			},
			"force_full_load_push": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Push the full configuration instead of an incremental sync (to-group only)",
			},
			"wait_for_sync": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				ForceNew:    true,
				Description: "Wait until the device group is in sync (all members hold the same commit)",
			},
			"triggers": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Arbitrary values; any change re-runs the sync (similar to null_resource triggers)",
			},
			"sync_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Sync status of the device group (In Sync, or the status BIG-IP reports for it, e.g. Changes Pending)",
			},
			"sync_summary": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Sync status detail for the device group",
			},
		},
	}
}

func haSyncCommand(group, direction string, fullLoad bool) (string, error) {
	if fullLoad {
		if direction != "to-group" {
			return "", fmt.Errorf("force_full_load_push requires direction = \"to-group\"")
		}
		return "config-sync force-full-load-push to-group " + group, nil
	}
	return fmt.Sprintf("config-sync %s %s", direction, group), nil
}

// haStatsEntry is the generic iControl REST stats node: a leaf with a
// description or value, or nestedStats with further entries.
type haStatsEntry struct {
	Description *string                 `json:"description"`
	Value       *float64                `json:"value"`
	NestedStats *haStatsNested          `json:"nestedStats"`
	Entries     map[string]haStatsEntry `json:"entries"`
}

type haStatsNested struct {
	Entries map[string]haStatsEntry `json:"entries"`
}

// haSyncGroupDetail returns the status and text of the device group's line in
// the cm sync-status details ("<group> (<status>): <text>"), or "" when the
// group is not listed (BIG-IP lists only groups that are not in sync).
func haSyncGroupDetail(resp []byte, group string) (status, text string, err error) {
	var root haStatsEntry
	if err := json.Unmarshal(resp, &root); err != nil {
		return "", "", fmt.Errorf("decoding sync status: %w", err)
	}
	short := haShortName(group)
	var found bool
	var walk func(map[string]haStatsEntry)
	walk = func(entries map[string]haStatsEntry) {
		for k, e := range entries {
			if found {
				return
			}
			if k == "details" && e.Description != nil {
				line := strings.TrimSpace(*e.Description)
				for _, name := range []string{short, haFullPath("Common", short)} {
					if rest, ok := strings.CutPrefix(line, name+" ("); ok {
						if i := strings.Index(rest, ")"); i >= 0 {
							status = rest[:i]
							text = strings.TrimSpace(strings.TrimPrefix(rest[i+1:], ":"))
							found = true
							return
						}
					}
				}
			}
			if e.NestedStats != nil {
				walk(e.NestedStats.Entries)
			}
		}
	}
	walk(root.Entries)
	return status, text, nil
}

// haGroupCommits reads the per-member commit (originator + time) of a device
// group from its stats. A group is in sync when every member holds the same
// commit.
func haGroupCommits(client *bigip.BigIP, group string) (map[string]string, error) {
	resp, err := haRequest(client, "GET", "cm/device-group/"+haURIName("Common", group)+"/stats", nil)
	if err != nil {
		return nil, err
	}
	var root haStatsEntry
	if err := json.Unmarshal(resp, &root); err != nil {
		return nil, fmt.Errorf("decoding device group stats: %w", err)
	}
	commits := map[string]string{}
	for _, member := range root.Entries {
		if member.NestedStats == nil {
			continue
		}
		f := map[string]string{}
		for k, v := range member.NestedStats.Entries {
			if v.Description != nil {
				f[k] = *v.Description
			}
		}
		if f["device"] != "" {
			commits[f["device"]] = f["commitIdOriginator"] + "@" + f["commitIdTime"]
		}
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("device group %s reports no members", group)
	}
	return commits, nil
}

// haReadGroupSync reports whether the device group is in sync, the status to
// expose (In Sync or BIG-IP's status for the group) and a detail text.
func haReadGroupSync(client *bigip.BigIP, group string) (inSync bool, status, detail string, err error) {
	resp, err := haRequest(client, "GET", uriCmSyncStatus, nil)
	if err != nil {
		return false, "", "", err
	}
	lineStatus, lineText, err := haSyncGroupDetail(resp, group)
	if err != nil {
		return false, "", "", err
	}
	commits, err := haGroupCommits(client, group)
	if err != nil {
		return false, "", "", err
	}
	var first string
	inSync = true
	for _, c := range commits {
		if first == "" {
			first = c
		} else if c != first {
			inSync = false
		}
	}
	switch {
	case inSync && lineStatus == "":
		return true, haSyncStatusInSync, fmt.Sprintf("%d members hold the same commit", len(commits)), nil
	case lineStatus != "":
		return inSync && strings.EqualFold(lineStatus, haSyncStatusInSync), lineStatus, lineText, nil
	default:
		return false, "Changes Pending", "members hold different commits", nil
	}
}

func resourceBigipCmConfigSyncCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	group := d.Get("device_group").(string)

	cmd, err := haSyncCommand(group, d.Get("direction").(string), d.Get("force_full_load_push").(bool))
	if err != nil {
		return diag.FromErr(err)
	}
	run := func() error {
		log.Printf("[INFO] Running: tmsh run cm %s", cmd)
		if _, err := haRequest(client, "POST", uriCm, map[string]interface{}{
			"command":     "run",
			"utilCmdArgs": cmd,
		}); err != nil {
			return fmt.Errorf("running %q: %w", cmd, err)
		}
		return nil
	}
	if err := run(); err != nil {
		return diag.FromErr(err)
	}

	if d.Get("wait_for_sync").(bool) {
		lastRun := time.Now()
		err := haWaitFor(ctx, d.Timeout(schema.TimeoutCreate), fmt.Sprintf("device group %s to be In Sync", group), func() (bool, error) {
			inSync, status, detail, err := haReadGroupSync(client, group)
			if err != nil {
				return false, err
			}
			for _, failed := range haSyncFailureStatuses {
				if strings.EqualFold(status, failed) {
					return false, haStop(fmt.Errorf("sync status of %s is %q: %s", group, status, detail))
				}
			}
			if inSync {
				return true, nil
			}
			// BIG-IP may have skipped the command (30 second rule); send it again.
			if time.Since(lastRun) >= haSyncRetryAfter {
				log.Printf("[INFO] %s still %q after %s, re-sending sync", group, status, haSyncRetryAfter)
				if err := run(); err != nil {
					return false, err
				}
				lastRun = time.Now()
			}
			return false, fmt.Errorf("sync status of %s is %q: %s", group, status, detail)
		})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(fmt.Sprintf("%s-%d", group, time.Now().Unix()))
	return resourceBigipCmConfigSyncRead(ctx, d, meta)
}

func resourceBigipCmConfigSyncRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	_, status, detail, err := haReadGroupSync(client, d.Get("device_group").(string))
	if err != nil {
		return diag.FromErr(fmt.Errorf("reading sync status: %w", err))
	}
	_ = d.Set("sync_status", status)
	_ = d.Set("sync_summary", detail)
	return nil
}

func resourceBigipCmConfigSyncDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}
