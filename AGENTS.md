# Agent Notes

This repo is a small CLI for SDFI/Dataforsyningen GSearch, not Google Search.

## Operating Baseline

- Lifecycle: supported utility. Maintainer: Frederik Brunø Lottrup (Pendio
  Engineering); lifecycle changes require Engineering owner approval.
- Runtime: Go 1.22, declared in `go.mod`; CI pins Go 1.22.12.
- Entry point: this `AGENTS.md`; no shared foundation submodule is installed.
- Required local validation: `make validate`. It is credential-free and must
  remain so. Do not run live checks in CI.

Rules for changes:

- Keep credentials in `GSEARCH_TOKEN` or a secret manager. Never hardcode a real token.
- Use the documented GSearch base URL: `https://api.dataforsyningen.dk/rest/gsearch/v2.0/{resource}`.
- Preserve returned UUIDs and structured fields. Do not parse `visningstekst` as source-of-truth data.
- Use `husnummer` for building/access-address search and `adresse` for unit-level address search.
- Spatial filters sent to GSearch must use EPSG:25832 coordinates.
- GSearch does not replace DAWA reverse geocoding, `datavask`, BBR/BFE, history, replication, or bulk-download workflows.
