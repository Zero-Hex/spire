# classic-models

When two archives define the same race model (the classic skeleton `SKE` in
`global_chr.s3d` and the newer one in `global6_chr.s3d`), the client uses the
model from the one loaded first but takes same-named textures from the one
loaded last, so the older model shows with the newer textures. Race Loading
lists such pairs.

`rename_models.py` gives an older model a race number of its own instead: it
copies the archive with the model's race code renamed to that of a race the
client knows but has no model for. RoF2's `eqgame.exe` lists `SKL` (race 484,
Skeleton), `SPW` (483, Spirit Wolf) and `SCW` (481, Scaled Wolf), and no RoF2
archive defines them.

```sh
python3 -I scripts/classic-models/rename_models.py <client>/global_chr.s3d \
  starfall_classic_chr.s3d SKE=SKL WOE=SPW
```

- Renamed: the WLD's names (meshes, materials, actors, animation tracks
  such as `D05SKE_TRACK`), the bitmap names its textures list, and the texture
  files (`ske*.bmp`). Codes are the same length, so nothing in the WLD moves.
- The WLD is named after the new archive, and the directory's name checksums
  (CRC-32 of the lower-case name and its null) are recomputed.
- Every other model in the source archive is copied unchanged. They are the
  same as the ones the source archive already loads.

Then upload the archive to Client Patch (Beta only first), add it to
`Resources/GlobalLoad.txt` in Race Loading, and set NPCs to the new race
number (484 for the classic skeleton, 483 for the classic wolf). Keep the
original archives in their stock order so the old race numbers keep the
newer models with their own textures.
