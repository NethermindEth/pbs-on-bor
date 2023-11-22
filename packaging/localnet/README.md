# Local network setup

## What is being setup
 We will be setup local bor network without heimdall or L1 layer. One node is minning blocks another one is following (syncing) the chain and acts as a external block builder. 
 
Some code changes was required to skip validator set validation in syncing node, 
Important options are (see `config/*.toml`):
- `devfakeauthor = true`
- [heimdall]
  `"bor.without" = true`

## Setup

### Localnet directories structure

Copy the `localnet` directory somewhere outside. Cd into it and make other directories:
```bash
cd <PATH>/localnet
mkdir -p bin data data/keystore ipc logs
echo "pwd\npwd\npwd" > data/password.txt
```

here is the resulting directory structure:
```bash
├── Makefile
├── keys.txt
├── bin
├── config
│	 ├── genesis.json
│	 ├── node1_config.toml
│	 └── node2_config.toml
├── data
│	 ├── keystore
│	 └── password.txt
├── ipc
└── logs
```

Then follow instructions in `keys.txt` file to import accounts. These exact keys are needed or just `0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65` which is a coinbase account.

Next we need clone repositories to prepare proposer & builder binaries. Codebase is located here https://github.com/NethermindEth/pbs-on-bor/

To make use of the attached Makefile we need to clone code outside of the `localnet` directory. We will use `bor-proposer` and `bor-builder` directories for this purpose.


### Proposer node

For the block producer we use codebase of `branch-v1.0.4` branch of this repository. Important changes consist of the ability to fetch external blocks form the builder.

Clone the repository inside `../bor-proposer` folder:
```bash
# from localnet directory
cd ..
git clone -b branch-v1.0.4 --single-branch \
  https://github.com/NethermindEth/pbs-on-bor \
  bor-proposer
```

### Builder node

The builder is a Flashbots' builder code added on top of the bor node codebase. Builder specific endpoint is exposed on `localhost:28545` as you can see in `node2_config.toml` file along the other options.

```bash
# continuing 1 lvl above localnet directory (where we just cloned the proposer
git clone -b pnowosie/add_fb_builder --single-branch \
  https://github.com/NethermindEth/pbs-on-bor \
  bor-builder
```

### Running the network

In the terminal navigate back to the `localnet` directory and run:
```bash
make wipe wipe-logs wipe-bins bins
```

The result of running last task `bins` will build proposer binary as `./bin/bor` and the builder binary as `./bin/builder`.

Next in two terminal windows run:
- `make start1` to run the proposer node
- `make start2` to run the builder node

Logs of the nodes are captured in logs directory.

## Testing the network
You can interact with the nodes attaching to them
`bor attach ${ROOTDIR}/ipc/node_1/bor.ipc`
