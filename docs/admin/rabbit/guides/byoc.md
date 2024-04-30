# Bring your own cluster

Allow different teams to bring their own Kubernetes cluster into Coder, or optionally build their own templates on entirely different infrastructure.

## Before you start

First, you need a Coder deployment with [tiered RBAC](../README.md) enabled. You also must have the `Global Administrator` role.

## Option 1: Isolated Organizations

To give teams full autonomy of their Coder usage, templates, and infrastructure, consider giving them an isolated organization.

First, create an organization:

<div class="tabs">

## UI

Visit your deployment settings page: `https://coder.example.com/deployment/organizations`

![Create Organization](../../../images/rabbit/create-organization.png)

## CLI

```sh
coder organization create \
  --name data-science \
  --description "Templates managed by the data platform team"
```

## HCL

```hcl
provider "coderd" {}

resource "coderd_organization" {
  name        = "data-science"
  description = "Templates managed by the data platform team"
}
```

</div>

While template management is the responsibility of this new organization, you can optionally provide a set of starter templates that are tailored to them, outside of the Coder defaults:

![Starter templates](../../../images/rabbit/starter-templates.png)

The best way to give users access to this group is via claims identity provider claims:

<div class="tabs">

## coder server

The following server config maps people with the `ds` group claim to the `Everyone` group in the `data-science` organization:

```text
CODER_OIDC_GROUP_MAPPING '{"ds-user": "data-science::Member:Everyone"}'
CODER_OIDC_ROLE_MAPPING '{"ds-admin": "data-science:Administrator:Everyone"}'
```

## HCL

The following HCL maps people with the `ds-user` group claim to the `Everyone` group in the `data-science` organization:

```hcl
provider "coderd" {}

data "coder_organization" "data_science" {
  name = "data-science"
}

# Add users to the data science group
resource "coderd_idp_mapping" "data_science_members" {
  claim_name = "groups"
  claim_value_includes = "ds-user"
  organization_id = data.coder_organization.data_science.id
  group_id = data.coder_organization.data_science.default_group_id
  role_id = data.coder_organization.data_science.member_role_id
}

# Add admins to the data science group
resource "coderd_idp_mapping" "data_science_admins" {
  claim_name = "groups"
  claim_value_includes = "ds-user"
  organization_id = data.coder_organization.data_science.id
  group_id = data.coder_organization.data_science.default_group_id
  role_id = data.coder_organization.data_science.admin_role_id
}
```

</div>

Now, users can log in and gain access to the data science organization. Users with the `ds-admin` group in their identity provider will be able to manage provisioners, templates, and users within the organization.

## Option 2: Group Provisioners

If you want to manage a central set of templates, while allowing teams to bring their own infrastructure, group provisioners may be a good option.

First, create a custom role allowing certain organization members to create provisioners:

<div class="tabs">

## HCL

The `write:group-provisioner` role allows users to create provisioners in their organization on behalf of any groups they belong to.

```hcl
provider "coderd" {}

data "coder_organization" "default" {
  name = "default"
}

resource "coderd_custom_role" {
  organization = data.coder_organization.default.id
  name         = "provisioner-admin"
  description  = "Create provisioners"

  permission {
    action = "write"
    object = "group-provisioner"
  }
}
```
</div>

[Role sync](#) can be used to automatically add users to the `provisioner-admin` role based on group or role membership in the identity provider.

Users with this role will be able to create provisioners on behalf of their group:

<div class="tabs">

## UI

![Create a provisioner](../../../images/rabbit/create-provisioner.png)

## CLI

<!-- This needs work -->

```sh
coder provisionerd create \
  --scope=group \
  --group=data-science
```

</div>

When users are creating workspaces, they can now pick from a compatible provisioner:

![Provisioner picker](../../../images/rabbit/create-workspace-provisioner-picker.png)
