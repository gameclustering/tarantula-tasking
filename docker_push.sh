#!/bin/bash

Clean(){
    echo "deleting tags files"
}
Check(){
    if [[ $? -ne 0 ]]; then
        echo "publish failed, try again"
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

echo "publish image to : ${prefix}"

apps=("admin" "postoffice" "google_cloud" "vultr_cloud")
for app in "${apps[@]}"; do

  echo "pushing image : ${prefix}/tarantula.${app}:${version}"
  docker push ${prefix}/tarantula.$app:$version
  Check
done

Clean
