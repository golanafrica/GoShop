# Tenant routing

- Shop-scoped API: TenantResolver + RequireShopAccess (X-Shop-Slug only outside production)
- /api/shops CRUD: auth + owner check in usecase
- /api/admin/*: RequireRoles only
- Public: API key / invitation tokens