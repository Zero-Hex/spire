"""Copies an EverQuest .s3d character archive with race models renamed to
other race codes, so an older model can be given a race number of its own
(the classic skeleton SKE as race 484 SKL, say) while the newer one keeps its
number.

Every name carrying the code is renamed: the WLD's strings (meshes, materials,
actor and animation tracks: SKEHE0001, D05SKE_TRACK, T05BSKE..._TRACK), the
texture file names its bitmaps list, and the archive's own files (ske*.bmp).
Codes are the same length, so no WLD data moves. The archive is written
again with the files in their order, the WLD named after the new archive, and
the directory's name checksums recomputed.

    python3 -I rename_models.py global_chr.s3d starfall_classic_chr.s3d SKE=SKL WOE=SPW
"""
import os
import re
import struct
import sys
import zlib

KEY = bytes([0x95, 0x3A, 0xC5, 0x2A, 0x95, 0x7A, 0x95, 0x6A])
NAMES_CRC = 0x61580AC9  # the directory entry of the file name table
BLOCK = 8192


def xor(b):
    return bytes(x ^ KEY[i % 8] for i, x in enumerate(b))


def crc(name):
    """the checksum the directory keys a file by: CRC-32 (not reflected) of
    its lower-case name with the terminating null"""
    c = 0
    for x in name.lower().encode("latin-1") + b"\0":
        c ^= x << 24
        for _ in range(8):
            c = ((c << 1) ^ 0x04C11DB7) if c & 0x80000000 else (c << 1)
            c &= 0xFFFFFFFF
    return c


def read_pfs(data):
    dir_off = struct.unpack_from("<I", data, 0)[0]
    n = struct.unpack_from("<I", data, dir_off)[0]
    entries = [struct.unpack_from("<III", data, dir_off + 4 + i * 12) for i in range(n)]

    def unpack(off, size):
        out = bytearray()
        while len(out) < size:
            deflated, _ = struct.unpack_from("<II", data, off)
            out += zlib.decompress(data[off + 8: off + 8 + deflated])
            off += 8 + deflated
        return bytes(out)

    table_entry = [e for e in entries if e[0] == NAMES_CRC][0]
    table = unpack(table_entry[1], table_entry[2])
    count, p, names = struct.unpack_from("<I", table, 0)[0], 4, []
    for _ in range(count):
        length = struct.unpack_from("<I", table, p)[0]
        names.append(table[p + 4: p + 4 + length - 1].decode("latin-1"))
        p += 4 + length
    files = sorted((e for e in entries if e[0] != NAMES_CRC), key=lambda e: e[1])
    if len(files) != len(names):
        raise SystemExit("the name table does not match the directory")
    return [(name, unpack(e[1], e[2])) for name, e in zip(names, files)]


def pack(data):
    out = bytearray()
    for i in range(0, max(len(data), 1), BLOCK):
        chunk = data[i: i + BLOCK]
        z = zlib.compress(chunk, 9)
        out += struct.pack("<II", len(z), len(chunk)) + z
    return bytes(out)


def write_pfs(files):
    body = bytearray(b"\0" * 12)
    entries = []
    for name, data in files:
        entries.append((crc(name), len(body), len(data)))
        body += pack(data)
    table = bytearray(struct.pack("<I", len(files)))
    for name, _ in files:
        b = name.encode("latin-1") + b"\0"
        table += struct.pack("<I", len(b)) + b
    entries.append((NAMES_CRC, len(body), len(table)))
    body += pack(bytes(table))
    dir_off = len(body)
    body += struct.pack("<I", len(entries))
    for e in sorted(entries):
        body += struct.pack("<III", *e)
    struct.pack_into("<I4sI", body, 0, dir_off, b"PFS ", 0x20000)
    return bytes(body)


def renamer(codes):
    """renames a name's race code: at its start, or after an animation
    prefix of three or four characters (C05SKE..., T05BSKE...)"""
    pats = [(re.compile(r"^(%s)" % old, re.I), new) for old, new in codes.items()]
    pats += [(re.compile(r"^([A-Z]\d\d[A-Z]?)(%s)" % old, re.I), new) for old, new in codes.items()]

    def rename(s):
        for pat, new in pats:
            m = pat.match(s)
            if not m:
                continue
            at = m.start(m.lastindex)
            old = s[at: at + 3]
            new_cased = new.lower() if old.islower() else new.upper()
            return s[:at] + new_cased + s[at + 3:]
        return s
    return rename


def rename_wld(wld, rename, counts):
    magic, version, frags, regions, maxobj, hsize, unk = struct.unpack_from("<7I", wld, 0)
    if magic != 0x54503D02:
        raise SystemExit("not a WLD")
    out = bytearray(wld)
    # the string table
    strings = xor(wld[28: 28 + hsize]).split(b"\0")
    renamed = []
    for s in strings:
        t = rename(s.decode("latin-1")).encode("latin-1")
        counts["strings"] += t != s
        renamed.append(t)
    table = b"\0".join(renamed)
    assert len(table) == hsize
    out[28: 28 + hsize] = xor(table)
    # bitmap file names (fragment 0x03), stored in the fragment, encoded too
    p = 28 + hsize
    for _ in range(frags):
        size, ftype = struct.unpack_from("<II", wld, p)
        body = p + 8
        if ftype == 0x03:
            n = struct.unpack_from("<I", wld, body + 4)[0] + 1
            q = body + 8
            for _ in range(n):
                length = struct.unpack_from("<H", wld, q)[0]
                name = xor(wld[q + 2: q + 2 + length])
                text = name.rstrip(b"\0").decode("latin-1")
                new = rename(text).encode("latin-1")
                if new != text.encode("latin-1"):
                    counts["bitmaps"] += 1
                    out[q + 2: q + 2 + length] = xor(new + name[len(new):])
                q += 2 + length
        p = body + size
    return bytes(out)


def main():
    src, dst = sys.argv[1], sys.argv[2]
    codes = dict(a.upper().split("=") for a in sys.argv[3:])
    for old, new in codes.items():
        if len(old) != 3 or len(new) != 3:
            raise SystemExit("race codes are three characters")
    rename = renamer(codes)
    files = read_pfs(open(src, "rb").read())
    taken = {n.lower() for n, _ in files}
    for new in codes.values():
        if any(n.startswith(new.lower()) for n in taken):
            raise SystemExit("%s already has files named %s*: the code is taken" % (src, new.lower()))

    old_wld = os.path.splitext(os.path.basename(src))[0].lower() + ".wld"
    new_wld = os.path.splitext(os.path.basename(dst))[0].lower() + ".wld"
    counts = {"strings": 0, "bitmaps": 0, "files": 0}
    out = []
    for name, data in files:
        if name.lower() == old_wld:
            out.append((new_wld, rename_wld(data, rename, counts)))
            continue
        new = rename(name)
        counts["files"] += new != name
        out.append((new, data))
    open(dst, "wb").write(write_pfs(out))
    print("%s: %d files, renamed %d names, %d bitmap names, %d files" % (
        dst, len(out), counts["strings"], counts["bitmaps"], counts["files"]))


if __name__ == "__main__":
    main()
