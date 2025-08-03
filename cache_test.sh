#!/bin/bash

export NVM_DIR="$HOME/.nvm" && [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh" && [ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"
sudo ip netns exec U bash -c '. /home/dpdk/.nvm/nvm.sh && cd cache-tests && ./test-host.sh 192.0.2.10:8890' | cat > ./cache-tests/results/on-keyday.json
# sudo ip netns exec O bash -c '. /home/dpdk/.nvm/nvm.sh && cd cache-tests && npm run server --port=8888'
# go build -o /tmp/ncdn-bin/popcache ./popcache && sudo ip netns exec C0 sudo -u dpdk  /tmp/ncdn-bin/popcache -originURL http://192.168.88.30:8888/ -nodeId C0 -listenAddr 192.0.2.10:8889 -lbNodeId 0 -certFile ca/certs/20250719194728/server.crt -keyFile ca/private/20250719194728/server.key
