package system

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oneclickvirt/basics/model"
	"github.com/oneclickvirt/basics/system/gpu"
	gpustat "github.com/oneclickvirt/basics/system/gpu/stat"
	. "github.com/oneclickvirt/defaultset"
)

var updateGPUStatus int32
var gpuStat uint64

// 获取设备数据的最大尝试次数
const maxDeviceDataFetchAttempts = 3

// 获取主机数据的尝试次数，Key 为 Host 的属性名
var hostDataFetchAttempts = map[string]int{
	"GPU": 0,
}

// 获取状态数据的尝试次数，Key 为 HostState 的属性名
var statDataFetchAttempts = map[string]int{
	"GPU": 0,
}

func atomicStoreFloat64(x *uint64, v float64) {
	atomic.StoreUint64(x, math.Float64bits(v))
}

func updateGPUStat(gpuStat *uint64, wg *sync.WaitGroup) {
	if model.EnableLoger {
		InitLogger()
		defer Logger.Sync()
	}
	defer wg.Done()
	if !atomic.CompareAndSwapInt32(&updateGPUStatus, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&updateGPUStatus, 0)
	for statDataFetchAttempts["GPU"] < maxDeviceDataFetchAttempts {
		gs, err := gpustat.GetGPUStat()
		if err != nil {
			statDataFetchAttempts["GPU"]++
			if model.EnableLoger {
				Logger.Info(fmt.Sprintf("gpustat.GetGPUStat error: %s, attempt: %d", err.Error(), statDataFetchAttempts["GPU"]))
			}
			time.Sleep(1 * time.Second) // 等待一段时间再重试
		} else {
			statDataFetchAttempts["GPU"] = 0
			atomicStoreFloat64(gpuStat, gs)
			break
		}
	}
}

func getGPUInfo(ret *model.SystemInfo) (*model.SystemInfo, error) {
	gpuModels, err := gpu.GetGPUModel()
	if len(gpuModels) > 0 {
		if err != nil {
			hostDataFetchAttempts["GPU"]++
			return ret, fmt.Errorf("no gpu")
		} else {
			hostDataFetchAttempts["GPU"] = 0
		}
		ret.GpuModel = gpuModels[0]
		var wg sync.WaitGroup
		wg.Add(1)
		go updateGPUStat(&gpuStat, &wg)
		wg.Wait() // 等待 updateGPUStat 完成
		if math.Float64frombits(gpuStat) > 0 {
			ret.GpuStats = fmt.Sprintf("%f", math.Float64frombits(gpuStat))
		}
		return ret, nil
	} else {
		hostDataFetchAttempts["GPU"]++
		return ret, fmt.Errorf("no gpu")
	}
}
