package cpu

import (
	"bufio"
	"bytes"
	"github.com/arch3754/mrpc/log"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	historyCount int = 2
)

var (
	procStatHistory [historyCount]*ProcStat
	psLock          = new(sync.RWMutex)
)

func init() {
	go func() {
		if err := UpdateCpuStat(); err != nil {
			log.Rlog.Error("update cpustat failed:%v", err)
		}
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			if err := UpdateCpuStat(); err != nil {
				log.Rlog.Error("update cpustat failed:%v", err)
			}
		}
	}()
}
func UpdateCpuStat() error {
	ps, err := CurrentProcStat()
	if err != nil {
		return err
	}

	psLock.Lock()
	defer psLock.Unlock()
	for i := historyCount - 1; i > 0; i-- {
		procStatHistory[i] = procStatHistory[i-1]
	}

	procStatHistory[0] = ps
	return nil
}

func deltaTotal() uint64 {
	if procStatHistory[1] == nil {
		return 0
	}
	return procStatHistory[0].Cpu.Total - procStatHistory[1].Cpu.Total
}

func CpuIdle() float64 {
	psLock.RLock()
	defer psLock.RUnlock()
	dt := deltaTotal()
	if dt == 0 {
		return 0.0
	}
	invQuotient := 100.00 / float64(dt)
	return float64(procStatHistory[0].Cpu.Idle-procStatHistory[1].Cpu.Idle) * invQuotient
}

type CpuUsage struct {
	User    uint64 // time spent in user mode
	Nice    uint64 // time spent in user mode with low priority (nice)
	System  uint64 // time spent in system mode
	Idle    uint64 // time spent in the idle task
	Iowait  uint64 // time spent waiting for I/O to complete (since Linux 2.5.41)
	Irq     uint64 // time spent servicing  interrupts  (since  2.6.0-test4)
	SoftIrq uint64 // time spent servicing softirqs (since 2.6.0-test4)
	Steal   uint64 // time spent in other OSes when running in a virtualized environment (since 2.6.11)
	Guest   uint64 // time spent running a virtual CPU for guest operating systems under the control of the Linux kernel. (since 2.6.24)
	Total   uint64 // total of all time fields
}
type ProcStat struct {
	Cpu          *CpuUsage
	Cpus         []*CpuUsage
	Ctxt         uint64
	Processes    uint64
	ProcsRunning uint64
	ProcsBlocked uint64
}

func CurrentProcStat() (*ProcStat, error) {
	f := Root() + "/proc/stat"
	bs, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, err
	}

	ps := &ProcStat{Cpus: make([]*CpuUsage, NumCpu())}
	reader := bufio.NewReader(bytes.NewBuffer(bs))

	for {
		line, err := ReadLine(reader)
		if err == io.EOF {
			err = nil
			break
		} else if err != nil {
			return ps, err
		}
		parseLine(line, ps)
	}

	return ps, nil
}

const nuxRootFs = "NUX_ROOTFS"

// Root 获取系统变量
func Root() string {
	root := os.Getenv(nuxRootFs)
	if !strings.HasPrefix(root, string(os.PathSeparator)) {
		return ""
	}
	root = strings.TrimSuffix(root, string(os.PathSeparator))
	if pathExists(root) {
		return root
	}
	return ""
}
func NumCpu() int {
	return runtime.NumCPU()
}

func parseLine(line []byte, ps *ProcStat) {
	fields := strings.Fields(string(line))
	if len(fields) < 2 {
		return
	}

	fieldName := fields[0]
	if fieldName == "cpu" {
		ps.Cpu = parseCpuFields(fields)
		return
	}

	if strings.HasPrefix(fieldName, "cpu") {
		idx, err := strconv.Atoi(fieldName[3:])
		if err != nil || idx >= len(ps.Cpus) {
			return
		}

		ps.Cpus[idx] = parseCpuFields(fields)
		return
	}

	if fieldName == "ctxt" {
		ps.Ctxt, _ = strconv.ParseUint(fields[1], 10, 64)
		return
	}

	if fieldName == "processes" {
		ps.Processes, _ = strconv.ParseUint(fields[1], 10, 64)
		return
	}

	if fieldName == "procs_running" {
		ps.ProcsRunning, _ = strconv.ParseUint(fields[1], 10, 64)
		return
	}

	if fieldName == "procs_blocked" {
		ps.ProcsBlocked, _ = strconv.ParseUint(fields[1], 10, 64)
		return
	}
}

func parseCpuFields(fields []string) *CpuUsage {
	cu := new(CpuUsage)
	sz := len(fields)
	for i := 1; i < sz; i++ {
		val, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			continue
		}

		// user time is already include guest time
		if i != 9 {
			cu.Total += val
		}
		switch i {
		case 1:
			cu.User = val
		case 2:
			cu.Nice = val
		case 3:
			cu.System = val
		case 4:
			cu.Idle = val
		case 5:
			cu.Iowait = val
		case 6:
			cu.Irq = val
		case 7:
			cu.SoftIrq = val
		case 8:
			cu.Steal = val
		case 9:
			cu.Guest = val
		}
	}
	return cu
}

func pathExists(path string) bool {
	fi, err := os.Stat(path)
	if err == nil {
		return fi.IsDir()
	}
	return false
}

func ReadLine(r *bufio.Reader) ([]byte, error) {
	line, isPrefix, err := r.ReadLine()
	for isPrefix && err == nil {
		var bs []byte
		bs, isPrefix, err = r.ReadLine()
		line = append(line, bs...)
	}

	return line, err
}