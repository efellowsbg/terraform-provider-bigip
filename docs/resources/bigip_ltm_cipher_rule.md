---
layout: "bigip"
page_title: "BIG-IP: bigip_ltm_cipher_rule"
subcategory: "Local Traffic Manager(LTM)"
description: |-
  Provides details about bigip_ltm_cipher_rule resource
---

# bigip\_ltm\_cipher\_rule

`bigip_ltm_cipher_rule` Manages F5 BIG-IP LTM cipher rule via iControl REST API.

## Example Usage

```hcl
resource "bigip_ltm_cipher_rule" "test_cipher_rule" {
  name                 = "test_cipher_rule"
  partition            = "Uncommon"
  cipher_suites        = "TLS13-AES128-GCM-SHA256:TLS13-AES256-GCM-SHA384"
  dh_groups            = "P256:P384:FFDHE2048:FFDHE3072:FFDHE4096"
  signature_algorithms = "DEFAULT"
}
```

## Argument Reference

* `name` - (Required,type `string`) Name of the Cipher Rule.

* `partition` - (Optional,type `string`) The Partition in which the Cipher Rule will be created.

* `cipher_suites` - (Required,type `string`) This is a colon (:) separated string of cipher suites. example, `TLS13-AES128-GCM-SHA256:TLS13-AES256-GCM-SHA384`. The default value for this attribute is `DEFAULT`.

* `dh_groups` - (Optional,type `string`) Specifies the DH Groups algorithms, separated by colons (:).

* `signature_algorithms` - (Optional,type `string`) Specifies the Signature Algorithms, separated by colons (:).

## Read-Only

* `full_path` - (String) The full path of the cipher rule, e.g. /Common/test_cipher_rule.

## Importing
An existing cipher rule can be imported into this resource by supplying the cipher rule's `full path` as `id`.
An example is below:
```sh
$ terraform import bigip_ltm_cipher_rule.test_cipher_rule /Common/test_cipher_rule

```