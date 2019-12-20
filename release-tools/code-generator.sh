#!/usr/bin/env bash

CURRENT_DIR=$(pwd)

PROJECT_MODULE="github.com/maxaudron/zfs-csi-driver"
IMAGE_NAME="cocainefarm/zfs-csi-driver:generator"

CUSTOM_RESOURCE_NAME="foo"
CUSTOM_RESOURCE_VERSION="v1"

cmd="./generate-groups.sh all \
    "$PROJECT_MODULE/pkg/client" \
    "$PROJECT_MODULE/pkg/apis" \
    $CUSTOM_RESOURCE_NAME:$CUSTOM_RESOURCE_VERSION"

echo "Generating client codes..."
podman run --rm -u $(id -u):$(id -g) \
           -v "${PWD}:/go/src/${PROJECT_MODULE}" \
           "${IMAGE_NAME}" $cmd

