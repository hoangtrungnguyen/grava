---
name: Check Serena Status
description: Verifies if the Serena MCP server container is running and responding at the SSE endpoint.
---

# Check Serena Server Status

This skill is used to quickly confirm that the Serena MCP server is active and accessible from the host system.

## 1. Quick Check (Docker Container)
To verify that the container named `serena-container` is currently running:
```bash
docker ps --filter "name=serena-container" --filter "status=running"
```

## 2. API Connectivity Check (SSE Endpoint)
To verify that the MCP server is responding at the expected port and path:
```bash
# Returns HTTP 200 OK if active
curl -I http://localhost:9121/sse 2>/dev/null | head -n 1
```

## 3. Project-Level Status Check
To view which projects are already indexed and ready for AI assistants (requires the container to be running):
```bash
sh serena/check.sh
```

## 4. Troubleshooting
If the server is not active:
- **Starting the server:** Run `docker-compose up -d` in the `serena/` directory.
- **Checking Logs:** Run `docker-compose logs -f` to see the initialization/Go compilation progress.
---
name: Check Serena Status
description: Verifies if the Serena MCP server container is running and responding at the SSE endpoint.
---

# Check Serena Server Status

This skill is used to quickly confirm that the Serena MCP server is active and accessible from the host system.

## 1. Quick Check (Docker Container)
To verify that the container named `serena-container` is currently running:
```bash
docker ps --filter "name=serena-container" --filter "status=running"
```

## 2. API Connectivity Check (SSE Endpoint)
To verify that the MCP server is responding at the expected port and path:
```bash
# Returns HTTP 200 OK if active
curl -I http://localhost:9121/sse 2>/dev/null | head -n 1
```

## 3. Project-Level Status Check
To view which projects are already indexed and ready for AI assistants (requires the container to be running):
```bash
sh serena/check.sh
```

## 4. Troubleshooting
If the server is not active:
- **Starting the server:** Run `docker-compose up -d` in the `serena/` directory.
- **Checking Logs:** Run `docker-compose logs -f` to see the initialization/Go compilation progress.
