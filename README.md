# cosmos-cli

Download images and videos from [cosmos.so](https://cosmos.so) collections.

## Install

```sh
go install github.com/jblwilliams/cosmos-cli@latest
```

Or build from source:

```sh
git clone https://github.com/jblwilliams/cosmos-cli.git
cd cosmos-cli
go build -o cosmos-cli .
```

## Usage

Download a public collection:

```sh
cosmos-cli download https://www.cosmos.so/username/collection-name
```

### Login

Private collections require authentication:

```sh
cosmos-cli login --email you@example.com
```

Or pass a bearer token directly:

```sh
cosmos-cli login --token YOUR_TOKEN
```

### Download Options

```sh
cosmos-cli download https://www.cosmos.so/username/collection-name \
  --output ./my-images \
  --delay 0.2 \
  --no-skip
```

| Flag | Description |
|------|-------------|
| `-o, --output` | Output directory (default: `cosmos_downloads/<user>_<collection>`) |
| `-t, --token` | Override saved auth with a one-off token |
| `--delay` | Delay between downloads in seconds (default: 0.1) |
| `--no-skip` | Re-download files that already exist |

### Logout

```sh
cosmos-cli logout
```

## Credentials

Credentials are stored in `~/.config/cosmos/auth.json` with `0600` permissions.
