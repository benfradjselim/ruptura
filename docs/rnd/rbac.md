# Role-Based Access Control (RBAC)

OHE enforces multi-tenant, role-based access control to ensure secure, isolated environments for different teams.

## Roles
OHE implements three primary roles, enforced via API middleware:
1. **admin:** Full access to all org-wide resources, including user management, org configuration, and system-wide settings.
2. **operator:** Can manage alerts, dashboards, datasources, and notification channels. Cannot manage users.
3. **viewer:** Read-only access to metrics, logs, traces, and dashboards.

## Implementation
Roles are embedded within the JWT token claims. The API router uses custom handlers to wrap endpoints requiring specific roles:
- `adminOnly`: Middleware enforcing `admin` role.
- `operatorOnly`: Middleware enforcing `operator` role (and above).

## Multi-Tenancy
Access is strictly scoped by `org_id` (passed via `X-Org-ID` header or JWT claim). API requests without a valid `org_id` context will be rejected.
