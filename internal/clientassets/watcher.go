package clientassets

import (
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch imports anything copied into the drop folder (eg over SFTP) and
// removes it once imported; a file that cannot be imported is renamed to
// <name>.failed so it is not retried. Files already there are imported on
// start.
func (s *Store) Watch() {
	dir := s.DropDir()

	w, err := fsnotify.NewWatcher()
	if err != nil {
		s.logger.Error().Err(err).Msg("client assets: cannot watch the drop folder")
		return
	}
	if err := w.Add(dir); err != nil {
		s.logger.Error().Err(err).Any("dir", dir).Msg("client assets: cannot watch the drop folder")
		_ = w.Close()
		return
	}
	s.logger.Info().Any("dir", dir).Msg("client assets: watching the drop folder")

	s.importDropFolder()

	// a copy fires many write events; import once it has been quiet
	var timer *time.Timer
	for {
		select {
		case e, ok := <-w.Events:
			if !ok {
				return
			}
			if e.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) == 0 {
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(2*time.Second, s.importDropFolder)
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			s.logger.Error().Err(err).Msg("client assets: drop folder watcher")
		}
	}
}

func (s *Store) importDropFolder() {
	dir := s.DropDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) == ".failed" || e.Name()[0] == '.' {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		msg, err := s.Import(e.Name(), data)
		if err != nil {
			s.logger.Error().Err(err).Any("file", e.Name()).Msg("client assets: import failed")
			_ = os.Rename(path, path+".failed")
			continue
		}
		s.logger.Info().Any("file", e.Name()).Any("result", msg).Msg("client assets: imported")
		_ = os.Remove(path)
	}
}
