# gsearch-cli

Agent-friendly command line access to SDFI/Dataforsyningen GSearch.

GSearch is the Danish geodata search API for addresses, house numbers, roads,
postcodes, administrative geography, cadastral parcels, and place names. It is
not Google Search, and it is not a full DAWA replacement.

## Install

```sh
go install github.com/martincollignon/gsearch-cli@latest
```

During development:

```sh
go run . --help
```

## Authentication

Create a Dataforsyningen token and export it:

```sh
export GSEARCH_TOKEN="your-dataforsyningen-token"
```

The CLI sends the token as the documented `token` query parameter. It redacts
token values from transport errors.

## Commands

List supported resources:

```sh
gsearch-cli resources
```

Search any first-class GSearch resource:

```sh
gsearch-cli search husnummer "Søbakkevej 8, Tilst"
gsearch-cli search kommune aalborg
gsearch-cli search matrikel 123ab --limit 20
```

Get merged address suggestions:

```sh
gsearch-cli address suggest "Søbakkevej 8, Tilst"
```

This queries `husnummer` and `adresse` in parallel, prefers unit-level
`adresse` labels where available, and preserves the parent `husnummerId` for
DAR/Datafordeler BFE, BBR, and property workflows.

Run a nearest-house-number spatial lookup:

```sh
gsearch-cli spatial nearest-husnummer --easting 689255 --northing 6051787
```

Spatial filters use EPSG:25832 coordinates. Convert WGS84 latitude/longitude to
EPSG:25832 before calling this command.

Check token/service access:

```sh
gsearch-cli doctor
```

## Output

Commands return JSON by default. Use `--compact` for newline-delimited compact
JSON that is cheaper for agents to read.

## Boundaries

Use this CLI for GSearch workflows: search, autocomplete, filtered search, and
product-specific spatial lookup patterns.

Do not use GSearch as a replacement for DAWA `datavask`, DAWA reverse endpoint
semantics, BBR/BFE data access, history, replication, or bulk exports. Those
belong in DAR/Datafordeler, Adressevælger/Adressevask, downloads, events,
WFS/OGC, or dataset-specific tools.
