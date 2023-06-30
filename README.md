# Kion Tool

The Kion tool is a command-line app that facilitates getting credentials from [Kion](https://kion.io) (formerly cloudtamer). It has three primary subcommands:

1. _credentials_ – Creates and prints temporary AWS credentials in various formats
2. _credential-process_ – Acts as a [credential process](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html), allowing the AWS CLI and tools such Terraform to transparently fetch credentials
3. _console_ – Opens the AWS console logged in to a certain account as a certain role

For help on a subcommand, run `kion help [subcommand]`. For a full list of subcommands, run `kion help`.

## Installation

Install [Go 1.19 or above](https://go.dev/doc/install). Then:

```
$ go install github.com/corbaltcode/kion/cmd/kion@latest
```

## Setup

Run `kion setup` to set up kion interactively. This subcommand asks for your Kion host, login info, and other settings and writes `~/.config/kion/config.yml` similar to the following:

```yaml
app-api-key-duration: 168h0m0s
host: kion.example.com
idms: 1
region: us-east-1
rotate-app-api-keys: true
session-duration: 1h0m0s
username: alice
```

## Fetching Credentials

The `credentials` subcommand fetches and prints credentials:

```
$ kion credentials --account-id 123412341234 --cloud-access-role my-role

aws_access_key_id = ASIAUJXFFQ7OTYJMNHWO
aws_secret_access_key = EacVBgDmom1RVwV+v78+ijNjIJAtOoUJeWQ3tVJ0
aws_session_token = FwoGZXIvYXdzEA8aDBN8L9LFhehhIpoaICKoAbwe ...
```

With `--format export`, credentials are printed in a format that can be evaluated to set environment variables:

```
$ kion credentials --account-id 123412341234 --cloud-access-role my-role --format export | source
$ aws sts get-caller-identity

{
    "UserId": "ASIAUJXFFQ7OTYJMNHWO:alice",
    "Account": "123412341234",
    "Arn": "arn:aws:sts::123412341234:assumed-role/my-role/alice"
}
```

The `credentials` subcommand also supports JSON:

```
$ kion credentials --account-id 123412341234 --cloud-access-role my-role --format json | jq -r .access_key

ASIAUJXFFQ7OTYJMNHWO
```

## Launching the AWS Console

The `console` subcommand launches the AWS console as a certain role in a certain account:

```
### Opens a browser
$ kion console --account-id 123412341234 --cloud-access-role my-role
```

## Config and kion.yml

The Kion tool searches the following locations for arguments, in this order:

1. Command line
2. `kion.yml` in the working directory
3. `~/.config/kion/config.yml`

If a directory is associated with a particular AWS account and role, you can avoid repeatedly supplying arguments on the command line by putting them in `kion.yml`. For example, in `/path/to/workspace`, create the following `kion.yml`:

```yaml
account-id: "123412341234"
cloud-access-role: my-role
```

Then the `credentials` and `console` commands can be reduced to:
 
```
$ cd /path/to/workspace

### Fetches credentials for role my-role in account 123412341234
$ kion credentials

### Opens the AWS console for role my-role in account 123412341234
$ kion console
```

## AWS CLI Credential Process

The AWS CLI can get credentials from another program called  a [credential process](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html).

To use the Kion tool as a credential process, create an AWS profile with the `credential_process` setting, supplying the full path to `kion` and using the `credential-process` subcommand:

```
[profile my-profile]
credential_process = /path/to/kion credential-process --account-id 123412341234 --cloud-access-role my-role
```

Now specify this profile when you run AWS CLI commands:

```
$ aws --profile my-profile sts get-caller-identity
```

In directories with `kion.yml`, arguments are supplied by the file, so you can use a more general profile:

```
[profile kion]
credential_process = /path/to/kion credential-process
```

Exporting `AWS_PROFILE` allows you to omit `--profile` so that you need no extra arguments:

```
$ export AWS_PROFILE=kion

### In a directory with kion.yml
$ aws sts get-caller-identity
```

## Credential Process Caching

To avoid repeatedly fetching credentials, `kion credential-process` caches credentials on disk. The creation time of each set of credentials is recorded, and new credentials are fetched when the session duration has elapsed. The session duration is given in the `session-duration` argument. `kion setup` asks for this value and saves it to `~/.config/kion/config.yml`.

## App API Keys

To reduce the use of highly privileged user credentials, Kion supports authentication with App API Keys. `kion setup` creates an App API Key by default an configures the tool to use it.

Your App API Key has a short lifetime (e.g. a week), so you must rotate it regularly. To do so, use the `key` subcommand:

```
$ kion key rotate
```

If `rotate-app-api-keys` is set to `true` in `~/.config/kion/config.yml`, the Kion tool will automatically rotate your App API Key within three days of expiration when any primary command is run. (`kion setup` enables automatic rotation by default.)

The `key` subcommand also handles the situation where your key expires — for example, you don't run the Kion tool for a while. The `--force` flag permits the tool to overwrite an existing, possibly expired key:

```
### May prompt for user credentials
$ kion key create --force
```

## User Credentials

If you choose not to use an App API Key, `kion setup` stores user credentials in the system keyring (Secret Service on Linux, Keychain on macOS, Credential Manager on Windows).

To update the user credentials in the system keyring (e.g. your password changes), use the interactive `login` subcommand:

```
$ kion login
```

To remove credentials from the system keychain:

```
$ kion logout
```

## Scenario: Terraform

Combining the features above, you can configure Terraform to fetch credentials from Kion transparently.

### 1. Create kion.yml

In `/path/to/terraform/workspace/kion.yml`:

```yaml
account-id: "123412341234"
cloud-access-role: my-role
```

### 2. Create AWS profile

In `~/.aws/config`:

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

### 4. Run Terraform commands

```
$ cd /path/to/terraform/workspace
$ terraform plan
```