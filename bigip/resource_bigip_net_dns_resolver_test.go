/*
Copyright 2024 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/
package bigip

import (
	"fmt"
	"testing"

	bigip "github.com/efellowsbg/go-bigip"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

var TEST_DNS_RESOLVER_NAME = fmt.Sprintf("/%s/test-dns-resolver", TestPartition)

var TEST_DNS_RESOLVER_RESOURCE = `
resource "bigip_net_dns_resolver" "test-resolver" {
  name        = "` + TEST_DNS_RESOLVER_NAME + `"
  description = "test-dns-resolver"

  forward_zones {
    name        = "example.com"
    nameservers = ["1.1.1.1:53"]
  }
}
`

var TEST_DNS_RESOLVER_RESOURCE_UPDATE = `
resource "bigip_net_dns_resolver" "test-resolver" {
  name        = "` + TEST_DNS_RESOLVER_NAME + `"
  description = "test-dns-resolver"

  forward_zones {
    name        = "example.com"
    nameservers = ["1.1.1.1:53", "8.8.8.8:53"]
  }
}
`

func TestAccBigipNetDnsResolver_create(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAcctPreCheck(t)
		},
		Providers:    testAccProviders,
		CheckDestroy: testCheckDnsResolversDestroyed,
		Steps: []resource.TestStep{
			{
				Config: TEST_DNS_RESOLVER_RESOURCE,
				Check: resource.ComposeTestCheckFunc(
					testCheckDnsResolverExists(TEST_DNS_RESOLVER_NAME, true),
					resource.TestCheckResourceAttr("bigip_net_dns_resolver.test-resolver", "name", TEST_DNS_RESOLVER_NAME),
					resource.TestCheckResourceAttr("bigip_net_dns_resolver.test-resolver", "forward_zones.0.name", "example.com"),
					resource.TestCheckResourceAttr("bigip_net_dns_resolver.test-resolver", "forward_zones.0.nameservers.0", "1.1.1.1:53"),
				),
			},
		},
	})
}

func TestAccBigipNetDnsResolver_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAcctPreCheck(t)
		},
		Providers:    testAccProviders,
		CheckDestroy: testCheckDnsResolversDestroyed,
		Steps: []resource.TestStep{
			{
				Config: TEST_DNS_RESOLVER_RESOURCE,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("bigip_net_dns_resolver.test-resolver", "forward_zones.0.nameservers.0", "1.1.1.1:53"),
				),
			},
			{
				Config: TEST_DNS_RESOLVER_RESOURCE_UPDATE,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("bigip_net_dns_resolver.test-resolver", "forward_zones.0.nameservers.0", "1.1.1.1:53"),
					resource.TestCheckResourceAttr("bigip_net_dns_resolver.test-resolver", "forward_zones.0.nameservers.1", "8.8.8.8:53"),
				),
			},
		},
	})
}

func TestAccBigipNetDnsResolver_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAcctPreCheck(t)
		},
		Providers:    testAccProviders,
		CheckDestroy: testCheckDnsResolversDestroyed,
		Steps: []resource.TestStep{
			{
				Config: TEST_DNS_RESOLVER_RESOURCE,
				Check: resource.ComposeTestCheckFunc(
					testCheckDnsResolverExists(TEST_DNS_RESOLVER_NAME, true),
				),
				ResourceName:      TEST_DNS_RESOLVER_NAME,
				ImportState:       false,
				ImportStateVerify: true,
			},
		},
	})
}

func testCheckDnsResolverExists(name string, exists bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccProvider.Meta().(*bigip.BigIP)

		resolver, err := client.GetDNSResolver(name)
		if err != nil {
			return err
		}
		if exists && resolver == nil {
			return fmt.Errorf("dns resolver %s was not created.", name)
		}
		if !exists && resolver != nil {
			return fmt.Errorf("dns resolver %s still exists.", name)
		}
		return nil
	}
}

func testCheckDnsResolversDestroyed(s *terraform.State) error {
	client := testAccProvider.Meta().(*bigip.BigIP)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "bigip_net_dns_resolver" {
			continue
		}

		name := rs.Primary.ID
		resolver, err := client.GetDNSResolver(name)
		if err != nil {
			if isDNSResolverNotFound(err) {
				continue
			}
			return err
		}
		if resolver != nil {
			return fmt.Errorf("dns resolver %s not destroyed.", name)
		}
	}
	return nil
}
