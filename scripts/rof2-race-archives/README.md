# rof2-race-archives

Builds `internal/http/staticmaps/rof2-race-archives.json`: which RoF2 client
archives hold each race model code. Race Loading (Starfall) offers these
archives when a race is added to a `_chr.txt` or `GlobalLoad.txt`.

It reads the archives themselves: an `.s3d` names its models as
`CODE_ACTORDEF` in its WLD string table, an `.eqg` as `code.mod` / `code.mds`.
Only codes Spire's race list or the client's `_chr.txt` files know are kept,
so zone objects with three-letter names stay out.

```sh
python3 -I scripts/rof2-race-archives/scan.py <RoF2 client folder> \
  internal/http/staticmaps/race-inventory-map.json \
  internal/http/staticmaps/rof2-race-archives.json
```
