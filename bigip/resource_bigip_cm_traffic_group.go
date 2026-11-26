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

	bigip "github.com/efellowsbg/go-bigip"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const uriCmTrafficGroup = "cm/traffic-group"

// resourceBigipCmTrafficGroup manages floating traffic groups
// (tmsh: create/modify/delete cm traffic-group).
func resourceBigipCmTrafficGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBigipCmTrafficGroupCreate,
		ReadContext:   resourceBigipCmTrafficGroupRead,
		UpdateContext: resourceBigipCmTrafficGroupUpdate,
		DeleteContext: resourceBigipCmTrafficGroupDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the traffic group, e.g. traffic-group-2",
			},
			"partition": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "Common",
				ForceNew:    true,
				Description: "Partition of the traffic group",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "User-defined description",
			},
			"failover_method": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.StringInSlice([]string{"ha-order", "ha-score", "load-aware"}, false),
				Description:  "How the next active device is chosen: ha-order, ha-score (uses ha_group) or load-aware",
			},
			"ha_order": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
					// "bigip1" and "/Common/bigip1" name the same device (e.g. after import).
					DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
						return old != "" && new != "" && haShortName(old) == haShortName(new)
					},
				},
				Description: "Ordered list of device names to fail over to (used with failover_method = ha-order)",
			},
			"ha_group": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "HA group used to compute the HA score (used with failover_method = ha-score)",
			},
			"ha_load_factor": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(1, 1000),
				Description:  "Relative load this traffic group puts on a device (used with failover_method = load-aware)",
			},
			"auto_failback_enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Fail back to the first device in ha_order when it becomes available again",
			},
			"auto_failback_time": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(0, 300),
				Description:  "Seconds to wait before failing back (default 60)",
			},
			"mac": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "MAC masquerade address shared by the floating addresses, or `none`",
			},
			"full_path": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Full path of the traffic group, e.g. /Common/traffic-group-2; use it as traffic_group in bigip_net_selfip",
			},
		},
	}
}

type haTrafficGroup struct {
	Name                string      `json:"name"`
	Partition           string      `json:"partition"`
	FullPath            string      `json:"fullPath"`
	Description         string      `json:"description"`
	FailoverMethod      string      `json:"failoverMethod"`
	HaOrder             []string    `json:"haOrder"`
	HaGroup             string      `json:"haGroup"`
	HaLoadFactor        interface{} `json:"haLoadFactor"`
	AutoFailbackEnabled interface{} `json:"autoFailbackEnabled"`
	AutoFailbackTime    interface{} `json:"autoFailbackTime"`
	Mac                 string      `json:"mac"`
}

func haTrafficGroupBody(d *schema.ResourceData, create bool) map[string]interface{} {
	body := map[string]interface{}{}
	if create {
		body["name"] = d.Get("name").(string)
		body["partition"] = d.Get("partition").(string)
	}
	set := func(attr, field string, conv func(interface{}) interface{}) {
		v, ok := d.GetOk(attr)
		if (create && ok) || (!create && d.HasChange(attr)) {
			body[field] = conv(v)
		}
	}
	str := func(v interface{}) interface{} { return v }
	set("description", "description", func(v interface{}) interface{} {
		if v == nil || v.(string) == "" {
			return "none"
		}
		return v
	})
	set("failover_method", "failoverMethod", str)
	set("ha_group", "haGroup", str)
	set("ha_load_factor", "haLoadFactor", str)
	set("auto_failback_time", "autoFailbackTime", str)
	set("mac", "mac", str)
	if create || d.HasChange("auto_failback_enabled") {
		body["autoFailbackEnabled"] = fmt.Sprintf("%t", d.Get("auto_failback_enabled").(bool))
	}
	set("ha_order", "haOrder", func(v interface{}) interface{} {
		var devices []string
		if v != nil {
			for _, dev := range v.([]interface{}) {
				devices = append(devices, haFullPath("Common", dev.(string)))
			}
		}
		if len(devices) == 0 {
			return "none"
		}
		return devices
	})
	return body
}

func haTrafficGroupURI(d *schema.ResourceData) string {
	return uriCmTrafficGroup + "/" + strings.ReplaceAll(d.Id(), "/", "~")
}

func resourceBigipCmTrafficGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	fullPath := haFullPath(d.Get("partition").(string), d.Get("name").(string))

	body := haTrafficGroupBody(d, true)
	log.Printf("[INFO] Creating traffic group %s: %v", fullPath, body)
	if _, err := haRequest(client, "POST", uriCmTrafficGroup, body); err != nil {
		return diag.FromErr(fmt.Errorf("creating traffic group %s: %w", fullPath, err))
	}
	d.SetId(fullPath)
	return resourceBigipCmTrafficGroupRead(ctx, d, meta)
}

func resourceBigipCmTrafficGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)

	var tg haTrafficGroup
	if err := haGet(client, haTrafficGroupURI(d), &tg); err != nil {
		if isHANotFound(err) {
			log.Printf("[WARN] Traffic group %s not found, removing from state", d.Id())
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	_ = d.Set("name", tg.Name)
	if tg.Partition != "" {
		_ = d.Set("partition", tg.Partition)
	}
	_ = d.Set("full_path", d.Id())
	if tg.Description == "none" {
		tg.Description = ""
	}
	_ = d.Set("description", tg.Description)
	_ = d.Set("failover_method", tg.FailoverMethod)
	_ = d.Set("ha_group", tg.HaGroup)
	_ = d.Set("ha_load_factor", haToInt(tg.HaLoadFactor))
	_ = d.Set("auto_failback_enabled", haToBool(tg.AutoFailbackEnabled))
	_ = d.Set("auto_failback_time", haToInt(tg.AutoFailbackTime))
	_ = d.Set("mac", tg.Mac)

	configured := map[string]bool{}
	for _, v := range d.Get("ha_order").([]interface{}) {
		configured[v.(string)] = true
	}
	order := make([]string, 0, len(tg.HaOrder))
	for _, dev := range tg.HaOrder {
		order = append(order, haMatchNameForm(dev, configured))
	}
	if err := d.Set("ha_order", order); err != nil {
		return diag.FromErr(fmt.Errorf("saving ha_order of traffic group %s to state: %w", d.Id(), err))
	}
	return nil
}

func resourceBigipCmTrafficGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	body := haTrafficGroupBody(d, false)
	if len(body) > 0 {
		log.Printf("[INFO] Updating traffic group %s: %v", d.Id(), body)
		if _, err := haRequest(client, "PATCH", haTrafficGroupURI(d), body); err != nil {
			return diag.FromErr(fmt.Errorf("updating traffic group %s: %w", d.Id(), err))
		}
	}
	return resourceBigipCmTrafficGroupRead(ctx, d, meta)
}

func resourceBigipCmTrafficGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	log.Printf("[INFO] Deleting traffic group %s", d.Id())
	if _, err := haRequest(client, "DELETE", haTrafficGroupURI(d), nil); err != nil && !isHANotFound(err) {
		return diag.FromErr(fmt.Errorf("deleting traffic group %s: %w", d.Id(), err))
	}
	d.SetId("")
	return nil
}
