#!/usr/bin/env bash

cd $(dirname ${BASH_SOURCE[0]})

path="docker/filebeat"
file="$path/filebeat.yml"
fileTemplate="$path/filebeat.template.yml"

if [[ ! $file -nt $fileTemplate ]]; then
    [[ -f $file ]] && sudo rm $file

    cp $fileTemplate $file
    sudo chown root $file
fi