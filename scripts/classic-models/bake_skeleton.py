"""Builds a classic-model archive whose model carries every animation itself.

Classic models without their own full set borrow the rest from another race
when the client shows them under their stock race number (the classic
skeleton SKE borrows from the wood elf male ELM, in global_chr.s3d and
global3_chr.s3d). Under another race number the client borrows wrongly, so
here the borrowed animations are copied into the archive under the new code,
each fitted to this model's bones as a borrowing client does: a bone's shift
scaled by its rest length over the lender's, and the bone's own scale kept.

    python3 -I bake_skeleton.py <client> out.s3d SKE RON ELM

The archive is <client>/global_chr.s3d with SKE renamed RON (rename_models)
and the animations added to its WLD as 0x12 tracks and 0x13 track links named
<anim><variant>RON<bone>_TRACK, the way the client finds them.
"""
import math
import os
import struct
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rename_models as rm


def frags(wld):
    hdr = list(struct.unpack_from("<7I", wld, 0))
    table = rm.xor(wld[28: 28 + hdr[5]])
    name = lambda ref: table[-ref: table.index(b"\0", -ref)].decode("latin-1") if ref < 0 else ""
    out, p = [None], 28 + hdr[5]
    for _ in range(hdr[2]):
        size, t = struct.unpack_from("<II", wld, p)
        body = p + 8
        out.append({"type": t, "o": body, "size": size, "name": name(struct.unpack_from("<i", wld, body)[0])})
        p = body + size
    return hdr, table, out, p


def tracks(wld, fr, code):
    """0x13 links of a code: {(anim key, bone suffix): (0x13, 0x12)}; the rest pose keyed ("", suffix)"""
    out = {}
    for f in fr[1:]:
        if f["type"] != 0x13 or not f["name"].endswith("_TRACK"):
            continue
        n = f["name"][:-6]
        if n.startswith(code):
            key, suffix = "", n[len(code):]
        elif len(n) > 3 and n[1:3].isdigit() and n[3:].startswith(code):
            key, suffix = n[:3], n[3 + len(code):]
        elif len(n) > 4 and n[1:3].isdigit() and n[4:].startswith(code):
            key, suffix = n[:4], n[4 + len(code):]
        else:
            continue
        ref = struct.unpack_from("<i", wld, f["o"] + 4)[0]
        out[(key, suffix)] = (f, fr[ref])
    return out


def read_frames(wld, t12):
    n = struct.unpack_from("<I", wld, t12["o"] + 8)[0]
    return [list(struct.unpack_from("<8h", wld, t12["o"] + 12 + i * 16)) for i in range(n)]


def length(frame):
    return math.sqrt(sum((v / 256.0) ** 2 for v in frame[4:7]))


def clamp16(v):
    return max(-32768, min(32767, int(round(v))))


def main():
    client, out_path, code, new_code, lender = sys.argv[1:6]
    src = os.path.join(client, "global_chr.s3d")
    files = rm.read_pfs(open(src, "rb").read())
    wld = dict(files)["global_chr.wld"]
    hdr, table, fr, end = frags(wld)

    own = tracks(wld, fr, code)
    lend = tracks(wld, fr, lender)
    # more of the lender's animations
    extra_files = dict(rm.read_pfs(open(os.path.join(client, "global3_chr.s3d"), "rb").read()))
    w3 = extra_files["global3_chr.wld"]
    _, _, fr3, _ = frags(w3)
    lend3 = tracks(w3, fr3, lender)

    bones = sorted(s for (k, s) in own if k == "")
    own_rest = {s: read_frames(wld, own[("", s)][1])[0] for s in bones}
    lend_rest = {s: read_frames(wld, lend[("", s)][1])[0] for (k, s) in lend if k == "" and s in own_rest}
    own_keys = {k for (k, s) in own if k}
    lent = {}
    for src_w, tr in ((wld, lend), (w3, lend3)):
        for (k, s), (t13, t12) in tr.items():
            if k and k not in own_keys and s in own_rest and (k, s) not in lent:
                lent[(k, s)] = (src_w, t13, t12)
    keys = sorted({k for (k, s) in lent})

    # the new names and fragments, after the renamed WLD's own
    rename = rm.renamer({code: new_code})
    counts = {"strings": 0, "bitmaps": 0, "files": 0}
    renamed = rm.rename_wld(wld, rename, counts)
    table = rm.xor(renamed[28: 28 + hdr[5]])
    new_strings, new_frags = bytearray(), []
    next_index = hdr[2] + 1

    def add_name(s):
        off = len(table) + len(new_strings)
        new_strings.extend(s.encode("latin-1") + b"\0")
        return -off

    made = 0
    for k in keys:
        # a link's flags and frame time from any lent bone of this animation
        some = next(v for (kk, s), v in lent.items() if kk == k)
        link_flags = struct.unpack_from("<I", some[0], some[1]["o"] + 8)[0]
        sleep = struct.unpack_from("<I", some[0], some[1]["o"] + 12)[0] if link_flags & 1 else None
        for s in bones:
            if (k, s) in lent:
                src_w, t13, t12 = lent[(k, s)]
                frames = read_frames(src_w, t12)
                l_own, l_lend = length(own_rest[s]), length(lend_rest.get(s, own_rest[s]))
                for f in frames:
                    if l_lend > 1e-6:
                        ratio = l_own / l_lend
                        f[4:7] = [clamp16(v * ratio) for v in f[4:7]]
                    else:
                        f[4:7] = own_rest[s][4:7]
                    f[7] = own_rest[s][7]
                flags13 = struct.unpack_from("<I", src_w, t13["o"] + 8)[0]
                sleep13 = struct.unpack_from("<I", src_w, t13["o"] + 12)[0] if flags13 & 1 else None
            else:
                # a bone the lender's animation leaves alone: held at rest
                frames, flags13, sleep13 = [own_rest[s]], link_flags, sleep
            base = k[:3] + k[3:] + new_code + s
            def_body = struct.pack("<iII", add_name(base + "_TRACKDEF"), 8, len(frames)) + b"".join(struct.pack("<8h", *f) for f in frames)
            new_frags.append(struct.pack("<II", len(def_body), 0x12) + def_body)
            link = struct.pack("<iiI", add_name(base + "_TRACK"), next_index, flags13)
            if flags13 & 1:
                link += struct.pack("<I", sleep13)
            new_frags.append(struct.pack("<II", len(link), 0x13) + link)
            next_index += 2
            made += 1

    table = bytes(table) + bytes(new_strings)
    hdr2 = list(struct.unpack_from("<7I", renamed, 0))
    hdr2[2] += len(new_frags)
    hdr2[4] = max(hdr2[4], *(len(f) - 8 for f in new_frags))
    hdr2[5] = len(table)
    hdr2[6] += 2 * made
    body_start = 28 + hdr[5]
    out_wld = struct.pack("<7I", *hdr2) + rm.xor(table) + renamed[body_start:end] + b"".join(new_frags) + renamed[end:]

    out = []
    new_wld = os.path.splitext(os.path.basename(out_path))[0].lower() + ".wld"
    for name, data in files:
        if name.lower() == "global_chr.wld":
            out.append((new_wld, out_wld))
        else:
            out.append((rename(name), data))
    open(out_path, "wb").write(rm.write_pfs(out))
    print("%s: %d animations borrowed from %s (%s), %d tracks added, %d bones, renamed %d names" % (
        out_path, len(keys), lender, ", ".join(keys[:6]) + "...", made, len(bones), counts["strings"]))


if __name__ == "__main__":
    main()
