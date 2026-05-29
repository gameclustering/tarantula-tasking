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

if [[ -n "$2" ]]; then
    prefix="$2"
else
    prefix=dockerlinkpop
fi

echo "Build params : ${version} ${prefix}"

apps=("admin" "postoffice" "cloud")
for app in "${apps[@]}"; do
  echo "Current build target : $app"
  sudo docker build -f ./docker_application_build --build-arg app=$app --tag ${prefix}/tarantula.$app:$version .
  Check
  ((seq++))
done

sudo docker build -f ./docker_nginx_build --tag ${prefix}/tarantula.nginx:$version .
Check
sudo docker builder prune -af

Clean
