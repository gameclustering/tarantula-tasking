## Help 
### openssl.exe req -newkey rsa:2048 -nodes -keyout domain.key -x509 -days 365 -out domain.crt -subj "/CN=gameclustering.com" -addext "subjectAltName=IP:192.168.1.11,IP:192.168.1.3"

### vps setup  adduser tarantula usermod -aG sudo tarantula ssh-copy-id sudo ufw allow OpenSSH sudo ufw enable