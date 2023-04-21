# kion

_kion_ is a command-line interface to Kion (cloudtamer). It enables the AWS CLI and Terraform to fetch credentials from Kion transparently.

## Installation

```
go install github.com/corbaltcode/kion/cmd/kion@latest
```

## Setup

Run `kion setup` to set up kion interactively. This command writes _~/.config/kion/config.yml_, e.g.:

```yaml
host: kion.example.com
idms: 1
region: us-east-1
session-duration: 1h0m0s
username: alice
```

## Usage

_kion_ has three primary subcommands:

- _console_ – Opens the AWS console logged in to a certain account as a certain role
- _credentials_ – Creates and prints temporary AWS credentials in various formats
- _credential-process_ – Acts as a [credential process](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html) for the AWS CLI

For help on a command, run `kion help [command]`. For a full list of commands, run `kion help`.

## Examples

### Create & Print Credentials

```
$ kion credentials --account-id 123412341234 --cloud-access-role my-role --format export

export AWS_ACCESS_KEY_ID=ASIA ...
export AWS_SECRET_ACCESS_KEY=kgQn ...
export AWS_SESSION_TOKEN=FwoG ...
```

### Launch AWS Console

```
$ kion console --account-id 123412341234 --cloud-access-role my-role
```

A browser opens with the AWS console logged in to the given account. You may supply the `--print` flag to print the URL instead of opening a browser.


### Supply Credentials to AWS CLI

Create an AWS profile with the _credential\_process_ setting. For example, in _~/.aws/config_:

```
[profile my-profile]
credential_process = /path/to/kion credential-process --account-id 123412341234 --cloud-access-role my-role
```

Now `aws --my-profile kion s3 ls` generates temporary credentials and uses them to list buckets. The temporary credentials are cached on the local filesystem to speed up successive runs.

## Using with Terraform

_kion_ can be set up so that Terraform transparently fetches temporary AWS credentials.

### 1. Create kion.yml

Create _/path/to/terraform/workspace/kion.yml_:

```yaml
account-id: "123412341234"
cloud-access-role: my-role
```

_kion_ looks for arguments on the command line, _kion.yml_ in the working directory, and _~/.config/kion/config.yml_, in that order.


### 2. Create AWS profile

In _~/.aws/config_, add:

```
[profile kion]
credential_process = /path/to/kion credential-process
```

### 3. Set profile in Terraform provider block

```hcl
provider "aws" {
  profile = "kion"
}
```

If using remote state on S3, set _profile_ in the _backend_ block as well.

### 4. Run Terraform commands

To create a plan with temporary AWS credentials from Kion:

```
$ cd /path/to/terraform/workspace
$ terraform plan
```

The AWS provider is set up to use the "kion" profile. The profile uses the credential process _kion_, which finds _kion.yml_, containing the appropriate arguments, in the current working directory. 