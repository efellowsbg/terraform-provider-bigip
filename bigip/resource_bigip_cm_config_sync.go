/*
Copyright 2026 eFellows Ltd.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/

package bigip

import (
	"context"
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

// resourceBigipCmConfigSync is an action resource: creating it (or changing
// triggers) runs `tmsh run cm config-sync <direction> <device_group>` and
// optionally waits until the cluster reports "In Sync". Destroying it does
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
				Description: "Wait until the cluster sync status is In Sync",
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
				Description: "Cluster sync status reported by BIG-IP (e.g. In Sync, Changes Pending)",
			},
			"sync_summary": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Sync status summary text reported by BIG-IP",
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

func haReadSyncStatus(client *bigip.BigIP) (status, summary string, err error) {
	resp, err := haRequest(client, "GET", uriCmSyncStatus, nil)
	if err != nil {
		return "", "", err
	}
	fields, err := haStatsDescriptions(resp)
	if err != nil {
		return "", "", fmt.Errorf("decoding sync status: %w", err)
	}
	return fields["status"], fields["summary"], nil
}

func resourceBigipCmConfigSyncCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	group := d.Get("device_group").(string)

	cmd, err := haSyncCommand(group, d.Get("direction").(string), d.Get("force_full_load_push").(bool))
	if err != nil {
		return diag.FromErr(err)
	}
	log.Printf("[INFO] Running: tmsh run cm %s", cmd)
	if _, err := haRequest(client, "POST", uriCm, map[string]interface{}{
		"command":     "run",
		"utilCmdArgs": cmd,
	}); err != nil {
		return diag.FromErr(fmt.Errorf("running %q: %w", cmd, err))
	}

	if d.Get("wait_for_sync").(bool) {
		err := haWaitFor(ctx, d.Timeout(schema.TimeoutCreate), fmt.Sprintf("device group %s to be In Sync", group), func() (bool, error) {
			status, summary, err := haReadSyncStatus(client)
			if err != nil {
				return false, err
			}
			for _, failed := range haSyncFailureStatuses {
				if strings.EqualFold(status, failed) {
					return false, haStop(fmt.Errorf("sync status is %q: %s", status, summary))
				}
			}
			return status == haSyncStatusInSync || status == haSyncStatusStandby, nil
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
	status, summary, err := haReadSyncStatus(client)
	if err != nil {
		return diag.FromErr(fmt.Errorf("reading sync status: %w", err))
	}
	_ = d.Set("sync_status", status)
	_ = d.Set("sync_summary", summary)
	return nil
}

func resourceBigipCmConfigSyncDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}
