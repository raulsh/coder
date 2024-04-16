# Tiered RBAC (Enterprise)

> Note: This feature is in active development and behind a closed beta. If you're interested in trying this, contact your account team.

Tiered RBAC is a suite of features that improves Coder's security posture for highly regulated organizations, particularly with multiple business units or security levels.

This includes:

- Organizations
- Custom Roles
- Custom Token Scopes

## Organizations

Organizations allow multiple platform teams to coexist within a single Coder instance with isolated cloud credentials, templates, and Coder provisioners. Unlike groups, users within an organization can hold roles that allow them to access all resources within the organization, but not the Coder deployment.

There are several use cases for this:

- The data science platform team wants to use their own cluster for data science workloads and needs to manage several templates and troubleshoot broken workspaces
- Contractors are isolated to their own organization, can't access resources in other organizations

Users can belong to multiple organizations. Workspaces, templates, provisioners, and groups are scoped to a single organization.

## Custom Roles

Custom roles can be created to give users a granular set of permissions within the Coder deployment or organization.

There are several cases for this:

- The "Banking Compliance Auditor" custom role cannot create workspaces, but can read template source code and view audit logs
- The "Team Lead" role can access user workspaces for trobuleshooting but cannot edit templates
- The "Platform Member" role cannot edit or create workspaces as they are created via a third-party system

## Custom Token Scopes

Custom Token scopes are functionally the same as custom roles, but are designed for service accounts and CI jobs

- A "Health Check" token can view deployment status but cannot create workspaces, manage templates, or view users
- A "CI" token can update manage templates but cannot create workspaces or view users
