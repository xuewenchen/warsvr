package conf

import (
	"github.com/aceld/zinx/zlog"
	"github.com/fsnotify/fsnotify"
)

// OnConfigChange is called when the config file changes and is successfully reloaded.
type OnConfigChange func(cfg *Config)

// Watch starts watching the config file for changes and calls onReload after reloading.
// Returns the watcher so the caller can close it on shutdown.
func Watch(path string, onReload OnConfigChange) (*fsnotify.Watcher, error) {
	w, err := fsnotify.NewBufferedWatcher(0)
	if err != nil {
		return nil, err
	}
	if err := w.Add(path); err != nil {
		w.Close()
		return nil, err
	}

	go func() {
		defer w.Close()
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if ev.Op&(fsnotify.Write|fsnotify.Create) == 0 {
					continue
				}
				if err := Load(path); err != nil {
					zlog.Ins().ErrorF("config-watch: reload failed: %v", err)
					continue
				}
				zlog.Ins().InfoF("config-watch: %s reloaded", path)
				onReload(GlobalConfig)
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				zlog.Ins().ErrorF("config-watch: %v", err)
			}
		}
	}()

	return w, nil
}
