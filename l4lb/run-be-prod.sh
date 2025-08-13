#!/bin/bash
set -e

export MY_USER=${USER}
export SRC_DIR=$(readlink -f $(dirname $0)/..)
export BIN_DIR=/tmp/ncdn-bin
export CA_DIR=${SRC_DIR}/ca
export CA_SERIAL=20250719194728
mkdir -p ${BIN_DIR}
export LOG_DIR=/tmp/log
export QUIC_GO_LOG_LEVEL="debug"
mkdir -p ${LOG_DIR}

go build -o ${BIN_DIR}/popcache ${SRC_DIR}/popcache

controlPlaneAddr="${CONTROL_PLANE_ADDRESS:-ws://192.168.88.30:8080}"
originAddr="${ORIGIN_ADDRESS:-http://192.168.88.30:8888/}"
NODE_ID="${NODE_ID:-C0}"
VIP_ADDR="${VIP_ADDR:-192.0.2.10:8888}"

#ip addr add ${VIP_ADDR} dev lo || true

${BIN_DIR}/popcache -controlPlane ${controlPlaneAddr} -originURL ${originAddr} -nodeId ${NODE_ID} -listenAddr ${VIP_ADDR} 
#-certFile ${CA_DIR}/certs/${CA_SERIAL}/server.crt -keyFile ${CA_DIR}/private/${CA_SERIAL}/server.key
