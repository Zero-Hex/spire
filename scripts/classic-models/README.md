# classic-models

The RoF2 client shows a newer model for some classic race numbers (the
skeleton, race 60, uses the updated `SKE` in `global6_chr.s3d`). Line order in
GlobalLoad.txt does not fix it cleanly. The client takes same-named textures
from the archive loaded last, and classic models borrow most of their
animations from another race by race number. These scripts give a classic
model a race number of its own instead.

## bake_skeleton.py

Builds the classic skeleton as race 197 (`RON`, "Ronnie Test": a classic-era
race the client knows that has no model). Tested in game.

```sh
python3 -I scripts/classic-models/bake_skeleton.py <RoF2 client folder> \
  starfall_skeleton_chr.s3d SKE RON ELM
```

- It copies `global_chr.s3d` with `SKE` renamed `RON` (`rename_models.py`).
- Classic models borrow animations they lack: the skeleton borrows from the
  wood elf male `ELM` in `global_chr.s3d`. The borrowing rule is in
  Zero-Hex/Everquest_Model_Viewer's `CLASSIC_BORROW`. The 36 animations the
  skeleton lacks are copied in as `<anim>RON<bone>_TRACK`, each fitted to the
  skeleton like a borrowing client does: a bone's shift scaled by its rest
  length over the lender's, and its own scale kept. A bone an animation leaves
  alone is held at rest.
- `global3_chr.s3d`'s extra wood elf animations (a player's sitting pose,
  idles, social emotes) are left out; with them the skeleton sits like a
  player. `--global3` adds them.
- Sitting, as the classic skeleton does it: the client plays the sit-down
  `P02` forwards to sit and holds the sitting pose `P07`. The skeleton's `P02`
  runs from a heap of bones to standing, so its frames are turned round
  (standing to heap), and `P07` is the heap, held. Without a `P07` the
  skeleton stands straight back up.
- Result: 44 animations of its own, all from the client's files. Tested in
  game: it stands, moves, fights, collapses when it sits, stays down and
  reassembles when it stands.

In game: upload it to Client Patch, add `3,0,TFFFC,starfall_skeleton_chr,
Loading Characters` to Resources/GlobalLoad.txt, and set NPCs to race 197
with size 4. The server's default height for race 197 is 6, which floats
them.

The archive is then cut down to the skeleton alone (`trim_model.py`): 0.40 MB
instead of the 7.4 MB `global_chr.s3d` it starts from.

## trim_model.py

Cuts a character archive down to one model: its actor, skeleton, meshes,
materials, textures and animation tracks, and the texture files they name.
It follows the references between pieces, numbers the kept pieces again, and
keeps the string table whole so names need no change. A piece type it has no
reference layout for stops it rather than being copied wrongly.

```sh
python3 -I scripts/classic-models/trim_model.py in.s3d out.s3d RON
```

## rename_models.py

Copies a character archive with race codes renamed: WLD names, bitmap names
and texture files, with the directory checksums recomputed. Same-length codes,
so nothing in the WLD moves. On its own it only suits models with a full set of
animations: a renamed model that borrows animations borrows the wrong ones
under its new race number.
