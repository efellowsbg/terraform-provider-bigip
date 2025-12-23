/*
Copyright 2024 F5 Networks Inc.
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
)

func resourceBigipNetDnsResolver() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBigipNetDnsResolverCreate,
		ReadContext:   resourceBigipNetDnsResolverRead,
		UpdateContext: resourceBigipNetDnsResolverUpdate,
		DeleteContext: resourceBigipNetDnsResolverDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateF5NameWithDirectory,
				Description:  "Name of the DNS resolver (e.g. /Common/resolver1)",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "User defined description",
			},
			"answer_default_zones": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Specifies whether the resolver answers default zones.",
			},
			"cache_size": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "Specifies the cache size for the resolver.",
			},
			"randomize_query_name_case": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Specifies whether the resolver randomizes query name case.",
			},
			"route_domain": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Specifies the route domain for the resolver.",
			},
			"type": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Specifies the resolver type.",
			},
			"use_ipv4": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Specifies whether the resolver uses IPv4.",
			},
			"use_ipv6": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Specifies whether the resolver uses IPv6.",
			},
			"use_tcp": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Specifies whether the resolver uses TCP.",
			},
			"use_udp": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Specifies whether the resolver uses UDP.",
			},
			"forward_zones": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Description: "Forward zones with their nameservers.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Forward zone name.",
						},
						"nameservers": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "List of nameservers for the zone (IP[:port]).",
						},
					},
				},
			},
		},
	}
}

func resourceBigipNetDnsResolverCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	name := d.Get("name").(string)

	log.Printf("[INFO] Creating DNS Resolver %s", name)
	resolver := &bigip.DNSResolver{
		Name: name,
	}
	config := getNetDNSResolverConfig(d, resolver)

	if err := client.CreateDNSResolver(config); err != nil {
		return diag.FromErr(fmt.Errorf("error creating DNS resolver %s: %v", name, err))
	}

	d.SetId(name)
	return resourceBigipNetDnsResolverRead(ctx, d, meta)
}

func resourceBigipNetDnsResolverRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	name := d.Id()

	log.Printf("[INFO] Reading DNS Resolver %s", name)
	resolver, err := client.GetDNSResolver(name)
	if err != nil {
		if isDNSResolverNotFound(err) {
			log.Printf("[WARN] DNS Resolver (%s) not found, removing from state", name)
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("error retrieving DNS resolver %s: %v", name, err))
	}
	if resolver == nil {
		log.Printf("[WARN] DNS Resolver (%s) not found, removing from state", name)
		d.SetId("")
		return nil
	}

	if resolver.FullPath != "" {
		_ = d.Set("name", resolver.FullPath)
	} else {
		_ = d.Set("name", name)
	}
	_ = d.Set("description", resolver.Description)
	_ = d.Set("answer_default_zones", resolver.AnswerDefaultZones)
	_ = d.Set("cache_size", resolver.CacheSize)
	_ = d.Set("randomize_query_name_case", resolver.RandomizeQueryNameCase)
	_ = d.Set("route_domain", resolver.RouteDomain)
	_ = d.Set("type", resolver.Type)
	_ = d.Set("use_ipv4", resolver.UseIpv4)
	_ = d.Set("use_ipv6", resolver.UseIpv6)
	_ = d.Set("use_tcp", resolver.UseTcp)
	_ = d.Set("use_udp", resolver.UseUdp)
	_ = d.Set("forward_zones", flattenDNSResolverForwardZones(resolver.ForwardZones))

	return nil
}

func resourceBigipNetDnsResolverUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	name := d.Id()

	log.Printf("[INFO] Updating DNS Resolver %s", name)
	resolver := &bigip.DNSResolver{
		Name: name,
	}
	config := getNetDNSResolverConfig(d, resolver)

	if err := client.ModifyDNSResolver(name, config); err != nil {
		return diag.FromErr(fmt.Errorf("error modifying DNS resolver %s: %v", name, err))
	}
	return resourceBigipNetDnsResolverRead(ctx, d, meta)
}

func resourceBigipNetDnsResolverDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*bigip.BigIP)
	name := d.Id()

	log.Printf("[INFO] Deleting DNS Resolver %s", name)
	if err := client.DeleteDNSResolver(name); err != nil {
		if isDNSResolverNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("error deleting DNS resolver %s: %v", name, err))
	}
	d.SetId("")
	return nil
}

func getNetDNSResolverConfig(d *schema.ResourceData, config *bigip.DNSResolver) *bigip.DNSResolver {
	config.Description = d.Get("description").(string)
	config.AnswerDefaultZones = d.Get("answer_default_zones").(string)
	config.CacheSize = d.Get("cache_size").(int)
	config.RandomizeQueryNameCase = d.Get("randomize_query_name_case").(string)
	config.RouteDomain = d.Get("route_domain").(string)
	config.Type = d.Get("type").(string)
	config.UseIpv4 = d.Get("use_ipv4").(string)
	config.UseIpv6 = d.Get("use_ipv6").(string)
	config.UseTcp = d.Get("use_tcp").(string)
	config.UseUdp = d.Get("use_udp").(string)
	if v, ok := d.GetOk("forward_zones"); ok {
		config.ForwardZones = expandDNSResolverForwardZones(v.([]interface{}))
	}
	return config
}

func expandDNSResolverForwardZones(raw []interface{}) []bigip.DNSResolverForwardZone {
	if len(raw) == 0 {
		return nil
	}
	zones := make([]bigip.DNSResolverForwardZone, 0, len(raw))
	for _, item := range raw {
		data, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		zone := bigip.DNSResolverForwardZone{}
		if v, ok := data["name"]; ok {
			zone.Name = v.(string)
		}
		if v, ok := data["nameservers"]; ok && v != nil {
			if list, ok := v.([]interface{}); ok {
				names := listToStringSlice(list)
				nameServers := make([]bigip.DNSResolverNameserver, 0, len(names))
				for _, ns := range names {
					nameServers = append(nameServers, bigip.DNSResolverNameserver{Name: ns})
				}
				zone.NameServers = nameServers
			}
		}
		zones = append(zones, zone)
	}
	return zones
}

func flattenDNSResolverForwardZones(zones []bigip.DNSResolverForwardZone) []interface{} {
	if len(zones) == 0 {
		return []interface{}{}
	}
	result := make([]interface{}, 0, len(zones))
	for _, zone := range zones {
		nameServers := make([]string, 0, len(zone.NameServers))
		for _, ns := range zone.NameServers {
			if ns.Name != "" {
				nameServers = append(nameServers, ns.Name)
			}
		}
		result = append(result, map[string]interface{}{
			"name":        zone.Name,
			"nameservers": nameServers,
		})
	}
	return result
}

func isDNSResolverNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "01020036") ||
		strings.Contains(strings.ToLower(msg), "not found") ||
		strings.Contains(msg, "404")
}
