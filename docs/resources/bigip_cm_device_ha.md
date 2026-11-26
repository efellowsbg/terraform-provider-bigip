---
layout: "bigip"
page_title: "BIG-IP: bigip_cm_device_ha"
subcategory: "Device Management"
description: |-
  Manages the HA connectivity settings (config sync, mirroring, failover unicast) of an existing cm device.
---

# bigip_cm_device_ha

`bigip_cm_device_ha` sets the high-availability connectivity of an **existing** cm device object, normally the local device that BIG-IP creates at install time:

* config-sync address (`configsync-ip`)
* connection/state mirroring addresses (`mirror-ip`, `mirror-secondary-ip`)
* network-failover unicast addresses (`unicast-address`)

Unlike `bigip_cm_device`, it never creates or deletes the device object, and it supports in-place updates. Destroying the resource resets these settings to the BIG-IP defaults (`configsync-ip none`, `mirror-ip any6`, `mirror-secondary-ip any6`, `unicast-address none`).

Use one provider alias per BIG-IP and configure each unit's own device.

## Example Usage

```hcl
resource "bigip_cm_device_ha" "bigip1" {
  name                = "bigip1.example.net"
  configsync_ip       = "10.10.99.1"
  mirror_ip           = "10.10.99.1"
  mirror_secondary_ip = "10.10.98.1"

  unicast_address {
    ip = "10.10.99.1"
  }
  unicast_address {
    ip = "management-ip"
  }
}
```

## Argument Reference

* `name` - (Required, ForceNew) Name of the existing cm device (`tmsh list cm device`).
* `configsync_ip` - (Optional) Local self IP used for config sync. `none` disables it.
* `mirror_ip` - (Optional) Primary local self IP for connection/state mirroring. `any6` disables it.
* `mirror_secondary_ip` - (Optional) Secondary local self IP for mirroring. `any6` disables it.
* `unicast_address` - (Optional) Network-failover unicast addresses, in order. Each block supports:
  * `ip` - (Required) Local self IP, or `management-ip` for the management address.
  * `port` - (Optional) UDP port for failover heartbeats. Default `1026`.

Attributes that are not configured are left unchanged on BIG-IP and read back into state.

## Import

```
terraform import bigip_cm_device_ha.bigip1 bigip1.example.net
```
