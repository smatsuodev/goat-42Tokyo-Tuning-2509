#!/bin/bash -e

config_dir_names=("common" "$(hostname)")

for config_dir_name in "${config_dir_names[@]}"; do
	(
		if cd "$config_dir_name" &> /dev/null; then
			find . -type f | while read -r rel_path; do
				src_path=$(pwd)/$rel_path
				dest_path=${rel_path#.}
				sudo cp -auv "$src_path" "$dest_path"
			done
		fi
	)
done
