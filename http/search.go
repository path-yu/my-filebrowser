package fbhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/filebrowser/filebrowser/v2/search"
)

const searchPingInterval = 5

var searchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	response := make(chan map[string]interface{})
	ctx, cancel := context.WithCancelCause(r.Context())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Avoid connection timeout
		timeout := time.NewTimer(searchPingInterval * time.Second)
		defer timeout.Stop()
		for {
			var err error
			var infoBytes []byte
			select {
			case info := <-response:
				if info == nil {
					return
				}
				infoBytes, err = json.Marshal(info)
			case <-timeout.C:
			// Send a heartbeat packet and re-arm the timer: without the
			// reset the one-shot timer fires a single time and long-running
			// searches stall without heartbeats until an intermediary proxy
			// kills the idle connection.
			infoBytes = nil
			timeout.Reset(searchPingInterval * time.Second)
			case <-ctx.Done():
				return
			}
			if err != nil {
				cancel(err)
				return
			}
			_, err = w.Write(infoBytes)
			if err == nil {
				_, err = w.Write([]byte("\n"))
			}
			if err != nil {
				cancel(err)
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}()
	query := r.URL.Query().Get("query")
	after, before, applyFilter := parseModifiedRange(r.URL.Query())

	// 产品编号检索：query 匹配编号前缀的 PDF 一并作为搜索结果返回（走数据库索引）。
	// 在文件系统扫描之前输出，保证按编号搜索时结果即时可见。
	if d.store.ProductCode != nil && query != "" {
		root := strings.TrimSuffix(r.URL.Path, "/")
		if entries, err := d.store.ProductCode.FindByCodePrefix(query); err == nil {
			for _, e := range entries {
				if root != "" && root != "/" && !strings.HasPrefix(e.Path, root+"/") {
					continue
				}
				if !d.Check(e.Path) {
					continue
				}
				info, err := d.user.Fs.Stat(e.Path)
				if err != nil || info.IsDir() {
					continue
				}
				if applyFilter && !inModifiedRange(info.IsDir(), info.ModTime(), after, before) {
					continue
				}
				// 文件名本身已包含关键词时跳过：文件系统扫描会命中它，避免重复结果
				if strings.Contains(strings.ToLower(info.Name()), strings.ToLower(query)) {
					continue
				}
				select {
				case <-ctx.Done():
				case response <- map[string]interface{}{
					"dir":      false,
					"path":     e.Path,
					"name":     info.Name(),
					"size":     info.Size(),
					"modified": info.ModTime().UTC().Format(time.RFC3339Nano),
				}:
				}
			}
		}
	}

	err := search.Search(ctx, d.user.Fs, r.URL.Path, query, d, func(path string, f os.FileInfo) error {
		if applyFilter && !inModifiedRange(f.IsDir(), f.ModTime(), after, before) {
			return nil
		}
		select {
		case <-ctx.Done():
		case response <- map[string]interface{}{
			"dir":      f.IsDir(),
			"path":     path,
			"name":     f.Name(),
			"size":     f.Size(),
			"modified": f.ModTime().UTC().Format(time.RFC3339Nano),
		}:
		}
		return context.Cause(ctx)
	})
	close(response)
	wg.Wait()
	if err == nil {
		err = context.Cause(ctx)
	}
	// ignore cancellation errors from user aborts
	if err != nil && !errors.Is(err, context.Canceled) {
		return http.StatusInternalServerError, err
	}

	return 0, nil
})
