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
	"time"

	bigip "github.com/efellowsbg/go-bigip"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	uriCmTrustDomainRoot  = "cm/trust-domain/~Common~Root"
	uriCmAddToTrust       = "cm/add-to-trust"
	uriCmRemoveFromTrust  = "cm/remove-from-trust"
	haTrustDomainRootName = "Root"
)

// resourceBigipCmDeviceTrust adds a peer BIG-IP to the local device trust domain
// (tmsh: modify cm trust-domain Root ca-devices add { <peer> }).
func resourceBigipCmDeviceTrust() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBigipCmDeviceTrustCreate,
		ReadContext:   resourceBigipCmDeviceTrustRead,
		UpdateContext: resourceBigipCmDeviceTrustUpdate,
		DeleteContext: resourceBigipCmDeviceTrustDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(5 * time.Minute),
			Delete: schema.DefaultTimeout(5 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"peer_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Device name (cm device name / hostname) of the peer BIG-IP to add to the trust domain",
			},
			"peer_address": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Management IP address or FQDN the local BIG-IP uses to reach the peer",
			},
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Administrative user on the peer, used only while establishing trust",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				Description: "Password of the administrative user on the peer, used only while establishing trust",
			},
			"ca_device": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				ForceNew:    true,
				Description: "Add the peer as a certificate-signing authority (peer). Set to false to add it as a subordinate (non-authority) device",
			},
		},
	}
}

type haTrustDomain struct {
	CaDevices    []string `json:"caDevices"`
	NonCaDevices []string `json:"nonCaDevices"`
}

// haTrustMembership returns whether peerName is in the trust domain and, if so,
// whether it is a CA (peer) device.
func haTrustMembership(client *bigip.BigIP, peerName string) (member bool, ca bool, err error) {
	var td haTrustDomain
	if err := haGet(client, uriCmTrustDomainRoot, &td); err != nil {
		return false, false, err
	}
	for _, dev := range td.CaDevices {
		if haShortName(dev) == haShortName(peerName) {
			return true, true, nil
		}
	}
	for _, dev := range td.NonCaDevices {
		if haShortName(dev) == haShortName(peerName) {
			return true, false, nil
		}
	}
	return false, false, nil
}

func resourceBigipCmDeviceTrustCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	peerName := d.Get("peer_name").(string)

	body := map[string]interface{}{
		"command":    "run",
		"name":       haTrustDomainRootName,
		"caDevice":   d.Get("ca_device").(bool),
		"device":     d.Get("peer_address").(string),
		"deviceName": peerName,
		"username":   d.Get("username").(string),
		"password":   d.Get("password").(string),
	}
	log.Printf("[INFO] Adding device %s (%s) to trust domain", peerName, d.Get("peer_address").(string))
	if _, err := haRequest(client, "POST", uriCmAddToTrust, body); err != nil {
		return diag.FromErr(fmt.Errorf("adding %s to trust domain: %w", peerName, err))
	}

	err := haWaitFor(ctx, d.Timeout(schema.TimeoutCreate), fmt.Sprintf("%s to join the trust domain", peerName), func() (bool, error) {
		member, _, err := haTrustMembership(client, peerName)
		return member, err
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(peerName)
	return resourceBigipCmDeviceTrustRead(ctx, d, meta)
}

func resourceBigipCmDeviceTrustRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	peerName := d.Id()

	member, ca, err := haTrustMembership(client, peerName)
	if err != nil {
		return diag.FromErr(fmt.Errorf("reading trust domain: %w", err))
	}
	if !member {
		log.Printf("[WARN] Device %s is no longer in the trust domain, removing from state", peerName)
		d.SetId("")
		return nil
	}
	_ = d.Set("peer_name", peerName)
	_ = d.Set("ca_device", ca)

	// peer_address is only filled on import: the configured value may be an FQDN,
	// and overwriting it with the management IP would force a replacement.
	if d.Get("peer_address").(string) == "" {
		var dev struct {
			ManagementIP string `json:"managementIp"`
		}
		if err := haGet(client, "cm/device/"+haURIName("Common", peerName), &dev); err == nil {
			_ = d.Set("peer_address", dev.ManagementIP)
		}
	}
	return nil
}

// Only username/password can change in place; they are used only at creation,
// so an update just stores the new values.
func resourceBigipCmDeviceTrustUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceBigipCmDeviceTrustRead(ctx, d, meta)
}

func resourceBigipCmDeviceTrustDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	peerName := d.Id()

	body := map[string]interface{}{
		"command":    "run",
		"name":       haTrustDomainRootName,
		"deviceName": peerName,
	}
	log.Printf("[INFO] Removing device %s from trust domain", peerName)
	if _, err := haRequest(client, "POST", uriCmRemoveFromTrust, body); err != nil && !isHANotFound(err) {
		return diag.FromErr(fmt.Errorf("removing %s from trust domain: %w", peerName, err))
	}

	err := haWaitFor(ctx, d.Timeout(schema.TimeoutDelete), fmt.Sprintf("%s to leave the trust domain", peerName), func() (bool, error) {
		member, _, err := haTrustMembership(client, peerName)
		return !member, err
	})
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}
