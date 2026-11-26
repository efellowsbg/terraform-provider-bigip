---
layout: "bigip"
page_title: "BIG-IP: bigip_sys_failover"
subcategory: "Device Management"
description: |-
  Forces a BIG-IP to standby, offline or online.
---

# bigip_sys_failover

`bigip_sys_failover` is an action resource. Creating it runs `tmsh run sys failover standby|offline|online [traffic-group <tg>]` on the BIG-IP the provider is connected to and, by default, waits until the device reports the expected failover state. A change to any argument (including `triggers`) runs the action again. Destroying the resource does nothing on BIG-IP; in particular, destroying an `offline` action does **not** bring the device back online. Apply an `online` action for that.

This resource changes which unit carries traffic. Use it in maintenance runs only, not in configuration that is applied routinely.

## Example Usage

```hcl
# Move all traffic groups away from bigip1 before maintenance
resource "bigip_sys_failover" "bigip1_standby" {
  provider = bigip.bigip1
  action   = "standby"

  triggers = {
    change = "CHG-1234"
  }
}
```

## Argument Reference

* `action` - (Required, ForceNew) `standby`, `offline` or `online`.
* `traffic_group` - (Optional, ForceNew) With `standby`, fail over only this traffic group. By default, all traffic groups fail over.
* `wait_for_state` - (Optional, ForceNew) Wait until the action has taken effect. Without `traffic_group`: until the device reports `STANDBY`, `FORCED OFFLINE`, or a state other than `FORCED OFFLINE` (for `online`). With `traffic_group`: until another device is active for that traffic group. Default `true`.
* `triggers` - (Optional, ForceNew) Map of arbitrary values; any change re-runs the action.

## Attribute Reference

* `failover_state` - Failover state of the device (for example `ACTIVE`, `STANDBY` or `FORCED OFFLINE`).

## Timeouts

* `create` - (Default `5m`) How long to wait for the expected state.
