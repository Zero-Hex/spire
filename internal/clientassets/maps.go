package clientassets

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
)

// The icon and item model pickers list what Spire's static maps name. These
// rewrite the stock maps: icons are limited to what the RoF2 client has plus
// imported sheets, and imported models with a preview are added.

type staticMapDir struct {
	Type     string          `json:"type"`
	Name     string          `json:"name"`
	Contents []staticMapFile `json:"contents"`
}

type staticMapFile struct {
	Name string `json:"name"`
}

var (
	iconFileId   = regexp.MustCompile(`item_(\d+)\.png$`)
	mapCacheLock sync.Mutex
	mapCache     = map[string]struct {
		version int64
		data    []byte
	}{}
)

// ItemIconsMap is the stock item-icons map without the icons RoF2 does not
// have, plus every imported icon
func (s *Store) ItemIconsMap(stock []byte) []byte {
	return s.cachedMap("item-icons", stock, func(dir *staticMapDir) {
		custom := map[int]bool{}
		for _, id := range s.CustomIconIds() {
			custom[id] = true
		}

		kept := dir.Contents[:0]
		for _, f := range dir.Contents {
			m := iconFileId.FindStringSubmatch(f.Name)
			if m == nil {
				continue
			}
			var id int
			_, _ = fmt.Sscan(m[1], &id)
			if id <= Rof2LastStockIcon && !custom[id] {
				kept = append(kept, f)
			}
		}
		for id := range custom {
			kept = append(kept, staticMapFile{Name: fmt.Sprintf("./assets/item_icons/item_%d.png", id)})
		}
		dir.Contents = kept
	})
}

// ObjectsMap is the stock item model map plus every imported model that has
// a preview
func (s *Store) ObjectsMap(stock []byte) []byte {
	return s.cachedMap("objects", stock, func(dir *staticMapDir) {
		have := map[string]bool{}
		for _, f := range dir.Contents {
			have[f.Name] = true
		}
		for _, id := range s.PreviewModelIds() {
			name := fmt.Sprintf("./assets/objects/CTN_%d.png", id)
			if !have[name] {
				dir.Contents = append(dir.Contents, staticMapFile{Name: name})
			}
		}
	})
}

func (s *Store) cachedMap(key string, stock []byte, edit func(dir *staticMapDir)) []byte {
	version := s.Index().Version

	mapCacheLock.Lock()
	defer mapCacheLock.Unlock()
	if c, ok := mapCache[key]; ok && c.version == version {
		return c.data
	}

	var dirs []staticMapDir
	if err := json.Unmarshal(stock, &dirs); err != nil || len(dirs) == 0 {
		return stock
	}
	edit(&dirs[0])
	out, err := json.Marshal(dirs)
	if err != nil {
		return stock
	}

	mapCache[key] = struct {
		version int64
		data    []byte
	}{version, out}
	return out
}
