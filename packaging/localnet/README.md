# Local network setup

## What is being setup
 We will be setup local bor network without heimdall or L1 layer. One node is minning blocks another one is following (syncing) the chain. Some code changes was required to skip validator set validation in syncing node,
 
Important options are (see `config/*.toml`):
- `devfakeauthor = true`
- [heimdall]
  `"bor.without" = true`

## Setup

1. Check out branch `branch-v1.0.4`
2. Build bor client by running `make bor`
3. Copy `bor` binary to `bin` folder of the localnet directory
4. Create directories for data and keystore
```
ROOTDIR=<localnet dir>
mkdir -p ${ROOTDIR}/data ${ROOTDIR}/data/keystore ${ROOTDIR}/ipc
echo "pwd" > ${ROOTDIR}/data/password.txt
cp ./config ${ROOTDIR}/
```
5. Import accounts, follow instructions in `keys.txt`, you need to import these exact keys or just `0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65` which is a coinbase account.
6. Run
```
cd ${ROOTDIR}
./bin/bor server --config config/node1_config.toml
# in another terminal:
./bin/bbuilder server --config config/node2_builder_config.toml 2>&1 | tee -a run.log
```

## Testing the network
You can interact with the nodes attaching to them
`bor attach ${ROOTDIR}/ipc/node_1/bor.ipc`
