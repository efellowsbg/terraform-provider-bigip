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
	uriSysFailover      = "sys/failover"
	uriCmFailoverStatus = "cm/failover-status"
)

// resourceBigipSysFailover is an action resource that runs
// `tmsh run sys failover standby|offline|online [traffic-group <tg>]` on the
// device the provider is connected to. Destroying it does nothing on the BIG-IP.
func resourceBigipSysFailover() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBigipSysFailoverCreate,
		ReadContext:   resourceBigipSysFailoverRead,
		DeleteContext: resourceBigipSysFailoverDelete,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(5 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"action": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"standby", "offline", "online"}, false),
				Description:  "standby: hand traffic group(s) to the peer; offline: take the device out of the cluster; online: bring it back",
			},
			"traffic_group": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "With action = standby, fail over only this traffic group (default: all traffic groups)",
			},
			"wait_for_state": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				ForceNew:    true,
				Description: "Wait until the device reports the expected failover state (STANDBY, FORCED OFFLINE, or not FORCED OFFLINE). Ignored when traffic_group is set",
			},
			"triggers": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Arbitrary values; any change re-runs the action",
			},
			"failover_state": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Failover state of the device (ACTIVE, STANDBY, FORCED OFFLINE, ...)",
			},
		},
	}
}

func haReadFailoverState(client *bigip.BigIP) (string, error) {
	resp, err := haRequest(client, "GET", uriCmFailoverStatus, nil)
	if err != nil {
		return "", err
	}
	fields, err := haStatsDescriptions(resp)
	if err != nil {
		return "", fmt.Errorf("decoding failover status: %w", err)
	}
	return fields["status"], nil
}

func resourceBigipSysFailoverCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	action := d.Get("action").(string)
	tg := d.Get("traffic_group").(string)

	if tg != "" && action != "standby" {
		return diag.Errorf("traffic_group can only be used with action = \"standby\"")
	}
	body := map[string]interface{}{"command": "run", action: true}
	if tg != "" {
		body["trafficGroup"] = tg
	}
	log.Printf("[INFO] Running sys failover %s %s", action, tg)
	if _, err := haRequest(client, "POST", uriSysFailover, body); err != nil {
		return diag.FromErr(fmt.Errorf("running sys failover %s: %w", action, err))
	}

	if d.Get("wait_for_state").(bool) && tg == "" {
		err := haWaitFor(ctx, d.Timeout(schema.TimeoutCreate), "failover state after "+action, func() (bool, error) {
			state, err := haReadFailoverState(client)
			if err != nil {
				return false, err
			}
			state = strings.ToUpper(state)
			switch action {
			case "standby":
				return state == "STANDBY", nil
			case "offline":
				return state == "FORCED OFFLINE", nil
			default: // online
				return state != "" && state != "FORCED OFFLINE", nil
			}
		})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(fmt.Sprintf("%s-%d", action, time.Now().Unix()))
	return resourceBigipSysFailoverRead(ctx, d, meta)
}

func resourceBigipSysFailoverRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	state, err := haReadFailoverState(client)
	if err != nil {
		return diag.FromErr(fmt.Errorf("reading failover status: %w", err))
	}
	_ = d.Set("failover_state", state)
	return nil
}

func resourceBigipSysFailoverDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}
