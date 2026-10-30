"""Builds rof2-race-archives.json from a RoF2 client folder: which archives
hold each race model code, what each race archive defines and animates, and
every archive the client has. See README.md."""
import glob, json, os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from pfs import contents

client, mapfile, out = sys.argv[1], sys.argv[2], sys.argv[3]
races = json.load(open(mapfile))["races"]

# codes Spire's race list or the client's _chr.txt files know
known = set()
for r in races:
    for c in (r["male_gender_model_code"], r["female_gender_model_code"], r["neutral_gender_model_code"]):
        if c:
            known.add(c.upper())
    for s in r["sources"]:
        for m in s.get("models") or []:
            known.add(m["code"].upper())

# archives the client's own race files name, whatever they are called
named = set()
for f in glob.glob(os.path.join(client, "*_chr.txt")):
    for l in open(f, errors="replace").read().splitlines()[1:]:
        if "," in l:
            code, archive = [p.strip() for p in l.split(",")[:2]]
            known.add(code.upper())
            named.add(archive.lower())
for l in open(os.path.join(client, "Resources", "GlobalLoad.txt"), errors="replace").read().splitlines():
    cols = l.split(",")
    if len(cols) >= 4:
        named.add(os.path.splitext(cols[3].strip().lower())[0] if cols[3].lower().endswith(".eqg") else cols[3].strip().lower())

all_files, codes, archives, failed = [], {}, {}, []
for name in sorted(os.listdir(client), key=str.lower):
    low = name.lower()
    if not low.endswith((".s3d", ".eqg")):
        continue
    all_files.append(low)
    stem = os.path.splitext(low)[0]
    race_like = (low.endswith(".s3d") and "chr" in low) or low.endswith(".eqg") or stem in named
    if not race_like or (low.startswith("it") and low[2:3].isdigit()):
        continue  # zones, and item models
    try:
        models, animations = contents(os.path.join(client, name), known)
    except Exception as e:
        failed.append((name, str(e)[:60]))
        continue
    models = {c for c in models if c in known or "chr" in low}
    animations = {c for c in animations if c in known}
    if low.endswith(".eqg") and not models:
        continue  # zone and object .eqg
    for c in models:
        codes.setdefault(c, []).append(low)
    archives[low] = {"models": sorted(models), "animations": sorted(animations)}

json.dump({
    "client": "RoF2",
    "codes": {c: sorted(v) for c, v in sorted(codes.items())},
    "archives": archives,
    "files": all_files,
}, open(out, "w"), separators=(",", ":"))
print("archives", len(all_files), "race archives", len(archives), "codes", len(codes), "failed", failed[:3])
