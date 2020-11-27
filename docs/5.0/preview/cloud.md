---
title: Teleport Cloud Preview
description: Teleport hosted and managed by the Teleport team.
---

# Teleport Cloud Preview

Teleport cloud is a hosted, managed Teleport cluster.
Request access to the preview [here](https://goteleport.com/get-started/).

## Status

Teleport Cloud is currently only recommended for environments that do not require
meeting strict compliance standards or SLA uptime guarantees.

We have designed Teleport Cloud environment to be secure; however we are in the process of
scrutinizing and executing on our security roadmap and working with independent security
auditors to identify any gaps.

## Roadmap

For the first half of 2021, we will be working with auditors to realize the
threat modeling and security controls.

Once we realize, test and audit the security model we will start working towards meeting
compliance programs such as FedRAMP.

## Billing

Usage will be collected, but billing will not be set up until February.
The preview version of Teleport Cloud will be free of charge to customers until
we commit to a formal SLA.

## Managed Teleport Settings

SSH sessions are recorded [on nodes](../architecture/nodes.md#session-recording).
Teleport Cloud Proxy does not terminate SSH sessions when using OpenSSH and `tsh` sessions.
It terminates Application, Database and Kubernetes sessions on the proxy.

## Data retention

The session records are stored in S3 using at-rest encryption.
We have yet to define specific retention policies.

Customer data, including audit logging, is backed up using the DynamodDB
"point in time recovery" system. Data can be recovered from up to 35 days.
This retention period is not configurable.

## High availability

Clusters are deployed in a single AWS region in 2 availability zones.
AWS guarantees [99.99%](https://aws.amazon.com/compute/sla/) of monthly uptime percentage.

Teleport Cloud preview version does not commit to any SLA. It sets availability
objective to 99.7% of monthly uptime percentage (a maximum of two hours of downtime per month).

We plan to get to 99.99% uptime SLA.

# FAQ

### Can I use Teleport Cloud Preview in production?

It should be OK to use Cloud in non mission critical systems that do not
require 24/7 access or when fallback option is available.

### Are you using AWS-managed encryption keys, or CMKs via KMS?

We use AWS-managed keys. There is currently no option to provide your own key.

### Is this Teleport's S3 bucket, or my bucket based on my AWS credentials?

It's a Teleport-managed S3 bucket with AWS-managed keys.
There is no way to provide your own bucket.

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
We will publish it in Q1 2021 after fixing any security issues found.

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

### How does teleport manage web certificates? Can I upload my own?

Teleport uses [letsencrypt.org](https://letsencrypt.org/) to issue
certificates for every customer. It is not possible to upload a custom
certificate or use a custom domain name.

### Do you encrypt data at rest?

Each deployment is using at-rest encryption using AWS DynamoDB and S3 at-rest encryption
for customer data including session recordings and user records.
