---
layout: "bigip"
page_title: "BIG-IP: bigip_cm_config_sync"
subcategory: "Device Management"
description: |-
  Runs a config sync for a device group.
---

# bigip_cm_config_sync

`bigip_cm_config_sync` is an action resource. Creating it runs `tmsh run cm config-sync <direction> <device_group>` on the BIG-IP the provider is connected to and, by default, waits until the cluster reports **In Sync**. A change to any argument (including `triggers`) runs the sync again. Destroying the resource does nothing on BIG-IP.

Use `depends_on` or `triggers` so the sync runs after the configuration it should carry.

## Example Usage

```hcl
resource "bigip_cm_config_sync" "after_change" {
  device_group = bigip_cm_devicegroup.failover.name
  direction    = "to-group"

  triggers = {
    traffic_group = bigip_cm_traffic_group.tg2.id
    selfip        = bigip_net_selfip.floating.id
  }
}
```

## Argument Reference

* `device_group` - (Required, ForceNew) Device group to synchronise.
* `direction` - (Optional, ForceNew) `to-group` (push this device's configuration) or `from-group` (pull the group's configuration). Default `to-group`.
* `force_full_load_push` - (Optional, ForceNew) Push the full configuration instead of an incremental sync. Requires `to-group`. Default `false`.
* `wait_for_sync` - (Optional, ForceNew) Wait until the sync status is `In Sync`. Default `true`. A `Sync Failure` status fails the apply immediately.
* `triggers` - (Optional, ForceNew) Map of arbitrary values; any change re-runs the sync.

## Attribute Reference

* `sync_status` - Sync status reported by BIG-IP (for example `In Sync` or `Changes Pending`). It is refreshed on every plan, so a later drift shows up as a changed attribute but does not re-run the sync on its own.
* `sync_summary` - Sync status summary text.

## Timeouts

* `create` - (Default `10m`) How long to wait for `In Sync`.
