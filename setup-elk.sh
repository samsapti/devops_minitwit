#!/usr/bin/env sh

[ $(id -u) -ne 0 ] && exit 1

path="docker/filebeat"
file="$path/filebeat.yml"
fileTemplate="$path/filebeat.template.yml"

if [ ! "$file" -nt "$fileTemplate" ]; then
    [ -f "$file" ] && rm "$file"
    cp "$fileTemplate" "$file"
fi
