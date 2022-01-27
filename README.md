![IMG](https://user-images.githubusercontent.com/96906027/147944368-66d18a6e-81cc-4c05-b26b-5c1872c60a16.png)
# Renloi
Renloi is an open-source, non-profit EVM-compatible Proof of Authority blockchain network.
## Vision
A digital decentralized version of cash will allow extremely fast transactions and low fees and an open-source fair economy. Renloi's decentralized finance (DeFi) ecosystem never sleeps or discriminates will allow anyone to make payments using only their internet connection. With Renloi, you can send, receive, borrow, earn interest, and even stream funds anywhere and anytime. To operate, you just need to set up a wallet, where you will be in complete control of your personal data.

## Executables

The Renloi source comes with several executables found in the `cmd` directory.

|    Command    | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| :-----------: | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
|  **`renloi`**   | Renloi's main CLI client. capable of running as a full node (default), archive node (retaining all historical state) or a light node (retrieving data live). It can be used by other processes as a gateway into the Renloi network via JSON RPC endpoints exposed on top of HTTP, WebSocket and/or IPC transports. 
|   `clef`    | Stand-alone signing tool, which can be used as a backend signer for `renloi`.  |
|   `devp2p`    | Utilities to interact with nodes on the networking layer, without running a full blockchain. |
|   `abigen`    | Source code generator to convert Renloi contract definitions into easy to use, compile-time type-safe Go packages. It operates on plain [Ethereum contract ABIs](https://docs.soliditylang.org/en/develop/abi-spec.html) with expanded functionality if the contract bytecode is also available. However, it also accepts Solidity source files, making development much more streamlined. Please see our [Native DApps](https://geth.ethereum.org/docs/dapp/native-bindings) page for details. |
|  `bootnode`   | Stripped down version of our Renloi client implementation that only takes part in the network node discovery protocol, but does not run any of the higher level application protocols. It can be used as a lightweight bootstrap node to aid in finding peers in private networks.                                                                                                                                                                                                                                                                 |
|     `evm`     | Developer utility version of the EVM (Ethereum Virtual Machine) that is capable of running bytecode snippets within a configurable environment and execution mode. Its purpose is to allow isolated, fine-grained debugging of EVM opcodes (e.g. `evm --code 60ff60ff --debug run`).                                                                                                                                                                                                                                                                     |
|   `rlpdump`   | Developer utility tool to convert binary RLP ([Recursive Length Prefix](https://eth.wiki/en/fundamentals/rlp)) dumps (data encoding used by the Renloi protocol both network as well as consensus wise) to user-friendlier hierarchical representation (e.g. `rlpdump --hex CE0183FFFFFFC4C304050583616263`).                                                                                                                                                                                                                                 |
|   `puppeth`   | A CLI wizard that aids in creating a new Ethereum-compatible network.     

## Build Renloi
Make sure you have golang installed.
Build for your operating system:

`$ make renloi`

## Build all executables
Make sure you have golang installed.
Build for your operating system:

`$ make all`

## Renloi's community
* Website https://renloi.org
* Discord https://discord.gg/renloi
* Telegram https://t.me/renloi_org



