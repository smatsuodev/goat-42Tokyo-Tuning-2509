#!/bin/bash -e

for config_path in "$@"; do
	dst_path="$(pwd)/common/${config_path#/}"
	if [ -e "$config_path" ]; then
		mkdir -p "$(dirname "$dst_path")"
		if [ -d "$config_path" ]; then
			sudo cp -auv "$config_path/." "$dst_path"
		else
			sudo cp -auv "$config_path" "$dst_path"
		fi
	fi
done

# NOTE: これで良いかは要検討
# permission denied を避けるため
sudo chown -R "$(whoami)" common/
