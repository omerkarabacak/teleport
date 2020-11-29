---
title: Teleport Cloud Preview
description: Teleport hosted and managed by the Teleport team.
---

# Teleport Cloud Preview

Teleport Cloud is a hosted, managed Teleport cluster. 
Currently Teleport Cloud is in preview status and we are maintaining a 
wait-list of interested parties wanting access to the service. 
To request access age get on the waiting list [here](https://goteleport.com/get-started/).

## Status

Teleport Cloud is currently only recommended for environments that do not require
strict compliance standards or SLA uptime guarantees.

We have designed Teleport Cloud environment to be secure; however we are still in the process of
scrutinizing and executing on our security roadmap and working with independent security
auditors to identify any gaps. Only once this is complete will we be able to to verify 
that it is ready for strict compliance use-cases.

## Roadmap

For the first half of 2021, we will be working with auditors to realize the
threat modeling and security controls.

Once we realize, test and audit the security model we will start working towards meeting
compliance programs such as FedRAMP.

## Billing

Usage data will be collected, but billing will not be set up until February.
The preview version of Teleport Cloud is provided with free of charge primarily
for testing and POC purposes until we commit to a formal SLA. 

## Managed Teleport Settings

SSH sessions are recorded [on nodes](../architecture/nodes.md#session-recording).
Teleport Cloud Proxy does not terminate SSH sessions when using OpenSSH and `tsh` sessions.
It terminates Application, Database and Kubernetes sessions on the proxy.

## Data retention

The session records are stored in S3 using at-rest encryption.
We have yet to define specific retention policies.

Customer data, including audit logging, is backed up using the DynamoDB
"point in time recovery" system. Data can be recovered up to 35 days.
This retention period is not configurable.

## High availability

Clusters are deployed in a single AWS region in 2 availability zones.
AWS guarantees [99.99%](https://aws.amazon.com/compute/sla/) of monthly uptime percentage.

Teleport Cloud preview version does not commit to any SLA. It sets availability
objective to 99.7% of monthly uptime percentage (a maximum of two hours of downtime per month).

We plan to get to 99.99% uptime SLA.

# FAQ

### Can I use Teleport Cloud Preview in production?

We expect Teleport Cloud to be OK to use in non mission critical systems
that do not require 24/7 access or a guaranteed SLA. We recommend a 
fallback option be make available. 

### Are you using AWS-managed encryption keys, or CMKs via KMS?

We use AWS-managed keys. Currently there is no option to provide your own key.

### Is this Teleport's S3 bucket, or my bucket based on my AWS credentials?

It's a Teleport-managed S3 bucket with AWS-managed keys.
Currently there is no way to provide your own bucket.

### How do I add nodes to Teleport Cloud?

You can connect servers, kubernetes clusters, databases and applications
using [reverse tunnels](../admin-guide.md#adding-a-node-located-behind-nat).

There is no need to open any ports on your infrastructure for inbound traffic.

### How can I access the tctl admin tool?

We have made changes to allow you to log into your cluster using `tsh`, then use `tctl` remotely:

```bash
$ tsh login --proxy=myinstance.teleport.sh:443
# tctl (Enterprise edition) can use tsh credentials as of version 5.0
$ tctl status
```

### Are dynamic node tokens available?

After [connecting](#how-can-i-access-the-tctl-admin-tool) `tctl` to Teleport Cloud, users can generate
[dynamic tokens](../admin-guide.md#short-lived-dynamic-tokens):

```bash
$ tctl nodes add --ttl=5m --roles=node,proxy --token=$(uuid)
```

### When will a security audit be available?

The Teleport Cloud security audit starts in January. 
We will work on fixing any security issues that are identified and 
publish it in Q1 2021.

### What does Teleport Cloud run in?

Teleport Cloud is deployed using a [Gravity](https://github.com/gravitational/gravity)
cluster on AWS.

### Will Teleport be automatically upgraded with each release?

We will be upgrading the preview version of Teleport Cloud automatically.
We will add an option to trigger upgrades manually and automatically
within a configurable maintenance window.

### Does your SOCII report include Teleport Cloud?

Our current SOCII Type2 report covers our organization, Teleport and Gravity OSS and Enterprise versions.
We are pursuing SOC2 for cloud and expect to receive our report at the end of Q2 2021
after our 6 month audit period is completed.

### Can a customer deploy multiple clusters in Teleport Cloud?

Not at this time.

### Is FIPS mode an option?

FIPS is not currently an option for Teleport Cloud clusters.

### How do you store passwords?

Password hashes are generated using
[Golang's bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt).

### How does Teleport manage web certificates? Can I upload my own?

Teleport uses [letsencrypt.org](https://letsencrypt.org/) to issue
certificates for every customer. It is not possible to upload a custom
certificate or use a custom domain name.

### Do you encrypt data at rest?

Each deployment is using at-rest encryption using AWS DynamoDB and S3 at-rest encryption
for customer data including session recordings and user records.
