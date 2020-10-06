ipfs-counter
=======================

> Counting the server nodes in the IPFS DHT

Crawls the IPFS DHT to give some metrics. Note: this project is a WIP and resulting stats should not be taken as definitive.

## Documentation

### Install

`go install`

### Build

`go build`

### Usage

Running the application output prometheus metrics on the path `/metrics` with port 1234. Set the environment variable `IPFS_METRICS_PASSWORD` to control access to the prometheus metrics.

Network data is output into the `netdata` folder via levelDB

## Lead Maintainer

[Adin Schmahmann](https://github.com/aschmahmann)

## Contributing

Contributions are welcome! This repository is part of the IPFS project and therefore governed by our [contributing guidelines](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md).

## License

[SPDX-License-Identifier: Apache-2.0 OR MIT](LICENSE.md)
