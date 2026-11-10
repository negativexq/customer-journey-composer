#!/bin/sh
set -eu
# Pinned scanner release; no source tree or environment secrets are mounted.
scanner='aquasec/trivy:0.76.2@sha256:8f6bb947dbc528902e11f2f5be52f7b519f6a39d5d148f41166f2040499768fb'
for image in customer-journey-composer-api:dev customer-journey-composer-web:dev; do
 docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -v cjc-trivy-cache:/root/.cache/ "$scanner" image --no-progress --scanners vuln --severity HIGH,CRITICAL --exit-code 1 "$image"
done
