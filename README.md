# Kion Tool

The Kion tool is a command-line app that automatically fetches credentials from [Kion](https://kion.io) (formerly cloudtamer) when you run commands such as `aws` and `terraform`. It uses the official [Kion CLI](https://github.com/kionsoftware/kion-cli) Go library for browser-based SAML authentication. See [Scenario: Terraform](#scenario-terraform) for an example of how it works fully configured.

The tool has three primary subcommands:

1. `credentials` – Creates and prints temporary AWS credentials in various formats
2. `credential-process` – Acts as a [credential process](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html), allowing the AWS CLI and tools such Terraform to transparently fetch credentials
3. `console` – Opens the AWS console logged in to a certain account as a certain role

For help on a subcommand, run `kion help [subcommand]`. For a full list of subcommands, run `kion help`.

## Installation

Install [Go 1.25.11 or above](https://go.dev/doc/install). Then:

```
$ go install github.com/corbaltcode/kion/cmd/kion@latest
```

## Setup

Run `kion setup` to set up kion interactively. SAML is the default authentication method. The command opens a browser for authentication and writes `~/.config/kion/config.yml` similar to the following:

```yaml
app-api-key-duration: 168h0m0s
auth-method: saml
host: kion.example.com
idms: 1
rotate-app-api-keys: true
saml-metadata-file: https://idp.example.com/saml/metadata
saml-print-url: false
saml-sp-issuer: https://kion.example.com/api/v1/saml/auth
session-duration: 1h0m0s
```

The `host` setting is the Kion hostname without a URL scheme, such as `kion.example.com`; do not include `https://`.

The `auth-method` setting accepts `saml` or `password`; when omitted, it defaults to `password` for compatibility with existing configurations. The SAML browser callback listens on local port 8400. Set `saml-print-url: true` when the browser must be opened manually. Username/password authentication remains available during setup for Kion installations that still support it.

## Testing Authentication

`internal/client/client_test.go` contains live integration tests that authenticate against a Kion installation. The tests require environment variables and make real network requests. Set `KION_HOST` to the hostname without `https://`.

To test username/password authentication:

```bash
export KION_AUTHMETHOD=password
export KION_HOST=kion.example.com
export KION_IDMS=1
export KION_USERNAME='<username>'
export KION_PASSWORD='<password>'

go test -v -count=1 ./internal/client
```

To test SAML authentication:

```bash
export KION_AUTHMETHOD=saml
export KION_HOST=kion.example.com
export KION_IDMS=1
export KION_SAML_METADATA_FILE='https://idp.example.com/saml/metadata'
export KION_SAML_SP_ISSUER='https://kion.example.com/api/v1/saml/auth'
unset KION_USERNAME KION_PASSWORD

go test -v -count=1 ./internal/client
```

The SAML login test forces a fresh browser login. The invalid-credentials test is skipped for SAML because it only applies to username/password authentication.

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

SAML access and refresh tokens are stored in the operating system keychain. When the short-lived access token expires, the tool uses the refresh token to obtain another one. If the refresh token is unavailable or expired, the browser authentication flow runs again.

## App API Keys

To reduce the use of highly privileged user credentials, Kion supports authentication with App API Keys. `kion setup` creates an App API Key by default an configures the tool to use it.

Your App API Key has a short lifetime (e.g. a week), so you must rotate it regularly. To do so, use the `key` subcommand:

```
$ kion key rotate
```

If `rotate-app-api-keys` is set to `true` in `~/.config/kion/config.yml`, the Kion tool will automatically rotate your App API Key within three days of expiration when any primary command is run. (`kion setup` enables automatic rotation by default.)

The `key` subcommand also handles the situation where your key expires, for example when you don't run the Kion tool for a while. The `--force` flag permits the tool to authenticate with SAML and overwrite an existing, possibly expired key:

```
### May open a browser for SAML authentication
$ kion key create --force
```

## Authentication

When SAML is configured, `login` opens the browser and replaces the cached SAML session:

```
$ kion login
```

To remove the cached SAML session:

```
$ kion logout
```

### Legacy Username and Password

If you choose not to use an App API Key, `kion setup` stores user credentials in the system keyring (Secret Service on Linux, Keychain on macOS, Credential Manager on Windows).

To update the user credentials in the system keyring (e.g. your password changes), use the interactive `login` subcommand:

```
$ kion login
```

To remove legacy username/password credentials from the system keychain:

```
$ kion logout
```

## Printing Access Info

The `access` subcommand prints the current user's Cloud Access Roles and associated accounts. Each line contains a Cloud Access Role, account ID, and account name:

```
$ kion access
role1	123412341234	account1
role1	234123412341	account2
role2	123412341234	account1
role2	234123412341	account2
```

The list can be filtered with the `--cloud-access-role` (`-r`), `--account-id`, and `--account` flags:

```
$ kion access --cloud-access-role role1
role1	123412341234	account1
role1	234123412341	account2
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
