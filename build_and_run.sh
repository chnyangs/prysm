go mod tidy
# 
bazel build //cmd/beacon-chain:beacon-chain
#  
./bazel-bin/cmd/beacon-chain/beacon-chain_/beacon-chain --execution-endpoint=http://localhost:8551 --mainnet --jwt-secret='/Users/xyan0559/project/ethereum/jwt.hex' --checkpoint-sync-url=https://beaconstate.info --genesis-beacon-api-url=https://beaconstate.info 