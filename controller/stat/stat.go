package stat

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/yzp0n/ncdn/controller/protocol"
)

func GetMachineData() (*protocol.MachineData, error) {
	cpuCount, err := cpu.Counts(true)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU count: %w", err)
	}
	memoryTotal, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}
	diskUsage, err := disk.Usage("/")
	if err != nil {
		return nil, fmt.Errorf("failed to get disk usage: %w", err)
	}
	return &protocol.MachineData{
		CPUCount:    uint8(cpuCount),
		MemoryTotal: uint64(memoryTotal.Total),
		DiskTotal:   uint64(diskUsage.Total),
	}, nil
}

var startTime time.Time

func init() {
	// 初期化時に現在の時刻を記録
	startTime = time.Now()
}

func GetMachineStat() (*protocol.MachineStat, error) {
	stats := &protocol.MachineStat{}

	stats.Uptime = time.Since(startTime) / time.Second * time.Second // 秒単位でのアップタイム

	// 1. CPUUsages
	// CPUの使用率を1秒間隔で取得
	cpuStats, err := cpu.Percent(time.Second, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU usages: %w", err)
	}
	stats.CPUUsages = cpuStats

	// 2. MemoryUsage (RAM)
	vmem, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual memory info: %w", err)
	}
	// メモリ使用量をバイト単位で取得
	stats.MemoryUsage = vmem.Used

	// 3. DiskUsage
	// ルートパーティション ("/") のディスク使用量を取得
	dstats, err := disk.Usage("/")
	if err != nil {
		return nil, fmt.Errorf("failed to get disk usage info: %w", err)
	}
	// ディスク使用量をバイト単位で取得
	stats.DiskUsage = dstats.Used

	// 4. IOUtilization
	// I/Oの利用率は直接取得するのが難しい場合があるため、
	// ディスクのI/Oカウンタの変化から計算する必要があります。
	// この例では、IOUtilizationはゼロとしています。
	// より詳細な実装には、ディスクI/Oのメトリクスを一定時間計測する必要があります。
	stats.IOUtilization = 0.0

	// 5. DiskSwap
	swap, err := mem.SwapMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get swap memory info: %w", err)
	}
	// スワップ使用量をパーセンテージで取得
	stats.DiskSwap = swap.UsedPercent

	// 6. LoadAvg
	// ロードアベレージは過去1分間の平均を取得
	loadAvg, err := load.Avg()
	if err != nil {
		return nil, fmt.Errorf("failed to get load average: %w", err)
	}
	stats.LoadAvg = loadAvg.Load1

	return stats, nil
}
