---
authors: Roman Tkachenko (roman@goteleport.com)
state: draft
---

# RFD 73 - Teleport Discover

## Required approvers

@klizhentas && @xinding33 && @jimbishopp

## What

Proposes the `teleport discover` command that simplifies the UX for the
first-time users who are connecting their cloud resources to Teleport.

## Related RFDs

- [RFD 38: AWS databases auto-discovery](./0038-database-access-aws-discovery.md)
- [RFD 57: AWS EC2 nodes auto-discovery](https://github.com/gravitational/teleport/pull/12410)

## Why

The proposal is aimed at providing an easy way for Teleport administrators to
connect their cloud-hosted resources (EC2 instances, RDS databases, EKS clusters
an so on) to a Teleport cluster.

Over the past few releases Teleport has been adding automatic discovery
capabilities allowing it to find and register AWS databases, EC2 instances
and (WIP as of this writing) EKS clusters. Despite providing an improved UX
compared to registering the resources manually, connecting resources to cluster
and setting up auto-discovery remains cumbersome with multiple different
`teleport configure` commands and does not provide good visibility into the
discovered resources.

The `teleport discover` approach aims to take advantage of the existing auto
discovery mechanisms Teleport has and provide a unified approach for users to
enroll their cloud resources.

## Scope

- Works with both self-hosted edition and Teleport Cloud.
- Focuses on discovering resources in AWS, other clouds to follow.
- Covers currently supported cloud resources: EC2 instances and RDS databases.

## Prerequisites

In order to use `teleport discover` the user will need:

- Running Auth and Proxy, or a Cloud account.
- An AWS EC2 instance to run `teleport discover` command from. The node must have
  IAM identity allowing it to perform discovery operations in AWS account.

## Command

The goal for `teleport discover` is to eventually support multiple cloud
providers. Given that different providers often have different concepts and
different names for similar things, it may be hard to unify them all under a
single `teleport discover` command without making UX confusing, having
overlapping or prefixed flags, etc.

Hence, the proposal is to have a family of subcommands for different cloud
providers:

```
$ teleport discover aws ...
$ teleport discover gcp ...
```

This RFD will focus on `teleport discover aws`.

## UX

Administrator setting up Teleport will use `teleport discover aws` command on
an AWS instance that has IAM permissions required to discover and connect
resoures in this environment. We'll detail those permissions later.

The command will:

1. Using the instance's credentials, search the cloud account for the supported
   resources, matching some filters user can specify on the CLI.
2. Display the discovered resources to the user and ask for the confirmation if
   the user would like to connect them.
3. Once confirmed, import the resources in the cluster. Details below on what
   this means for each particular resource type.
4. Generate Teleport configuration and start Teleport service locally that will
   act as auto-discovery and/or database agent. This will assume that Teleport
   was installed from an apt/rpm repo and has a systemd service installed.

For EC2 nodes `teleport discover` will:

- Use SSM to install a Teleport SSH agent on each EC2 instance using the same
  approach as described in RFD 57.
- If auto-discovery is enabled via a CLI flag:
  - Enable it in the generated config file.
  - Update IAM permissions for the IAM role attached to the instance so it can
    run the discovery, similar to `teleport ssh configure`.

For RDS and other AWS databases `teleport discover` will:

- Register discovered databases in Teleport using dynamic resource registration.
  Will require appropriate Teleport permissions.
- If auto-discovery is enabled via a CLI flag:
  - Enable it in the generated config file.
  - Update IAM permissions for the IAM role attached to the instance so it can
    run the discovery, similar to `teleport db configure`.

Usage example:

```sh
$ teleport discover aws --proxy=proxy.example.com --token=xxx
🔑 Found AWS credentials, account 1234567890, user alice
🔍 Looking for EC2, RDS, Redshift and EKS resources in us-east-1, us-east-2 regions
🔍 Hint: use --types and --regions flags to narrow down the search
🔍 Found the following matching resources:

Type          Name/ID            Region      Tags
---------------------------------------------------------------------
AWS EC2       node-1/i-12345     us-east-1   env:prod,os:ubuntu
AWS EC2       node-2/i-67890     us-east-1   env:prod,os:centos
AWS EC2       node-1/i-54321     us-west-2   env:test
AWS RDS       mysql-prod         us-east-1   env:prod,engine:mysql
AWS RDS       postgres           us-east-1   env:test,engine:postgres
AWS Redshift  redshift-1         us-west-2   team:warehouse

❓ Would you like to connect all found resources? yes/no
🚜 Installing Teleport SSH service on [node-1/i-12345] in us-east-1... ✅
🚜 Installing Teleport SSH service on [node-2/i-67890] in us-east-1... ✅
🚜 Installing Teleport SSH service on [node-1/i-54321] in us-west-2... ✅
🛢 Registering RDS MySQL database [mysql-prod] from us-east-1... ✅
🛢 Registering Aurora PostgreSQL database [postgres] from us-east-1... ✅
🛢 Registering Redshift database [redshift-1] from us-west-2... ✅
🔨 Updating IAM permissions for auto-discovery for this instance's role... ✅
🔨 Generated Teleport configuration file /etc/teleport.yaml ✅
🔨 Starting Teleport service... ✅
🎉 Done!
```

Optional command flags:

`--regions`          | Cloud regions to search in. Defaults to US regions.
`--types`            | Resource types to consider. Defaults to `ec2`, `rds`, `redshift`, `eks` (when implemented).
`--labels`           | Labels (tags in AWS) resources should match. Defaults to `*: *`.
`--ec2`              | Selectors for EC2 nodes e.g. `us-east-1:env:prod` or `us-west-2:*:*`.
`--rds`              | Selectors for RDS databases.
`--redshift`         | Selectors for Redshift clusters.
`--elasticache`      | Selectors for Elasticache clusters.
`--eks`              | Selectors for EKS clusters (when implemented).
`--enable-discovery` | Enables auto-discovery on the agent with the same filters the command is run with.
`--config-out`       | If provided, write config file to specified path instead of starting Teleport.

## Security

The instance where the user runs `teleport discover` will need to have the
following AWS IAM permissions:

* List/describe permissions for EC2, RDS and other resources user wants to connect.
* SSM permissions to be able to run commands on EC2 instances to install agents.
* IAM permissions to be able to setup IAM permissions for auto-discovery.

The discover command will never ask the user to enter any credentials and instead
rely on standard cloud provider credential chain.

## Auto-discovery

When auto-discovery is requested, `teleport discover` will enable it in the
agent's configuration.

For EC2 auto-discovery:

```yaml
ssh_service:
  enabled: "yes"
  aws:
  - types: ["ec2"]
    regions: ["us-east-1"]
    tags:
      "*": "*"
```

For database auto-discovery:

```yaml
db_service:
  enabled: "yes"
  aws:
  - types: ["rds", "redshift", ...]
    regions: ["us-east-1"]
    tags:
      "*": "*"
```

### IAM

Auto-discovery requires specific IAM permissions on the node that runs an agent
performing discovery.

`teleport discover` will use the same mechanism for attaching appropriate IAM
roles used by `teleport db configure` and `teleport ssh configure` commands.

## Teleport Cloud / UI

`teleport discover` works with both self-hosted and Cloud versions of Teleport
as it only requires Auth and Proxy.

The discover flow is CLI-driven and due to the security constraints (i.e. no
access to cloud account credentials) Teleport Web UI cannot run the discovery
for the user.

Web UI can be updated to include a wizard-like interface guiding the user
through the steps necessary to connect resources to the cluster:

Step 1. Select the cloud provider to connect resources from.
Step 2. Select resource types to connect from the supported list.
Step 3. Display IAM policy required to successfully execute initial discovery.
Step 4. Display `teleport discover` command for user to run.

## Scenarios

### Default behavior

User runs default discover command which discovers all EC2 instances, databases
and EKS clusters in US regions.

```
$ teleport discover aws --proxy=proxy.example.com --token=xxx
```

### Auto-discovery

User wants to select resources matching particular labels and enable discovery.

```
$ teleport discover aws --proxy=proxy.example.com --token=xxx \
    --labels=teleport:true \
    --enable-discovery
```

### Multiple accounts

User wants to connect resources from multiple accounts and runs discovery twice,
from instance in each account.

From instance in account-a:

```
$ teleport discover aws --proxy=proxy.example.com --token=xxx \
    --regions=us-east-1,us-east-2
```

From instance in account-b:

```
$ teleport discover aws --proxy=proxy.example.com --token=xxx \
    --regions=us-west-1,us-west-2
```
