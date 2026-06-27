# Agent Notes

This repo is a small CLI for SDFI/Dataforsyningen GSearch, not Google Search.

Rules for changes:

- Keep credentials in `GSEARCH_TOKEN` or a secret manager. Never hardcode a real token.
- Use the documented GSearch base URL: `https://api.dataforsyningen.dk/rest/gsearch/v2.0/{resource}`.
- Preserve returned UUIDs and structured fields. Do not parse `visningstekst` as source-of-truth data.
- Use `husnummer` for building/access-address search and `adresse` for unit-level address search.
- Spatial filters sent to GSearch must use EPSG:25832 coordinates.
- GSearch does not replace DAWA reverse geocoding, `datavask`, BBR/BFE, history, replication, or bulk-download workflows.
