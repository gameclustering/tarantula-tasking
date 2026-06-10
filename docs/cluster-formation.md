# Cluster Formation on VPS Platforms

This document describes how a Tarantula cluster forms across VPS nodes, what each node runs, and how to configure and deploy a new cluster from scratch.

---

## Cluster Overview

Each VPS node runs four Docker containers that form one cluster member:

| Container | Role |
|-----------|------|
| `postoffice` | Cluster backbone — memberlist gossip, gRPC RPC layer, pub/sub routing |
| `admin` | Auth, user management, hash ring visibility, task definitions (PostgreSQL) |
| `cloud` | GCP resource provisioning, VM and repository lifecycle (PostgreSQL) |
| `web` (Caddy) | TLS termination, HTTP reverse proxy → admin and cloud |

`admin` and `cloud` are satellites — they register with `postoffice` on the same host over the Docker bridge network. Only `postoffice` participates directly in cluster gossip.

---

## Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| `7946` | TCP + UDP | HashiCorp memberlist gossip between cluster nodes |
| `7001` | TCP | gRPC — inter-service RPC (HashRing, Request, Publish, Subscribe, …) |
| `8080` | TCP | HTTP API — proxies `/admin/*` → admin container, `/cloud/*` → cloud container |
| `80` / `443` | TCP | Public HTTP/HTTPS via Caddy (TLS auto-provisioned via Let's Encrypt) |

Firewall rules must allow **7946 (TCP+UDP), 7001, and 8080** between all cluster nodes. Ports 80 and 443 are public-facing.

---

## How Cluster Formation Works

### 1. Seed Discovery

Cluster join is driven by the `CLUSTER_BOOTSTRAP` environment variable.

- **First node (seed)**: `CLUSTER_BOOTSTRAP` is left empty. The node starts as a single-node cluster.
- **Subsequent nodes**: `CLUSTER_BOOTSTRAP` is set to the HTTP address of any live node (e.g. `http://104.238.154.226:8080`).

At startup, `postoffice` calls:
```
GET {CLUSTER_BOOTSTRAP}/postoffice/seeds
```
which returns the current memberlist IPs. The joining node passes these to `memberlist.Join()`.

Any live cluster node can serve as the bootstrap address — the seeds endpoint returns all current members, so even a two-node bootstrap is sufficient.

### 2. Memberlist Gossip

Once joined, nodes exchange membership state via HashiCorp memberlist on port 7946. Node add/remove/update events drive the hash ring (`MemberHashRing`).

### 3. Hash Ring Partitioning

Each physical node contributes **7 virtual nodes** (`NODE_WEIGHT = 7`) to the consistent hash ring. For a 5-node cluster:

```
5 nodes × 7 virtual nodes = 35 ring partitions
```

Keys are assigned to partitions by CRC32 hash. The ring rebalances automatically as nodes join or leave — only the partitions belonging to changed nodes are migrated.

### 4. gRPC Inter-Service Communication

`admin` and `cloud` containers connect to their local `postoffice` over `POST_OFFICE_HOST` (Docker bridge, e.g. `172.17.0.1`) on gRPC port 7001. All cluster RPC calls (HashRing, Publish, Subscribe, Request, etc.) go through this connection with a **10-second timeout**.

---

## Environment Variables

All containers share a single `.env` file on each VPS node.

| Variable | Required | Description |
|----------|----------|-------------|
| `VERSION` | Yes | Docker image tag (e.g. `dev2`, `v1.3`) |
| `PREFIX` | Yes | Docker Hub org prefix (e.g. `dockerlinkpop`) |
| `ENV` | Yes | Runtime environment prefix used in cluster key space (`dev`, `prod`) |
| `SEQ` | Yes | Sequence offset added to base `NodeId` from conf.json — **must be unique per node** |
| `CLUSTER_BOOTSTRAP` | Joining nodes | HTTP address of any live cluster node (empty on the first/seed node) |
| `POST_OFFICE_HOST` | Yes | Docker bridge IP for admin/cloud → postoffice gRPC connectivity |
| `POSTOFFICE_ADVERTISE_IP` | Yes | Public IP:port advertised to other cluster nodes for memberlist (e.g. `104.238.154.226:7946`) |
| `VAULT_HOST` | Yes | HashiCorp Vault HTTP address |
| `VAULT_TOKEN` | Yes | Vault authentication token |

### NodeId Uniqueness

Each service has a base `NodeId` in its `*-conf.json`. `SEQ` is added at runtime:

```
effective NodeId = conf.NodeId + SEQ
```

For a 5-node cluster with `SEQ=0..4`, postoffice node IDs become 3, 4, 5, 6, 7 (base 3 + SEQ).

---

## Service Configuration Files

Baked into each Docker image at `/etc/tarantula/`:

| File | Service | Key fields |
|------|---------|------------|
| `postoffice-conf.json` | postoffice | `GroupName`, `NodeId: 3`, `ClusterBootstrap` (overridden by env) |
| `admin-conf.json` | admin | `GroupName`, `NodeId: 1`, `SqlEnabled: true` |
| `cloud-conf.json` | cloud | `GroupName`, `NodeId: 2`, `SqlEnabled: true` |

`SqlEnabled: true` causes the service to provision its PostgreSQL schema on first start (credentials fetched from Vault).

---

## VPS Node Setup

### Automated Setup

`POST /admin/vps/setup` automates initial node provisioning:

1. SSHes into the VPS as `root`
2. Installs Docker and Git
3. Creates a `tarantula` OS user
4. Copies the Vault SSH public key for future access

### Manual Bootstrap Sequence

After automated setup, deploy the cluster node:

```bash
# 1. Clone repo on VPS
ssh tarantula@<vps-ip>
git clone <repo-url> ~/tarantula-tasking
cd ~/tarantula-tasking

# 2. Create .env — copy from another node and update node-specific values:
#    SEQ=<unique 0-based index>
#    POSTOFFICE_ADVERTISE_IP=<this-node-public-ip>:7946
#    CLUSTER_BOOTSTRAP=http://<any-live-node-ip>:8080   # empty on seed node

# 3. Create Docker network (first time only)
docker network create tarantula-app-net

# 4. Pull images and start
docker compose pull
docker compose up -d
```

### Scaling the Cluster

To add a node to a running cluster:

1. Run VPS setup on the new machine
2. Set `CLUSTER_BOOTSTRAP` to any existing node's `http://<ip>:8080`
3. Set a unique `SEQ` value
4. `docker compose up -d`

The new postoffice instance queries `/postoffice/seeds`, joins the gossip ring, and the hash ring rebalances automatically. No restart of existing nodes is required.

---

## Image Build and Deploy

Build all images locally:
```bash
bash docker.sh <tag>        # builds dockerlinkpop/tarantula.*:<tag>
```

Push to Docker Hub:
```bash
docker push dockerlinkpop/tarantula.postoffice:<tag>
docker push dockerlinkpop/tarantula.admin:<tag>
docker push dockerlinkpop/tarantula.cloud:<tag>
docker push dockerlinkpop/tarantula.caddy:<tag>
```

Deploy on each VPS node:
```bash
# update VERSION in .env
sed -i 's/VERSION=.*/VERSION=<tag>/' .env
docker compose pull && docker compose up -d --force-recreate
```

Use a new tag (not an existing one) to guarantee Docker Hub serves fresh layers.

---

## Health Checks

| Check | How |
|-------|-----|
| Cluster membership | `GET /admin/presence/hashring` — returns all ring partitions and their owning nodes |
| Key assignment | `GET /admin/presence/keyring/{key}` — returns which node owns a given key |
| Container status | `docker ps` on each node — all four containers should be `Up` |
| Startup log | `docker logs admin` — last line should be `Admin service started :8080` |

A healthy 5-node cluster shows **35 ring partitions** (7 per node) on the hash ring dashboard.

---

## Production Reference (Vultr, June 2026)

| Node | Host | Public IP | SEQ |
|------|------|-----------|-----|
| ps01 | ps01.gameclustering.com | 104.238.154.226 | 0 |
| ps02 | ps02.gameclustering.com | — | 1 |
| ps03 | ps03.gameclustering.com | — | 2 |
| ps04 | ps04.gameclustering.com | — | 3 |
| ps05 | ps05.gameclustering.com | — | 4 |

- ps01 is the seed node (`CLUSTER_BOOTSTRAP` empty); all others point to `http://104.238.154.226:8080`
- Admin UI runs separately on GCP at `https://admin.gameclustering.com`
- Vault runs on `192.168.1.11` (internal network accessible from all VPS nodes)
