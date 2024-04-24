# Bring your own cluster

Allow different groups to bring their own Kubernetes cluster into Coder, or optionally build their own templates on entirely different infrastructure.

## Prerequisites

- A Coder deployment with [tiered RBAC](../README.md) enabled

## Tutorial

Not inside a list:

<div class="tabs">

## UI

## CLI

## HCL

```hcl
provider "coderd" {}

resource "coderd_organization" {
  name        = "data-science"
  description = "Lorem ipsum"
}
```

</div>

Text below it

1. Inside a list

    <div class="tabs">

    ## UI

    ## CLI

    ## HCL

    ```hcl
    provider "coderd" {}

    resource "coderd_organization" {
      name        = "data-science"
      description = "Lorem ipsum"
    }
    ```

    </div>

    For optimal auditing and self-service, we recommend managing organizations with infrastructure as code (HCL).

1. idk
