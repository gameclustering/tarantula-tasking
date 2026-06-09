#!/bin/bash

Clean(){
    echo "deleting build files"
}
Check(){
    if [[ $? -ne 0 ]]; then
        echo "build failed, try again"
        Clean
        exit $?
    fi
}

if [[ -n "$1" ]]; then
    version="$1"
else
    version=latest
fi

echo "Build params : ${version}"

apps=("admin" "postoffice" "cloud")
for app in "${apps[@]}"; do
  echo "Current build target : $app"
  podman build  -f ./docker_application_build --build-arg app=$app --tag tarantula.$app:$version .
  Check
  ((seq++))
done

podman build  -f ./docker_caddy_build --tag tarantula.caddy:$version .
Check
podman image prune --filter "label=stage=builder"

Clean
