ipfs-counter
=======================

> Counting the server nodes in the IPFS DHT

Crawls the IPFS DHT to give some metrics. Note: this project is a WIP and resulting stats should not be taken as definitive.

## Documentation

### Compile

`go build`

### Usage

`go run . ...`

Arguments:
```
   --output value               Output file location (default: "crawl-output")
   --dataset value              Google biquery dataset ID for insertion
   --table value                Google bigquery table prefix for insertion
   --create-tables              To create bigquery tables if they do not exist (default: false)
   --seed-file value            Use peers from a file to seed crawling
   --seed-table value           Use peers / multiaddrs from previous trial table to seed crawling
   --seed-table-duration value  when seeding from table, select date range for querying hosts (default: 168h0m0s)
   --parallelism value          How many connections to open at once (default: 1000)
   --timeout value              How long to wait on dial attempts (default: 5s)
   --crawltime value            How long to crawl for (default: 20h0m0s)
   --debug                      Print debugging messages (default: false)
```

Local Example (saves to crawl-output.json):
```
go run . --parallelism 300 --crawltime 5000s
```

Cloud Example:
```
GOOGLE_APPLICATION_CREDENTIALS=./creds.json go run . --dataset dev_dataset --table dev_table --crawltime 500s --parallelism 300
```

### Deployment

* Tags committed to this repository are configured to trigger a google `cloudbuild` of the project. This follows the definition in `cloudbuild.yaml` to regenerate a docker image and push it with the `latest` tag to the google cloud internal container repository.
* The cron job defined in `kubernetes.yml` is applied to a k8s cluster in google cloud with permissions to read and write to big query. it will each night pull the latest docker image from the internal repo and run the crawl task.
* For changes to code, a tag here is sufficient for the next night's run to use the updated code.
* For changes to k8s configuration, e.g. changing the table that should be written to, the changed kubernetes.yml needs to be applied manually. this can be done through google cloud-console with the following commands:
    ```
    gcloud config set project resnetlab-p2p-observatory
    gcloud config set compute/zone us-central1-c
    gcloud container clusters get-credentials ipfs-counter
    kubectl apply -f kubernetes.yml
    ```

## Lead Maintainers

[Adin Schmahmann](https://github.com/aschmahmann)
[Will Scott](https://github.com/willscott)

## Contributing

Contributions are welcome! This repository is part of the IPFS project and therefore governed by our [contributing guidelines](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md).

## License

[SPDX-License-Identifier: Apache-2.0 OR MIT](LICENSE.md)
