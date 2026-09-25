package monitor

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Measurement — одно измерение машины (specs/016-dashboard.md,
// требование 11). Процессор и память читаются из /proc, поэтому не на
// Linux там нули; диск меряется везде.
type Measurement struct {
	CPUPercent       float64
	MemoryUsedBytes  int64
	MemoryTotalBytes int64
	SwapUsedBytes    int64
	DiskUsedBytes    int64
	DiskTotalBytes   int64
}

// cpuTimes — счётчики первой строки /proc/stat. Загрузка процессора —
// доля занятого времени между двумя такими снимками.
type cpuTimes struct {
	idle, total uint64
}

func readCPU() (cpuTimes, bool) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuTimes{}, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return cpuTimes{}, false
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTimes{}, false
	}
	var t cpuTimes
	for i, v := range fields[1:] {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return cpuTimes{}, false
		}
		t.total += n
		// idle и iowait: процессор ничего не считал.
		if i == 3 || i == 4 {
			t.idle += n
		}
	}
	return t, true
}

func cpuPercent(prev, cur cpuTimes) float64 {
	if cur.total <= prev.total {
		return 0
	}
	busy := float64((cur.total-prev.total)-(cur.idle-prev.idle)) / float64(cur.total-prev.total)
	return busy * 100
}

// readMemory возвращает занятую и всю память и занятую подкачку в байтах.
// Занятая — это MemTotal − MemAvailable: кэш файлов, который ядро отдаст
// по первому требованию, занятым не считается.
func readMemory() (used, total, swapUsed int64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, 0
	}
	defer f.Close()
	values := map[string]int64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[strings.TrimSuffix(fields[0], ":")] = n * 1024
	}
	total = values["MemTotal"]
	used = total - values["MemAvailable"]
	swapUsed = values["SwapTotal"] - values["SwapFree"]
	return used, total, swapUsed
}

// DiskUsage — занятое и всё место файловой системы, где лежит path.
// Занятым считается всё, что недоступно обычному пользователю: так
// «свободно» совпадает с тем, что покажет df. Папки ещё может не быть
// (файлов пока никто не загрузил) — тогда меряется ближайшая существующая
// родительская.
func DiskUsage(path string) (used, total int64, err error) {
	var st syscall.Statfs_t
	for {
		err = syscall.Statfs(path, &st)
		if err == nil || !errors.Is(err, fs.ErrNotExist) || filepath.Dir(path) == path {
			break
		}
		path = filepath.Dir(path)
	}
	if err != nil {
		return 0, 0, err
	}
	bsize := int64(st.Bsize)
	total = int64(st.Blocks) * bsize
	used = total - int64(st.Bavail)*bsize
	return used, total, nil
}
