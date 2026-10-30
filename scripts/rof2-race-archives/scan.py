import json, os, sys, glob
sys.path.insert(0, os.path.dirname(__file__))
from pfs import codes
client, mapfile, out = sys.argv[1], sys.argv[2], sys.argv[3]
races = json.load(open(mapfile))["races"]
known = set()
for r in races:
    for c in (r["male_gender_model_code"], r["female_gender_model_code"], r["neutral_gender_model_code"]):
        if c: known.add(c.upper())
    for s in r["sources"]:
        for m in s.get("models") or []: known.add(m["code"].upper())
for f in glob.glob(os.path.join(client, "*_chr.txt")):
    for l in open(f, errors="replace").read().splitlines()[1:]:
        if "," in l: known.add(l.split(",")[0].strip().upper())
index, scanned, failed = {}, 0, []
for name in sorted(os.listdir(client)):
    low = name.lower()
    if not (low.endswith(".s3d") and "chr" in low) and not low.endswith(".eqg"):
        continue
    if low.startswith("it") and low[2:3].isdigit():
        continue  # item models
    try:
        found, _ = codes(os.path.join(client, name))
    except Exception as e:
        failed.append((name, str(e)[:60])); continue
    scanned += 1
    for c in found & known:
        index.setdefault(c, []).append(name)
json.dump({"client": "RoF2", "codes": {c: sorted(v, key=str.lower) for c, v in sorted(index.items())}}, open(out, "w"), indent=1)
print("scanned", scanned, "codes", len(index), "known codes", len(known), "failed", len(failed), failed[:3])
