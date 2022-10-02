package event

import (
	"bufio"
	"errors"
	"hades-ebpf/user/decoder"
	"io"
	"os"
	"time"

	manager "github.com/ehids/ebpfmanager"
)

const maxModule = 256

type ModuleScan struct {
	decoder.BasicEvent `json:"-"`
	IterCount          uint32 `json:"iter_count"`
	KernelCount        uint32 `json:"kernel_count"`
	UserCount          uint32 `json:"user_count"`
}

func (ModuleScan) ID() uint32 {
	return 1203
}

// In DecodeEvent, get the count of /proc/modules, and we do compare them
func (m *ModuleScan) DecodeEvent(e *decoder.EbpfDecoder) (err error) {
	m.UserCount = 0
	var index uint8
	var file *os.File
	if err = e.DecodeUint8(&index); err != nil {
		return
	}
	if err = e.DecodeUint32(&m.IterCount); err != nil {
		return
	}
	if err = e.DecodeUint8(&index); err != nil {
		return
	}
	if err = e.DecodeUint32(&m.KernelCount); err != nil {
		return
	}
	if file, err = os.Open("/proc/modules"); err != nil {
		return
	}
	defer file.Close()
	s := bufio.NewScanner(io.LimitReader(file, 1024*1024))
	for s.Scan() {
		m.UserCount += 1
		if m.UserCount >= maxModule {
			break
		}
	}
	// If kernel space and userspace get same count
	// them no lefted
	if m.UserCount == m.KernelCount {
		err = ErrIgnore
	}
	return nil
}

func (ModuleScan) Name() string {
	return "anti_rkt_mod_scan"
}

func (i *ModuleScan) Trigger(m *manager.Manager) error {
	idt := kernelSymbols.Get("module_kset")
	if idt == nil {
		err := errors.New("mod_kset is not found")
		return err
	}
	// Only trigger the 0x80 here
	i.trigger(idt.Address)
	return nil
}

//go:noinline
func (m *ModuleScan) trigger(mod_kset uint64) error {
	return nil
}

func (m *ModuleScan) RegistCron() (decoder.EventCronFunc, *time.Ticker) {
	ticker := time.NewTicker(10 * time.Minute)
	return m.Trigger, ticker
}

func (ModuleScan) GetProbes() []*manager.Probe {
	return []*manager.Probe{
		{
			UID:              "ModuleScan",
			Section:          "uprobe/trigger_module_scan",
			EbpfFuncName:     "trigger_module_scan",
			AttachToFuncName: "hades-ebpf/user/event.(*ModuleScan).trigger",
			BinaryPath:       "/proc/self/exe",
		},
	}
}

func init() {
	decoder.RegistEvent(&ModuleScan{})
}