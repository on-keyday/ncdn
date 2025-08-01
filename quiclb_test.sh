#!/bin/bash
set -e
# must be root
if [ "$(id -u)" != "0" ]; then
  echo "This script must be run as root" 1>&2
  exit 1
fi
FILE_PREFIX=metrics/$(date +%Y%m%d%H%M%S)
mkdir -p ${FILE_PREFIX}
# NETSETUP=./l4lb/netns_setup.sh
# BACKEND=./l4lb/run-be.sh

LB_LAUNCH=./l4lb/run-lb.sh
NO_LB_LAUNCH=./l4lb/nolb.sh
$LB_LAUNCH > ${FILE_PREFIX}/lb.log 2>&1 &
PID_FILE=/run/ncdn/l4lb.pid
PID_FILE_TRIAL=3
for i in $(seq 1 $PID_FILE_TRIAL); do
  if [ -f $PID_FILE ]; then
    LB_PID=$(cat $PID_FILE)
    echo "Load balancer process started with PID: $LB_PID"
    break
  fi
  echo "Waiting for load balancer to start... (Attempt $i/$PID_FILE_TRIAL)"
  sleep 2
done

trap "kill $LB_PID" EXIT
sleep 5
REQ_COUNT=100000
TRIAL_COUNT=10
PARALLEL=
PARALLEL_LIMIT=1000
SKIP_LB=false

if [ "$SKIP_LB" != true ]; then
for i in $(seq 1 $TRIAL_COUNT); do
  echo "Trial $i"
  # ip netns exec U ./secrets/qclient -mode QUIC -parallelLimit $PARALLEL_LIMIT $PARALLEL -metricsFile ${FILE_PREFIX}/lb_exist_quic_${i}${PARALLEL}.json -requestCount $REQ_COUNT -serverAddress 192.0.2.10:8889 -rootCA /mnt/ncdn/ca/certs/20250719194728/root_ca.crt
  ip netns exec U ./secrets/qclient -mode TCP -parallelLimit $PARALLEL_LIMIT $PARALLEL -metricsFile ${FILE_PREFIX}/lb_exist_tcp_${i}${PARALLEL}.json -requestCount $REQ_COUNT -serverAddress 192.0.2.10:8889 -rootCA /mnt/ncdn/ca/certs/20250719194728/root_ca.crt
done
fi

# kill the load balancer
kill $LB_PID
echo "Load balancer process $LB_PID is terminating"
trap '' EXIT
sleep 5
$NO_LB_LAUNCH
for i in $(seq 1 $TRIAL_COUNT); do
  echo "Trial $i"
  ip netns exec U ./secrets/qclient -mode QUIC -parallelLimit $PARALLEL_LIMIT $PARALLEL -metricsFile ${FILE_PREFIX}/lb_not_exist_quic_${i}${PARALLEL}.json -requestCount $REQ_COUNT -serverAddress 192.0.2.10:8889 -rootCA /mnt/ncdn/ca/certs/20250719194728/root_ca.crt
  ip netns exec U ./secrets/qclient -mode TCP -parallelLimit $PARALLEL_LIMIT $PARALLEL -metricsFile ${FILE_PREFIX}/lb_not_exist_tcp_${i}${PARALLEL}.json -requestCount $REQ_COUNT -serverAddress 192.0.2.10:8889 -rootCA /mnt/ncdn/ca/certs/20250719194728/root_ca.crt
done
echo "All trials completed."
echo "Results saved with prefix: $FILE_PREFIX"
