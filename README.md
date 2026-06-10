# Tarantula Tasking

A distributed task management and clustering system written in Go, designed for coordinating and executing jobs across multiple nodes.

## Documentation

- [Cluster Formation on VPS Platforms](docs/cluster-formation.md) — how nodes discover each other, hash ring partitioning, env vars, deploy sequence, and health checks

## Overview

Tarantula is a multi-service platform built around three core microservices that work together via cluster membership discovery and event streaming:

- **Admin** — Authentication, user management, cluster visibility, and task definitions
- **Cloud** — GCP-integrated resource provisioning and VM lifecycle management
- **PostOffice** — Cluster membership backbone, pub/sub event routing, and gRPC RPC layer

Traffic is routed through an Nginx reverse proxy (`/admin/*` → admin service, `/cloud/*` → cloud service), with supporting infrastructure (PostgreSQL, Vault) managed separately.

## Architecture

```
                         ┌─────────────┐
         HTTP :80        │    Nginx     │
        ─────────────────│  (reverse   │
                         │   proxy)    │
                         └──────┬──────┘
                                │
               ┌────────────────┼────────────────┐
               ▼                                 ▼
        ┌────────────┐                   ┌────────────┐
        │   Admin    │                   │   Cloud    │
        │  :8080     │                   │  :8080     │
        └─────┬──────┘                   └─────┬──────┘
              │                               │
              └──────────────┬────────────────┘
                             ▼
                    ┌─────────────────┐
                    │   PostOffice    │
                    │  HTTP :8080     │
                    │  gRPC  :7001    │
                    │  (memberlist)   │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
         PostgreSQL        Vault         BadgerDB
```

**Key design choices:**
- HashiCorp memberlist for distributed node discovery
- Consistent hash ring for data partitioning across nodes
- gRPC (port 7001) for inter-node RPC
- Pub/sub event streaming for async messaging between services
- BadgerDB (embedded KV) for PostOffice state; PostgreSQL for Admin and Cloud

## Services

### Admin (`app/web/admin/`)

Manages authentication, access control, and cluster visibility.

| Endpoint | Description |
|----------|-------------|
| `POST /admin/login` | User authentication (returns JWT) |
| `POST /admin/password` | Change password |
| `POST /admin/login/add` | Add a user (sudo only) |
| `POST /admin/accesskey` | Create an access key |
| `POST /admin/cs/message/send` | Publish a message to a topic |
| `GET  /admin/cs/query/topic/{topic}` | Query events on a topic |
| `GET  /admin/presence/hashring` | Inspect the cluster hash ring |
| `GET  /admin/presence/keyring/{key}` | Look up key → node assignment |
| `GET  /admin/presence/subscription/task` | View task subscriptions |
| `GET  /admin/metrics` | Prometheus metrics |

### Cloud (`app/web/cloud/`)

Provisions and manages GCP virtual machines and repositories. Communicates internally via gRPC; no public REST endpoints. Handles `create`, `update`, and `check` transaction types for VM and repository objects.

### PostOffice (`app/web/postoffice/`)

The messaging backbone. Runs cluster membership (memberlist), hosts the gRPC server, and routes pub/sub events across the cluster. No external REST endpoints.

## Directory Structure

```
tarantula-tasking/
├── app/web/
│   ├── admin/               # Admin service (main, handlers, config)
│   ├── cloud/               # Cloud service (main, handlers, config)
│   └── postoffice/
│       ├── postoffice.go    # Service entry point
│       └── clustering/      # Cluster membership logic
├── internal/
│   ├── bootstrap/           # Shared HTTP server setup & app lifecycle
│   ├── core/                # Cluster, auth, sequence, RPC abstractions
│   ├── event/               # Event factory implementations
│   ├── persistence/         # BadgerDB and PostgreSQL layers
│   ├── protocol/            # Generated protobuf (.pb.go) files
│   └── util/                # GCP, SSH, Vault, and other utilities
├── deploy/
│   ├── service-compose.yaml # Infrastructure services (PostgreSQL, Vault)
│   ├── vault-compose.yaml   # Vault-specific compose
│   ├── vault-config.hcl     # Vault server config
│   └── prometheus.yaml      # Prometheus scrape config
├── docker-compose.yaml      # Main application compose
├── nginx.conf               # Reverse proxy routing
├── build.bat / docker.sh    # Build scripts (Windows / Linux)
└── .env                     # Runtime environment variables
```

## Configuration

Each service loads a JSON config file at startup alongside environment variables from `.env`.

| Variable | Description |
|----------|-------------|
| `VERSION` | Image and artifact version tag |
| `ENV` | Runtime environment (`dev`, `prod`) |
| `SEQ` | Node sequence number |
| `VAULT_HOST` | HashiCorp Vault address |
| `VAULT_TOKEN` | Vault authentication token |
| `POST_OFFICE_HOST` | PostOffice hostname for cluster seed |

Service-level config files (`*-conf.json`) set `GroupName`, `NodeId`, `SqlEnabled`, and cluster seed IPs.

## Technology Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.24+ |
| RPC | gRPC + Protocol Buffers |
| Cluster membership | HashiCorp memberlist |
| Secrets | HashiCorp Vault |
| Embedded DB | BadgerDB v4 |
| SQL DB | PostgreSQL 16 (pgx/v5) |
| Logging | Zerolog |
| Metrics | Prometheus client |
| Tracing | OpenTelemetry |
| Cloud | Google Cloud Platform (auth, compute) |
| Auth | JWT, RBAC |
| Proxy | Nginx |
| Containers | Docker / Podman |

## Security

- JWT-based authentication on all protected endpoints
- Two access control levels: `ADMIN_ACCESS_CONTROL` and `SUDO_ACCESS_CONTROL`
- HashiCorp Vault stores SSH keys, auth certificates, and GCP credentials
- SSH key management for cloud VM provisioning

## Building and Running

**Build images (Windows):**
```bat
build.bat
```

**Build images (Linux):**
```sh
./docker.sh
```

**Start supporting infrastructure:**
```sh
docker compose -f deploy/service-compose.yaml up -d
```

**Start application services:**
```sh
docker compose up -d
```

Services are reachable at `http://localhost/admin/` and `http://localhost/cloud/` after startup.

## Event Topics

The pub/sub system routes messages across these named topics:

- `MESSAGE_TOPIC_NAME` — general messages
- `REGISTER_TOPIC_NAME` — node registration events
- `LOG_TOPIC_NAME` — structured log events
- `LOGIN_TOPIC_NAME` — authentication events
- `REQUEST_TOPIC_NAME` — inbound request tracking
- `TASK_TOPIC_NAME` — task lifecycle events
- `TRANSACTION_TOPIC_NAME` — distributed transaction coordination

## License

Apache License 2.0
