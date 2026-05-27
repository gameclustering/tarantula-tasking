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

apps=("admin" "postoffice" "cloud" "nginx")
for app in "${apps[@]}"; do

  echo "pushing image : ${prefix}/${app}:${version}"
  sudo docker push ${prefix}/tarantula.$app:$version
  Check
done

Clean
