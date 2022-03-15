COMBCore
--------
Prototype Wallet/Node for [Haircomb](https://github.com/natasha-otomoski/haircomb/blob/master/WhyTheCombOfNatashaOtomoskiHas21Teeth.txt) built around [libcomb](https://github.com/dyoform/libcomb).

COMBCore primarily implements commit storage and mining as well as wallet loading and saving.
It also exposes an RPC control interface which provides everything needed to interact with Haircomb.

A Qt GUI built on COMBCore's control interface is available [here](https://github.com/dyoform/combcore-gui).

Enhancements over combfullui:
- commit mining from Bitcoin Cores block files.
- commit mining using Bitcoin Cores REST interface.
- BTC block hashes are used to enforce mining order.
- every commit is now stored instead of just previously unseen ones.
- deciders are no longer defined in a chain.
- undecided merkle segments can now be stored in your wallet.
- testnet mode (compliant with [watashi's testnet](https://bitbucket.org/watashi564/combfullui-0.3.4-testnet/src/master/testnetpaper.txt))

Unimplemented features of combfullui:
- used key detection
- contract templates

Wont implement features of combfullui:
- commit mining from Bitcoin Cores RPC interface (highly insecure and slow).

Future features:
- Lightwallet support (working on this now!)
- Full test suite for libcomb
- P2P mining and validation
- Integerated DEX (requires fork)


Config
------
Example config for running COMBCore and Bitcoin Core on the same machine.
Set the `btc_data` path to enable direct mining (very fast).

in config.ini
```ini
[btc]
btc_peer = 127.0.0.1
#btc_data = /path/to/btc/data
btc_port = 8332
[combcore]
comb_network = mainnet
comb_host = 127.0.0.1
comb_port = 2211
```
in bitcoin.conf
```ini
[main]
server=1
rest=1
rpcport=8332
rpcallowip=127.0.0.1
rpcbind=127.0.0.1
```

Testnet Config
--------------
Example config for running COMBCore and Bitcoin Core on the same machine and in Testnet mode.

in config.ini
```ini
[btc]
btc_peer = 127.0.0.1
#btc_data = /path/to/btc/data
btc_port = 18332
[combcore]
comb_network = testnet
comb_host = 127.0.0.1
comb_port = 2211
```
in bitcoin.conf
```ini
testnet=1
[test]
server=1
rest=1
rpcallowip=127.0.0.1
rpcbind=127.0.0.1
rpcport=18332
```

Pushing Blocks
--------------
Specify a client to push blocks to via config.ini
```ini
[push]
push_client_ip = 10.0.0.1
push_client_port = 2211
```
Disable BTC mining on the client by not specifying a BTC peer. Complete client config.ini:
```ini
[combcore]
comb_host = 10.0.0.1
comb_port = 2211
```

Building
--------
```bash
git clone --recursive https://github.com/dyoform/combcore
cd combcore
go get
go build
./combcore
```