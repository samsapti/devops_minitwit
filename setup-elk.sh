#!/usr/bin/env bash

[[ $(id -u) -ne 0 ]] && exit 1
cd $(dirname ${BASH_SOURCE[0]})

path="docker/filebeat"
file="$path/filebeat.yml"
fileTemplate="$path/filebeat.template.yml"

if [[ ! $file -nt $fileTemplate ]]; then
    [[ -f $file ]] && rm $file
    cp $fileTemplate $file
fi
