#!/bin/sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo "usage: $0 IMAGE [MAX_MB]" >&2
  exit 2
fi

image_name=$1
max_mb=${2:-0}

image_bytes=$(docker image inspect "$image_name" --format '{{.Size}}')
image_mb=$(( (image_bytes + 1048575) / 1048576 ))

echo "image=$image_name"
echo "content_bytes=$image_bytes"
echo "content_mb=$image_mb"
echo "largest_layers:"
docker history --no-trunc --format '{{.Size}}\t{{.CreatedBy}}' "$image_name" | head -n 15

if [ "$max_mb" -gt 0 ] && [ "$image_mb" -gt "$max_mb" ]; then
  echo "budget=failed max_mb=$max_mb actual_mb=$image_mb" >&2
  exit 1
fi

if [ "$max_mb" -gt 0 ]; then
  echo "budget=passed max_mb=$max_mb actual_mb=$image_mb"
fi
