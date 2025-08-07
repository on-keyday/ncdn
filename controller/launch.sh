#!/bin/bash
set -e
go build -o ./secrets/c-plane ./controller/cmd/server
go build -o ./secrets/controller ./controller/cmd/controller

sudo ip netns exec O ./secrets/c-plane &
CPLANE_PID=$!
trap "sudo kill $CPLANE_PID" EXIT
sudo ip netns exec O ./secrets/controller 
