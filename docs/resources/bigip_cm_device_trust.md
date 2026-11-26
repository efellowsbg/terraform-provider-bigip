---
layout: "bigip"
page_title: "BIG-IP: bigip_cm_device_trust"
subcategory: "Device Management"
description: |-
  Adds a peer BIG-IP to the local device trust domain.
---

# bigip_cm_device_trust

`bigip_cm_device_trust` adds a peer BIG-IP to the device trust domain of the BIG-IP the provider is connected to (`tmsh modify cm trust-domain Root ca-devices add { <peer> }`). It is the first step of building a sync-failover cluster: run it on one unit only, and the trust then propagates to the peer.

The provider waits until the peer appears in the trust domain. Destroying the resource removes the peer from the trust domain.

## Example Usage

```hcl
resource "bigip_cm_device_trust" "peer" {
  peer_name    = "bigip2.example.net"
  peer_address = "192.0.2.12"
  username     = "admin"
  password     = var.peer_password
}
```

## Argument Reference

* `peer_name` - (Required, ForceNew) Device name of the peer, as shown by `tmsh list cm device` on the peer (normally its hostname).
* `peer_address` - (Required, ForceNew) Management IP address or FQDN used to reach the peer.
* `username` - (Required) Administrative user on the peer. Used only while establishing trust.
* `password` - (Required, Sensitive) Password of `username`. Used only while establishing trust; it is still stored in the Terraform state.
* `ca_device` - (Optional, ForceNew) Add the peer as a certificate-signing authority (peer device). Default `true`. Set to `false` to add it as a subordinate (non-authority) device.

## Timeouts

* `create` - (Default `5m`) How long to wait for the peer to join the trust domain.
* `delete` - (Default `5m`) How long to wait for the peer to leave the trust domain.

## Import

Import using the peer device name:

```
terraform import bigip_cm_device_trust.peer bigip2.example.net
```

`username` and `password` are not readable from BIG-IP; after import, set them in configuration (the next apply only stores them).
