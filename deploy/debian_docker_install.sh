#!/bin/bash
sudo apt-get remove -y docker docker-engine docker.io containerd runc
# Update the apt package index
sudo apt-get update

# Install packages required for HTTPS repository access
sudo apt-get install -y ca-certificates curl gnupg lsb-release

# Create the keyring directory
sudo install -m 0755 -d /etc/apt/keyrings

# Download Docker's official GPG key
sudo curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc

# Set read permissions for all users
sudo chmod a+r /etc/apt/keyrings/docker.asc

# Add the Docker repository to apt sources
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Refresh package index to include Docker's repo
sudo apt-get update


# Install Docker Engine and related components
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Add current user to the docker group
sudo usermod -aG docker $USER

# Activate the new group in the current session
newgrp docker

# Enable Docker and containerd to start at boot
sudo systemctl enable docker.service
sudo systemctl enable containerd.service

# Create a daemon.json file with log rotation settings
sudo tee /etc/docker/daemon.json <<'EOF'
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}
EOF

# Restart Docker to apply the configuration
sudo systemctl restart docker