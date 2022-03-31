#!/usr/bin/env bash

cd $(dirname ${BASH_SOURCE[0]})

file="docker/filebeat/filebeat.yml"
fileTemplate="$file.template"

if [[ ! $file -nt $fileTemplate ]]; then
    [[ -f $file ]] && sudo rm $file

    cp $fileTemplate $file
    sudo chown root $file
fi