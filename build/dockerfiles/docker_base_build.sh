#! /usr/bin/env bash
set -e

TAG=$(date +%y%m%d)
TARGETS="sgx-runtime sgx-sdk sgx-go"

for target in $TARGETS; do
    echo "Building: $target"
    docker build --target $target -t balajia/$target:latest -t balajia/$target:$TAG -f Dockerfile.base .
    docker push balajia/$target
    echo "Build complete: $target"
done

