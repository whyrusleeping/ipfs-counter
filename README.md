# dht-graph
A simple libp2p DHT crawler

Crawls the `/ipfs/kad/1.0.0` and `/ipfs/kad/2.0.0` DHTs and dumps the resulting routing tables into a GraphViz compatible dot file.

Run with `dht-graph -o outputFile`, if no output file is specified it will default to `rt.dot`.
