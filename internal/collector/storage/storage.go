package storage

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type Filesystem struct {
	Mount     string  `json:"mount"`
	FSType    string  `json:"fstype"`
	Total     int64   `json:"total"`
	Used      int64   `json:"used"`
	Available int64   `json:"available"`
	Percent   float64 `json:"percent"`
	Mounted   bool    `json:"mounted"`
}

type StatfsFunc func(path string) (total, used, available int64, err error)

type Collector struct {
	root    string
	ignored map[string]bool
	statfs  StatfsFunc
}

func New(root string, ignored []string, statfs StatfsFunc) *Collector {
	if statfs == nil {
		statfs = unixStatfs
	}
	ig := map[string]bool{}
	for _, f := range ignored {
		ig[f] = true
	}
	return &Collector{root: root, ignored: ig, statfs: statfs}
}

func unixStatfs(path string) (int64, int64, int64, error) {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(path, &buf); err != nil {
		return 0, 0, 0, err
	}
	bsize := int64(buf.Bsize)
	if bsize <= 0 {
		bsize = int64(buf.Frsize)
	}
	if bsize <= 0 {
		return 0, 0, 0, os.ErrInvalid
	}
	total := int64(buf.Blocks) * bsize
	used := int64(buf.Blocks-buf.Bfree) * bsize
	available := int64(buf.Bavail) * bsize
	return total, used, available, nil
}

func parseMounts(raw string) (mounts, types []string) {
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mounts = append(mounts, unescape(fields[1]))
		types = append(types, fields[2])
	}
	return
}

func unescape(p string) string {
	if !strings.Contains(p, `\`) {
		return p
	}
	p = strings.ReplaceAll(p, `\040`, " ")
	p = strings.ReplaceAll(p, `\011`, "\t")
	p = strings.ReplaceAll(p, `\012`, "\n")
	p = strings.ReplaceAll(p, `\134`, `\`)
	return p
}

func percent(used, total int64) float64 {
	if total <= 0 {
		return 0
	}
	p := 100 * float64(used) / float64(total)
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return p
}

func (c *Collector) Collect() []Filesystem {
	out := []Filesystem{}
	raw, err := os.ReadFile(filepath.Join(c.root, "proc", "mounts"))
	if err != nil {
		return out
	}
	seen := map[string]bool{}
	mounts, types := parseMounts(string(raw))
	for i, mount := range mounts {
		if mount == "" || c.ignored[types[i]] || seen[mount] {
			continue
		}
		seen[mount] = true
		total, used, available, err := c.statfs(filepath.Join(c.root, mount))
		if err != nil {
			continue
		}
		out = append(out, Filesystem{
			Mount:     mount,
			FSType:    types[i],
			Total:     total,
			Used:      used,
			Available: available,
			Percent:   percent(used, total),
			Mounted:   true,
		})
	}
	return out
}
