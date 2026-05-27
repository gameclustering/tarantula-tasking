#!/usr/bin/env bash
# Sets up Docker Engine on a fresh Debian 12 (Bookworm) instance.
# Usage: bash setup-docker-debian.sh [username]
# If username is omitted, the current user is added to the docker group.

set -euo pipefail

USER_TO_ADD="${1:-$(whoami)}"

echo "==> Updating apt..."
sudo apt-get update -qq

echo "==> Installing prerequisites..."
sudo apt-get install -y -qq ca-certificates curl

echo "==> Adding Docker GPG key..."
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

echo "==> Adding Docker apt repository..."
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

echo "==> Installing Docker Engine..."
sudo apt-get update -qq
sudo apt-get install -y -qq \
  docker-ce docker-ce-cli containerd.io \
  docker-buildx-plugin docker-compose-plugin

echo "==> Adding '$USER_TO_ADD' to docker group..."
sudo usermod -aG docker "$USER_TO_ADD"

echo "==> Enabling Docker service..."
sudo systemctl enable --now docker

echo ""
echo "Docker $(docker --version) installed."
echo "Log out and back in (or run 'newgrp docker') for group membership to take effect."
