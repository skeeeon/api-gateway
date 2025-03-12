# Permission System Documentation

This document explains in detail how the permission system works in the API Gateway, and how to configure and use it effectively.

## Permission Model

The API Gateway uses an MQTT-style topic pattern matching system for permissions. This approach was chosen to provide a unified permission model that works across both HTTP APIs and MQTT messaging systems.

### Core Concepts

1. **Permission Types**
   - **Publish Permissions**: Control write operations (POST, PUT, PATCH, DELETE)
   - **Subscribe Permissions**: Control read operations (GET, HEAD, OPTIONS)

2. **Permission Patterns**
   - Stored as arrays of strings in the PocketBase role
   - Follow MQTT topic pattern format
   - Support wildcards for flexible matching

3. **HTTP Path Mapping**
   - HTTP paths are mapped to MQTT topics by removing the leading slash
   - For example: `/api/v1/device/123` → `api/v1/device/123`

4. **Method Mapping**
   - GET, HEAD, OPTIONS → Check against subscribe permissions
   - POST, PUT, PATCH, DELETE → Check against publish permissions

## Wildcard System

The permission system supports two types of wildcards:

1. **Single-Level Wildcard (`+`)**
   - Matches exactly one path segment
   - Must occupy an entire segment (cannot be partial)
   - Example: `api/+/device` matches `api/v1/device` or `api/v2/device`

2. **Multi-Level Wildcard (`#`)**
   - Matches zero or more path segments
   - Must be the last character in the pattern
   - Example: `api/v1/#` matches `api/v1`, `api/v1/device`, `api/v1/device/123`, etc.

## Permission Evaluation

When a request comes in, the permission evaluation process works as follows:

1. Determine which permission type to check based on HTTP method
2. Convert the HTTP path to an MQTT topic format
3. For each permission pattern in the user's role:
   - Apply the MQTT topic matching algorithm
   - If any pattern matches, allow the request
4. If no pattern matches, deny the request

## Permission Pattern Strategies

### Hierarchical API Design

Structure your API paths hierarchically to take advantage of the wildcard system:

```
/api/v1/devices/{device_id}/readings/{reading_id}
```

This allows for permission patterns like:
- `api/v1/devices/#` (all device operations)
- `api/v1/devices/+/readings` (readings for any device, but not individual readings)
- `api/v1/devices/device-123/#` (all operations for a specific device)

### Recommended Patterns

1. **Administrative Access**
   ```json
   {
     "publish_permissions": ["api/#"],
     "subscribe_permissions": ["api/#"]
   }
   ```

2. **Read-Only Access**
   ```json
   {
     "publish_permissions": [],
     "subscribe_permissions": ["api/#"]
   }
   ```

3. **Limited Scope Access**
   ```json
   {
     "publish_permissions": ["api/v1/users/+", "api/v1/devices/+"],
     "subscribe_permissions": ["api/v1/users/+", "api/v1/devices/+", "api/v1/public/#"]
   }
   ```

4. **Single Resource Access**
   ```json
   {
     "publish_permissions": ["api/v1/users/user-123"],
     "subscribe_permissions": ["api/v1/users/user-123", "api/v1/public/#"]
   }
   ```

5. **Version-Specific Access**
   ```json
   {
     "publish_permissions": ["api/v1/#"],
     "subscribe_permissions": ["api/v1/#", "api/v2/#"]
   }
   ```

## Common Patterns and Examples

### Path Matching Examples

| Pattern | Path | Result | Explanation |
|---------|------|--------|-------------|
| `api/v1/devices/+` | `/api/v1/devices/123` | ✅ Match | Single-level wildcard matches device ID |
| `api/v1/devices/+` | `/api/v1/devices/123/readings` | ❌ No Match | Path has an extra segment |
| `api/v1/devices/#` | `/api/v1/devices/123/readings` | ✅ Match | Multi-level wildcard matches all sub-paths |
| `api/+/devices` | `/api/v1/devices` | ✅ Match | Single-level wildcard matches version |
| `api/+/+/readings` | `/api/v1/devices/readings` | ✅ Match | Multiple single-level wildcards |
| `api/+/+/readings` | `/api/v1/devices/123/readings` | ❌ No Match | Too many segments for pattern |

### HTTP Method Examples

| Method | Pattern Type | Pattern | Path | Result |
|--------|--------------|---------|------|--------|
| GET | subscribe | `api/v1/devices/+` | `/api/v1/devices/123` | ✅ Match |
| POST | publish | `api/v1/devices/+` | `/api/v1/devices/123` | ✅ Match |
| PUT | publish | `api/v1/devices/+` | `/api/v1/devices/123` | ✅ Match |
| DELETE | publish | `api/v1/devices/+` | `/api/v1/devices/123` | ✅ Match |
| GET | subscribe | `api/v1/devices/+` | `/api/v1/devices/123/readings` | ❌ No Match |
| GET | publish | `api/v1/devices/+` | `/api/v1/devices/123` | ❌ No Match (wrong permission type) |

## Setting Up Permissions in PocketBase

1. **Create Roles**
   - Navigate to the PocketBase admin UI
   - Go to the `mqtt_roles` collection
   - Create a new role with a name (e.g., "Admin", "User", "ReadOnly")
   - Add JSON arrays for publish and subscribe permissions

2. **Example Role Configurations**

   **Admin Role:**
   ```json
   {
     "name": "Admin",
     "publish_permissions": ["api/#"],
     "subscribe_permissions": ["api/#"]
   }
   ```

   **ReadOnly Role:**
   ```json
   {
     "name": "ReadOnly",
     "publish_permissions": [],
     "subscribe_permissions": ["api/#"]
   }
   ```

   **DeviceManager Role:**
   ```json
   {
     "name": "DeviceManager",
     "publish_permissions": ["api/v1/devices/#"],
     "subscribe_permissions": ["api/v1/devices/#", "api/v1/public/#"]
   }
   ```

3. **Assign Roles to Users**
   - Navigate to the users collection
   - Edit a user
   - Set the role_id field to the appropriate role

## Best Practices

1. **Design for Least Privilege**
   - Start with minimal permissions and add as needed
   - Avoid overly broad patterns like `#` at the top level

2. **Consistent API Path Design**
   - Use consistent path structures across your API
   - Group related resources under common prefixes
   - Use clear, semantic path segments

3. **Version Your API**
   - Include version in the path (e.g., `/api/v1/resource`)
   - This allows version-specific permissions

4. **Public vs. Protected Resources**
   - Use a consistent pattern for public resources (e.g., `/api/v1/public/`)
   - Grant broad read access to public resources

5. **Test Permission Patterns**
   - Verify patterns work as expected before deployment
   - Consider edge cases like empty segments or special characters

6. **Document Permission Requirements**
   - For each API endpoint, document required permissions
   - Make it clear which permission type (publish/subscribe) is needed

7. **Audit and Review**
   - Regularly review role permissions
   - Look for overly permissive patterns
   - Check for unused or redundant patterns

## Advanced Features

### Permission Logging

You can enable debug-level logging to see detailed permission checks:

```json
{
  "logLevel": "debug"
}
```

This will log:
- The HTTP path and method
- The mapped MQTT topic
- The permission type being checked
- The patterns being evaluated
- The result of each pattern match

### Custom Permission Mapping

If you need more advanced permission mapping beyond the default:

1. Extend the `Matcher` implementation
2. Customize the `MapPathToTopic` method
3. Add special handling for specific API patterns or methods

## Troubleshooting

### Common Issues

1. **403 Forbidden Errors**
   - Check user's role and permissions
   - Verify HTTP method maps to correct permission type
   - Ensure path format matches pattern structure
   - Check for trailing slashes (they count as segments)

2. **Unexpected Permissions**
   - Look for overly broad patterns (`#` or `+`)
   - Check for conflicting patterns
   - Verify role assignment is correct

3. **Permission Changes Not Taking Effect**
   - Check if cache TTL has expired
   - Force refresh the cache
   - Verify changes were saved in PocketBase

### Debugging Steps

1. Enable debug logging
2. Check the logs for permission evaluation details
3. Verify user authentication is successful
4. Confirm role retrieval works
5. Check pattern matching logic for the specific path
