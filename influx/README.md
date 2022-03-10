# InfluxDB container image

This directory contains the source for the InfluxDB container image used in the
Go Performance Monitoring system. The image is based on the Google-maintained
GCP InfluxDB 2 image, with an additional small program to perform initial
database setup and push access credentials to Google Secret Manager.

## Local

To run an instance locally:

    $ sudo docker build -t golang_influx . && sudo docker run --rm -p 443:8086 golang_influx

Browse / API connect to https://localhost:8086 (note that the instance uses a
self-signed certificate), and authenticate with user 'admin' or 'reader' with
the password or API token logged by the container.

## Google Cloud

Perform the one-time project setup described in `setup.sh`, and then run the
script to start an instance:

    $ ./setup.sh <project> us-central1-docker.pkg.dev/<project>/golang-perf-docker-repo

The instance can be accessed via the "EXTERNAL IP" in the output. View
[VM instance logs](https://console.cloud.google.com/compute/instances) to verify
successful setup.

The authentication credentials are stored in the project's Secret Manager. e.g.,
to access the admin password:

    $ gcloud secrets versions access latest --secret=influx-admin-pass
