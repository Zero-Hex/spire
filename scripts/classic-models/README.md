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
  wood elf male `ELM`, in `global_chr.s3d` and `global3_chr.s3d`. The
  borrowing rule is in Zero-Hex/Everquest_Model_Viewer's
  `CLASSIC_BORROW`. The 64 animations the skeleton lacks are copied in as
  `<anim>RON<bone>_TRACK`, each fitted to the skeleton like a borrowing
  client does: a bone's shift scaled by its rest length over the lender's, and
  its own scale kept. A bone an animation leaves alone is held at rest.
- Result: 71 animations of its own. In the model viewer it matches the stock
  skeleton pose for pose.

In game: upload it to Client Patch, add `3,0,TFFFC,starfall_skeleton_chr,
Loading Characters` to Resources/GlobalLoad.txt, and set NPCs to race 197
with size 4. The server's default height for race 197 is 6, which floats
them.

The archive is a full copy of `global_chr.s3d` (7.4 MB). Its other classic
models are identical to the ones `Global_chr` already loads.

## rename_models.py

Copies a character archive with race codes renamed: WLD names, bitmap names
and texture files, with the directory checksums recomputed. Same-length codes,
so nothing in the WLD moves. On its own it only suits models with a full set of
animations: a renamed model that borrows animations borrows the wrong ones
under its new race number.
