"""Cuts a character archive down to one model: its actor, skeleton,
meshes, materials, textures and animation tracks, and the texture files those
name. Everything else in the WLD and the archive is dropped.

    python3 -I trim_model.py in.s3d out.s3d RON

Pieces are taken by name (all named <code>..., and the tracks
<anim><code>..._TRACK the client finds animations by) and by the references
they hold, followed through: actor 0x14 -> skeleton link 0x11 / mesh link
0x2D, skeleton 0x10 -> tracks 0x13 and meshes 0x2D, 0x13 -> 0x12, 0x2D -> mesh
0x36 -> material list 0x31 -> materials 0x30 -> 0x05 -> 0x04 -> bitmap names
0x03. Pieces keep their order and are numbered again. The string table is
kept whole, so names (negative references) need no change.
"""
import os
import re
import struct
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rename_models as rm


def parse(wld):
    hdr = list(struct.unpack_from("<7I", wld, 0))
    table = rm.xor(wld[28: 28 + hdr[5]])
    name = lambda ref: table[-ref: table.index(b"\0", -ref)].decode("latin-1") if ref < 0 else ""
    out, p = [None], 28 + hdr[5]
    for _ in range(hdr[2]):
        size, t = struct.unpack_from("<II", wld, p)
        out.append({"type": t, "o": p + 8, "size": size, "name": name(struct.unpack_from("<i", wld, p + 8)[0])})
        p += 8 + size
    return hdr, out, p


def ref_slots(wld, f):
    """the body offsets (from the fragment's start) holding fragment references"""
    o, t = f["o"], f["type"]
    u32 = lambda at: struct.unpack_from("<I", wld, at)[0]
    if t in (0x11, 0x2D, 0x05, 0x13):
        return [4]
    if t == 0x36:
        return [8, 12]
    if t == 0x30:
        return [24]
    if t == 0x31:
        return [12 + 4 * k for k in range(u32(o + 8))]
    if t == 0x04:
        fl, n, q = u32(o + 4), u32(o + 8), 12
        if fl & 4:
            q += 4
        if fl & 8:
            q += 4
        return [q + 4 * k for k in range(n)]
    if t == 0x10:
        fl, nb = u32(o + 4), u32(o + 8)
        slots, q = [12], 16
        if fl & 1:
            q += 12
        if fl & 2:
            q += 4
        for _ in range(nb):
            slots += [q + 8, q + 12]
            q += 20 + 4 * u32(o + q + 16)
        if fl & 0x200:
            n = u32(o + q)
            slots += [q + 4 + 4 * k for k in range(n)]
        return slots
    if t == 0x14:
        fl, n_act, n_ref = u32(o + 4), u32(o + 12), u32(o + 16)
        q = 24 + (4 if fl & 1 else 0) + (28 if fl & 2 else 0)
        for _ in range(n_act):
            q += 8 + 4 * u32(o + q)
        return [20] + [q + 4 * k for k in range(n_ref)]
    if t in (0x03, 0x12, 0x37):
        return []
    raise SystemExit("no reference layout for fragment type 0x%02X (%s)" % (t, f["name"]))


def trim(files, wld_name, code):
    wld = dict(files)[wld_name]
    hdr, fr, end = parse(wld)
    track = re.compile(r"^[A-Z]\d\d[A-Z]?%s[A-Z0-9_]*_TRACK(DEF)?$" % code)
    keep, todo = set(), [i for i, f in enumerate(fr) if f and (f["name"].startswith(code) or track.match(f["name"]))]
    while todo:
        i = todo.pop()
        if i in keep:
            continue
        keep.add(i)
        for s in ref_slots(wld, fr[i]):
            r = struct.unpack_from("<i", wld, fr[i]["o"] + s)[0]
            if 0 < r < len(fr) and r not in keep:
                todo.append(r)
    order = sorted(keep)
    new_index = {old: n + 1 for n, old in enumerate(order)}

    body = bytearray()
    for old in order:
        f = fr[old]
        data = bytearray(wld[f["o"]: f["o"] + f["size"]])
        for s in ref_slots(wld, f):
            r = struct.unpack_from("<i", data, s)[0]
            if r > 0:
                if r not in new_index:
                    raise SystemExit("%s refers to a piece not kept" % f["name"])
                struct.pack_into("<i", data, s, new_index[r])
        body += struct.pack("<II", f["size"], f["type"]) + data
    hdr[2], hdr[4] = len(order), max(fr[i]["size"] for i in order)
    out_wld = struct.pack("<7I", *hdr) + wld[28: 28 + hdr[5]] + bytes(body) + wld[end:]

    # the texture files the kept bitmap names name
    textures = set()
    for i in order:
        f = fr[i]
        if f["type"] == 0x03:
            n, q = struct.unpack_from("<I", wld, f["o"] + 4)[0] + 1, f["o"] + 8
            for _ in range(n):
                length = struct.unpack_from("<H", wld, q)[0]
                textures.add(rm.xor(wld[q + 2: q + 2 + length]).rstrip(b"\0").decode("latin-1").lower())
                q += 2 + length
    names = {n.lower() for n, _ in files}
    missing = sorted(textures - names)
    if missing:
        raise SystemExit("textures named but not in the archive: %s" % ", ".join(missing[:5]))
    kept = [(n, out_wld if n == wld_name else d) for n, d in files if n == wld_name or n.lower() in textures]
    return kept, len(order), len(fr) - 1


def main():
    src, dst, code = sys.argv[1], sys.argv[2], sys.argv[3].upper()
    files = rm.read_pfs(open(src, "rb").read())
    wld_name = next(n for n, _ in files if n.lower().endswith(".wld"))
    kept, n_kept, n_all = trim(files, wld_name, code)
    open(dst, "wb").write(rm.write_pfs(kept))
    print("%s: %d of %d pieces, %d of %d files, %.1f MB -> %.2f MB" % (
        dst, n_kept, n_all, len(kept), len(files), os.path.getsize(src) / 1e6, os.path.getsize(dst) / 1e6))


if __name__ == "__main__":
    main()
