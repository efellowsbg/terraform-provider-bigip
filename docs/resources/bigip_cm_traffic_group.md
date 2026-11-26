---
layout: "bigip"
page_title: "BIG-IP: bigip_cm_traffic_group"
subcategory: "Device Management"
description: |-
  Manages a floating traffic group.
---

# bigip_cm_traffic_group

`bigip_cm_traffic_group` manages a floating traffic group (`tmsh create/modify/delete cm traffic-group`). Floating self IPs and virtual addresses are attached to a traffic group and move together between devices on failover. Use `full_path` as `traffic_group` in `bigip_net_selfip`.

Traffic groups are synchronised by config sync, so create them on one unit of the device group and then sync (for example with `bigip_cm_config_sync`).

## Example Usage

```hcl
resource "bigip_cm_traffic_group" "tg2" {
  name                  = "traffic-group-2"
  description           = "Second active unit"
  failover_method       = "ha-order"
  ha_order              = ["bigip2.example.net", "bigip1.example.net"]
  auto_failback_enabled = true
  auto_failback_time    = 60
}

resource "bigip_net_selfip" "floating" {
  name          = "/Common/internal-floating-2"
  ip            = "10.10.10.250/24"
  vlan          = "/Common/internal"
  traffic_group = bigip_cm_traffic_group.tg2.full_path
}
```

## Argument Reference

* `name` - (Required, ForceNew) Name of the traffic group.
* `partition` - (Optional, ForceNew) Partition. Default `Common`.
* `description` - (Optional) User-defined description.
* `failover_method` - (Optional) `ha-order`, `ha-score` or `load-aware`. BIG-IP default `ha-order`.
* `ha_order` - (Optional) Ordered list of device names to fail over to (with `ha-order`). Bare names are sent as `/Common/<name>`.
* `ha_group` - (Optional) HA group used to compute the HA score (with `ha-score`).
* `ha_load_factor` - (Optional) Relative load of the traffic group, 1-1000 (with `load-aware`).
* `auto_failback_enabled` - (Optional) Fail back to the first device in `ha_order` when it becomes available again. Default `false`.
* `auto_failback_time` - (Optional) Seconds to wait before failing back, 0-300. BIG-IP default `60`.
* `mac` - (Optional) MAC masquerade address shared by the floating addresses, or `none`.

## Attribute Reference

* `full_path` - Full path of the traffic group, e.g. `/Common/traffic-group-2`.

## Import

```
terraform import bigip_cm_traffic_group.tg2 /Common/traffic-group-2
```
