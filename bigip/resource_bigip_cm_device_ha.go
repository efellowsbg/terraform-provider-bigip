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

	bigip "github.com/efellowsbg/go-bigip"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const (
	haDefaultConfigsyncIP = "none"
	haDefaultMirrorIP     = "any6"
	haDefaultUnicastPort  = 1026
	haManagementIPKeyword = "management-ip"
)

// resourceBigipCmDeviceHa manages the HA connectivity settings of an EXISTING
// cm device object (normally the local device created at install time):
// config-sync address, mirroring addresses and network-failover unicast
// addresses (tmsh: modify cm device <name> configsync-ip/mirror-ip/
// mirror-secondary-ip/unicast-address).
//
// Unlike bigip_cm_device it never creates or deletes the device object.
// Destroying the resource resets the managed settings to the BIG-IP defaults.
func resourceBigipCmDeviceHa() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBigipCmDeviceHaCreate,
		ReadContext:   resourceBigipCmDeviceHaRead,
		UpdateContext: resourceBigipCmDeviceHaUpdate,
		DeleteContext: resourceBigipCmDeviceHaDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the existing cm device (as shown by `tmsh list cm device`), e.g. bigip1.example.net",
			},
			"configsync_ip": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Local self IP used for config sync. `none` disables it",
			},
			"mirror_ip": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Primary local self IP used for connection/state mirroring. `any6` disables it",
			},
			"mirror_secondary_ip": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Secondary local self IP used for connection/state mirroring. `any6` disables it",
			},
			"unicast_address": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Description: "Network-failover unicast addresses (typically the HA self IP and `management-ip`)",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ip": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Local self IP, or `management-ip` for the management address",
						},
						"port": {
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      haDefaultUnicastPort,
							ValidateFunc: validation.IsPortNumber,
							Description:  "UDP port for failover heartbeats (default 1026)",
						},
					},
				},
			},
		},
	}
}

type haDevice struct {
	Name              string                   `json:"name"`
	ConfigsyncIP      string                   `json:"configsyncIp"`
	MirrorIP          string                   `json:"mirrorIp"`
	MirrorSecondaryIP string                   `json:"mirrorSecondaryIp"`
	UnicastAddress    []map[string]interface{} `json:"unicastAddress"`
}

func haDeviceURI(name string) string {
	return "cm/device/" + haURIName("Common", name)
}

func haExpandUnicast(raw []interface{}) interface{} {
	if len(raw) == 0 {
		return haDefaultConfigsyncIP // "none" clears the list
	}
	out := make([]map[string]interface{}, 0, len(raw))
	for _, r := range raw {
		m := r.(map[string]interface{})
		ip := m["ip"].(string)
		port := m["port"].(int)
		addr := map[string]interface{}{"ip": ip, "port": port}
		if ip != haManagementIPKeyword {
			addr["effectiveIp"] = ip
			addr["effectivePort"] = port
		}
		out = append(out, addr)
	}
	return out
}

func haFlattenUnicast(in []map[string]interface{}) []interface{} {
	out := make([]interface{}, 0, len(in))
	for _, a := range in {
		ip, _ := a["ip"].(string)
		port := haToInt(a["port"])
		if port == 0 {
			port = haDefaultUnicastPort
		}
		out = append(out, map[string]interface{}{"ip": ip, "port": port})
	}
	return out
}

// haDeviceHaPatch builds the PATCH body. With all=true every configured
// attribute is sent (create); otherwise only attributes that changed.
func haDeviceHaPatch(d *schema.ResourceData, all bool) map[string]interface{} {
	body := map[string]interface{}{}
	for attr, field := range map[string]string{
		"configsync_ip":       "configsyncIp",
		"mirror_ip":           "mirrorIp",
		"mirror_secondary_ip": "mirrorSecondaryIp",
	} {
		v, ok := d.GetOk(attr)
		if ok && (all || d.HasChange(attr)) {
			body[field] = v.(string)
		}
	}
	if v, ok := d.GetOk("unicast_address"); (all && ok) || (!all && d.HasChange("unicast_address")) {
		var raw []interface{}
		if v != nil {
			raw = v.([]interface{})
		}
		body["unicastAddress"] = haExpandUnicast(raw)
	}
	return body
}

func resourceBigipCmDeviceHaCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	name := d.Get("name").(string)

	var existing haDevice
	if err := haGet(client, haDeviceURI(name), &existing); err != nil {
		if isHANotFound(err) {
			return diag.Errorf("cm device %q does not exist; bigip_cm_device_ha only manages existing devices (check `tmsh list cm device`)", name)
		}
		return diag.FromErr(err)
	}

	body := haDeviceHaPatch(d, true)
	log.Printf("[INFO] Setting HA connectivity on cm device %s: %v", name, body)
	if len(body) > 0 {
		if _, err := haRequest(client, "PATCH", haDeviceURI(name), body); err != nil {
			return diag.FromErr(fmt.Errorf("configuring HA settings of cm device %s: %w", name, err))
		}
	}
	d.SetId(name)
	return resourceBigipCmDeviceHaRead(ctx, d, meta)
}

func resourceBigipCmDeviceHaRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	name := d.Id()

	var dev haDevice
	if err := haGet(client, haDeviceURI(name), &dev); err != nil {
		if isHANotFound(err) {
			log.Printf("[WARN] cm device %s not found, removing from state", name)
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	_ = d.Set("name", name)
	_ = d.Set("configsync_ip", dev.ConfigsyncIP)
	_ = d.Set("mirror_ip", dev.MirrorIP)
	_ = d.Set("mirror_secondary_ip", dev.MirrorSecondaryIP)
	if err := d.Set("unicast_address", haFlattenUnicast(dev.UnicastAddress)); err != nil {
		return diag.FromErr(fmt.Errorf("saving unicast_address of cm device %s to state: %w", name, err))
	}
	return nil
}

func resourceBigipCmDeviceHaUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	name := d.Id()
	body := haDeviceHaPatch(d, false)
	if len(body) > 0 {
		log.Printf("[INFO] Updating HA connectivity on cm device %s: %v", name, body)
		if _, err := haRequest(client, "PATCH", haDeviceURI(name), body); err != nil {
			return diag.FromErr(fmt.Errorf("updating HA settings of cm device %s: %w", name, err))
		}
	}
	return resourceBigipCmDeviceHaRead(ctx, d, meta)
}

func resourceBigipCmDeviceHaDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	name := d.Id()
	body := map[string]interface{}{
		"configsyncIp":      haDefaultConfigsyncIP,
		"mirrorIp":          haDefaultMirrorIP,
		"mirrorSecondaryIp": haDefaultMirrorIP,
		"unicastAddress":    haDefaultConfigsyncIP,
	}
	log.Printf("[INFO] Resetting HA connectivity on cm device %s", name)
	if _, err := haRequest(client, "PATCH", haDeviceURI(name), body); err != nil && !isHANotFound(err) {
		return diag.FromErr(fmt.Errorf("resetting HA settings of cm device %s: %w", name, err))
	}
	d.SetId("")
	return nil
}
