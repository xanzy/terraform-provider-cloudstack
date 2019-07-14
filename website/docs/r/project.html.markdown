---
layout: "cloudstack"
page_title: "CloudStack: cloudstack_project"
sidebar_current: "docs-cloudstack-resource-project"
description: |-
  Creates a Project.
---

# cloudstack_project

Creates a Project.

## Example Usage

Basic usage:

```hcl
resource "cloudstack_project" "default" {
  name         = "test-project"
  display_text = "test-project-display-text"
}
```

## Argument Reference

The following arguments are supported:

* `name` - (Required) The name of the Project.

* `display_text` - (Optional) The display text of the Project. Defaults to the name
  of the project.

* `account` - (Optional) The name of the Account to use for this Project. Changing
  this forces a new resource to be created.

* `domain_id` - (Optional) The name or ID of the Domain to use for this Project.
  Changing this forces a new resource to be created.

## Attributes Reference

The following attributes are exported:

* `id` - The ID of the Project.

## Import

Project's can be imported; use `<Project ID>` as the import ID. For
example:

```shell
terraform import cloudstack_project.default 84b23264-917a-4712-b8bf-cd7604db43b0
```
