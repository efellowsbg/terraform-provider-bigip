---
layout: "bigip"
page_title: "BIG-IP: bigip_cm_config_sync"
subcategory: "Device Management"
description: |-
  Runs a config sync for a device group.
---

# bigip_cm_config_sync

`bigip_cm_config_sync` is an action resource. Creating it runs `tmsh run cm config-sync <direction> <device_group>` on the BIG-IP the provider is connected to and, by default, waits until **that device group** is in sync: every member holds the same commit. Other device groups, such as BIG-IP's built-in `datasync-*` groups, do not affect the wait. If the group is still not in sync after 35 seconds the command is sent again, because BIG-IP silently skips a sync requested less than 30 seconds after the previous one (for example right after the device group was created). A change to any argument (including `triggers`) runs the sync again. Destroying the resource does nothing on BIG-IP.

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
* `wait_for_sync` - (Optional, ForceNew) Wait until the device group is in sync. Default `true`. A `Sync Failure` status for the group fails the apply immediately.
* `triggers` - (Optional, ForceNew) Map of arbitrary values; any change re-runs the sync.

## Attribute Reference

* `sync_status` - Sync status of the device group: `In Sync`, or the status BIG-IP lists for the group (for example `Changes Pending` or `Awaiting Initial Sync`). It is refreshed on every plan, so a later drift shows up as a changed attribute but does not re-run the sync on its own.
* `sync_summary` - Detail text for the group's sync status.

## Timeouts

* `create` - (Default `10m`) How long to wait for the device group to be in sync.
