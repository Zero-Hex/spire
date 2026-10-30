import mmap, re, struct, sys, zlib

def pfs_files(data):
    dir_off = struct.unpack_from("<I", data, 0)[0]
    count = struct.unpack_from("<I", data, dir_off)[0]
    entries = []
    for i in range(count):
        crc, off, size = struct.unpack_from("<III", data, dir_off + 4 + i * 12)
        entries.append((crc, off, size))
    def unpack(off, size):
        out = bytearray()
        while len(out) < size:
            deflen, inflen = struct.unpack_from("<II", data, off)
            out += zlib.decompress(data[off + 8: off + 8 + deflen])
            off += 8 + deflen
        return bytes(out)
    names_entry = [e for e in entries if e[0] == 0x61580AC9]
    rest = sorted([e for e in entries if e[0] != 0x61580AC9], key=lambda e: e[1])
    table = unpack(names_entry[0][1], names_entry[0][2])
    n = struct.unpack_from("<I", table, 0)[0]
    p, names = 4, []
    for _ in range(n):
        l = struct.unpack_from("<I", table, p)[0]; p += 4
        names.append(table[p:p + l - 1].decode("latin-1")); p += l
    return {names[i].lower(): (lambda e=rest[i]: unpack(e[1], e[2])) for i in range(min(len(names), len(rest)))}

KEY = bytes([0x95, 0x3A, 0xC5, 0x2A, 0x95, 0x7A, 0x95, 0x6A])

def wld_strings(wld):
    magic, version, frags, regions, maxobj, hsize, unk = struct.unpack_from("<7I", wld, 0)
    if magic != 0x54503D02:
        return []
    raw = bytes(b ^ KEY[i % 8] for i, b in enumerate(wld[28:28 + hsize]))
    return [s.decode("latin-1") for s in raw.split(b"\0") if s]

ACTOR = re.compile(r"^([A-Z0-9]{3})_ACTORDEF$")
# an animation track: <animation><race code><bone>_TRACK, where the
# animation is C05 or T05B: C05SKEHEAD_TRACK, T05BSKEPEBIP01_TRACK
TRACK = re.compile(r"^[A-Z]\d\d[A-Z0-9_]*_TRACK$")
ITEM = re.compile(r"^IT\d")


def track_codes(name):
    """the race codes an animation track could be for: after a three or a
    four letter animation; the caller keeps the one it knows"""
    if not TRACK.match(name):
        return []
    out = [name[3:6]]
    if name[3].isalpha():
        out.append(name[4:7])
    return out


def contents(path, known=frozenset()):
    """the race model codes an archive defines, and the known codes it
    animates (a track's three-letter reading first)"""
    with open(path, "rb") as fh:
        data = mmap.mmap(fh.fileno(), 0, access=mmap.ACCESS_READ)
        files = pfs_files(data)
        models, animations = set(), set()
        if path.lower().endswith(".s3d"):
            for name, read in files.items():
                if name.endswith(".wld"):
                    for s in wld_strings(read()):
                        u = s.upper()
                        m = ACTOR.match(u)
                        if m and not ITEM.match(m.group(1)):
                            models.add(m.group(1))
                        for c in track_codes(u):
                            if c in known:
                                animations.add(c)
                                break
        else:
            for name in files:
                m = re.match(r"^([a-z0-9]{3})\.(mod|mds)$", name)
                if m and not ITEM.match(m.group(1).upper()):
                    models.add(m.group(1).upper())
        return models, animations - models


def codes(path):
    models, _ = contents(path)
    return models, []


if __name__ == "__main__":
    for p in sys.argv[1:]:
        c, names = codes(p)
        print(p.split("/")[-1], sorted(c)[:20], len(c), names[:6])
