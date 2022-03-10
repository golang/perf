#!/bin/bash
# Copyright 2022 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -xe

# One-time project setup:
#
# 1. Create artifact registry for the project:
#
# $ gcloud artifacts repositories create golang-perf-docker-repo \
#     --repository-format=docker \
#     --location=us-central1 --description="Docker repository"
#
# 2. Configure authentication for (sudo) docker:
#
# $ sudo gcloud auth configure-docker us-central1-docker.pkg.dev
#
# 3. Create the secrets to store InfluxDB passwords/tokens in:
#
# $ gcloud secrets create influx-admin-pass
# $ gcloud secrets create influx-admin-token
# $ gcloud secrets create influx-reader-pass
# $ gcloud secrets create influx-reader-token
#
# 4. Grant access to the GCE default service account to update the secrets.
#
# $ export SERVICE_ACCOUNT=$(gcloud iam service-accounts list --format="value(EMAIL)" --filter="displayName:Compute Engine default service account")
# $ gcloud secrets add-iam-policy-binding influx-admin-pass --member=serviceAccount:${SERVICE_ACCOUNT} --role="roles/secretmanager.secretVersionAdder"
# $ gcloud secrets add-iam-policy-binding influx-admin-token --member=serviceAccount:${SERVICE_ACCOUNT} --role="roles/secretmanager.secretVersionAdder"
# $ gcloud secrets add-iam-policy-binding influx-reader-pass --member=serviceAccount:${SERVICE_ACCOUNT} --role="roles/secretmanager.secretVersionAdder"
# $ gcloud secrets add-iam-policy-binding influx-reader-token --member=serviceAccount:${SERVICE_ACCOUNT} --role="roles/secretmanager.secretVersionAdder"

# TODO(prattmic): This is getting complicated; move to Go program, including
# initial one-time setup above.

if [[ $# != 2 ]]; then
  echo "Usage: $0 <gcp project> <docker registry>"
  exit 1
fi

declare -r PROJECT=$1
declare -r REGISTRY=$2

declare -r TAG="${REGISTRY}/golang-influx:latest"

echo "Building Docker image..."
# We must be in this directory for docker.
(cd "$(dirname -- "${BASH_SOURCE[0]}")" && sudo docker build -t ${TAG} .)

echo "Pushing Docker image..."
sudo docker push ${TAG}

# TODO(prattmic): Set up VM with no external IP.

echo "Creating instance..."
gcloud --project ${PROJECT} compute instances create-with-container influx-test-1 \
  --container-image ${TAG} \
  --zone us-central1-a \
  --machine-type n1-standard-1 \
  --tags https-server \
  --scopes cloud-platform # Access fully controlled via IAM.
