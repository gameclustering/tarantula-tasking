@echo off
docker network create --driver bridge --subnet 10.20.0.0/16 --gateway 10.20.0.1 tarantula-app-net
docker network create --driver bridge --subnet 10.30.0.0/16 --gateway 10.30.0.1 tarantula-service-net