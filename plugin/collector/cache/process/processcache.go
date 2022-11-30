package process

import (
	"fmt"
	"strings"
	"sync"

	"k8s.io/utils/lru"
)

const MaxProcess = 4
const MaxArgv = 2048
const MaxPidCmdline = 4096

var PidCache = lru.New(MaxPidCmdline)
var ArgvCache = lru.New(MaxArgv)
var CmdlineCache = lru.New(MaxPidCmdline)

var Pool = NewPool()

type ProcessPool struct {
	p sync.Pool
}

func NewPool() *ProcessPool {
	return &ProcessPool{p: sync.Pool{
		New: func() interface{} {
			return &Process{}
		},
	}}
}

func (p *ProcessPool) Get() *Process {
	pr := p.p.Get().(*Process)
	pr.reset()
	return pr
}

func (p *ProcessPool) Put(pr *Process) {
	p.p.Put(pr)
}

func GetPidTree(pid int) (pidtree string) {
	var first = true
	for i := 0; i < MaxProcess; i++ {
		pidtree = fmt.Sprintf("%s%d.", pidtree, pid)
		if cmdline, ok := CmdlineCache.Get(pid); ok {
			pidtree = pidtree + cmdline.(string)
			goto PidLoop
		}
		// every event get one chance to flash the comm if a pid was found
		if first {
			first = false
			if comm, err := getComm(pid); err == nil {
				pidtree = pidtree + comm
				goto PidLoop
			}
		}
		break
	PidLoop:
		// break if the pid hits
		if pid == 0 || pid == 1 {
			break
		}
		if ppid, ok := PidCache.Get(pid); ok {
			pid = ppid.(int)
			pidtree = pidtree + "<"
		} else {
			break
		}
	}
	return strings.TrimRight(pidtree, "<")
}