---
layout: "cloudstack"
page_title: "Cloudstack: cloudstack_template"
sidebar_current: "docs-cloudstack-datasource-template"
description: |-
  Get informations on a Cloudstack template.
---

# cloudstack_network

Use this datasource to get the ID of a network for use in other resources.

### Example Usage

```hcl
data "cloudstack_network" "my_network" {
  filter {
    name = "name"
    value = "private_network"
  }

  filter {
    name = "cidr"
    value = "10.0.0.0/24"
  }
}
```

### Argument Reference

* `filter` - (Required) One or more name/value pairs to filter off of. You can apply filters on any exported attributes.

## Attributes Reference

The following attributes are exported:

* `id` - The network ID.
* `account` - The account name to which the network belongs.
* `cidr` - The CIDR block associated to the network.
* `display_text` - The network display text.
* `dns1` - The primary DNS server for the network.
* `dns2` - The secondary DNS server for the network.
* `gateway` - The IP address of the gateway.
* `name` - The network name.
* `network_domain` - The DNS domain for the network.
* `network_offering` - The name of the network offering associated to the network.
* `zone` - The name of the zone to which the network belongs.
