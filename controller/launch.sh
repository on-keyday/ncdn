#!/bin/bash
set -e
go build -o ./secrets/c-plane ./controller/cmd/server
go build -o ./secrets/controller ./controller/cmd/controller
go build -o ./secrets/console ./controller/cmd/console

#sudo ip netns exec O ./secrets/c-plane &
./secrets/c-plane &
CPLANE_PID=$!
trap "kill $CPLANE_PID" EXIT
#sudo ip netns exec O ./secrets/controller 
./secrets/controller 
