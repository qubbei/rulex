package device

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/hootrhino/rulex/common"
	"github.com/hootrhino/rulex/component/interdb"
	"github.com/hootrhino/rulex/glogger"
	"github.com/hootrhino/rulex/typex"
	"github.com/hootrhino/rulex/utils"
	"github.com/robinson/gos7"
)

// 点位表
type SiemensDataPoint struct {
	UUID          string `json:"uuid"` // 当UUID为空时新建
	DeviceUuid    string `json:"device_uuid"`
	Tag           string `json:"tag,omitempty"`
	Type          string `json:"type,omitempty"`
	Frequency     *int64 `json:"frequency,omitempty"`
	Address       *int   `json:"address,omitempty"`
	Start         *int   `json:"start"`
	Size          *int   `json:"size"`
	Status        int    `json:"status"`        // 运行时数据
	LastFetchTime uint64 `json:"lastFetchTime"` // 运行时数据
	Value         string `json:"value"`         // 运行时数据
}

type S1200CommonConfig struct {
	Host  string `json:"host" validate:"required"`  // 127.0.0.1:502
	Model string `json:"model" validate:"required"` // s7-200 s7-1500
	// https://cloudvpn.beijerelectronics.com/hc/en-us/articles/4406049761169-Siemens-S7
	Rack        *int  `json:"rack" validate:"required"`        // 0
	Slot        *int  `json:"slot" validate:"required"`        // 1
	Timeout     *int  `json:"timeout" validate:"required"`     // 5s
	IdleTimeout *int  `json:"idleTimeout" validate:"required"` // 5s
	AutoRequest *bool `json:"autoRequest" validate:"required"` // false
}
type S1200Config struct {
	CommonConfig S1200CommonConfig `json:"commonConfig" validate:"required"` // 通用配置
}

// https://www.ad.siemens.com.cn/productportal/prods/s7-1200_plc_easy_plus/07-Program/02-basic/01-Data_Type/01-basic.html
type s1200plc struct {
	typex.XStatus
	status            typex.DeviceState
	RuleEngine        typex.RuleX
	mainConfig        S1200Config
	client            gos7.Client
	lock              sync.Mutex
	SiemensDataPoints map[string]*SiemensDataPoint
}

/*
*
* 西门子 S1200 系列 PLC
*
 */
func NewS1200plc(e typex.RuleX) typex.XDevice {
	s1200 := new(s1200plc)
	s1200.RuleEngine = e
	s1200.lock = sync.Mutex{}
	Rack := 0
	Slot := 1
	Timeout := 1000
	IdleTimeout := 3000
	AutoRequest := false
	s1200.mainConfig = S1200Config{
		CommonConfig: S1200CommonConfig{
			Rack:        &Rack,
			Slot:        &Slot,
			Timeout:     &Timeout,
			IdleTimeout: &IdleTimeout,
			AutoRequest: &AutoRequest,
		},
	}
	s1200.SiemensDataPoints = map[string]*SiemensDataPoint{}
	return s1200
}

// 初始化
func (s1200 *s1200plc) Init(devId string, configMap map[string]interface{}) error {
	s1200.PointId = devId
	if err := utils.BindSourceConfig(configMap, &s1200.mainConfig); err != nil {
		glogger.GLogger.Error(err)
		return err
	}
	// 合并数据库里面的点位表
	// TODO 这里需要优化一下，而不是直接查表这种形式，应该从物模型组件来加载
	// DataSchema = schema.load(uuid)
	// DataSchema.update(k, v)
	var list []SiemensDataPoint
	errDb := interdb.DB().Table("m_siemens_data_points").
		Where("device_uuid=?", devId).Find(&list).Error
	if errDb != nil {
		return errDb
	}
	for _, v := range list {
		// 频率不能太快
		if *v.Frequency < 50 {
			return errors.New("'frequency' must grate than 50 millisecond")
		}
		s1200.SiemensDataPoints[v.UUID] = &v
	}
	return nil
}

// 启动
func (s1200 *s1200plc) Start(cctx typex.CCTX) error {
	s1200.Ctx = cctx.Ctx
	s1200.CancelCTX = cctx.CancelCTX
	//
	handler := gos7.NewTCPClientHandler(
		s1200.mainConfig.CommonConfig.Host,  // 127.0.0.1:1500
		*s1200.mainConfig.CommonConfig.Rack, // 0
		*s1200.mainConfig.CommonConfig.Slot) // 1
	handler.Timeout = time.Duration(
		*s1200.mainConfig.CommonConfig.Timeout) * time.Millisecond
	handler.IdleTimeout = time.Duration(
		*s1200.mainConfig.CommonConfig.IdleTimeout) * time.Millisecond
	if err := handler.Connect(); err != nil {
		return err
	} else {
		s1200.status = typex.DEV_UP
	}

	s1200.client = gos7.NewClient(handler)
	if !*s1200.mainConfig.CommonConfig.AutoRequest {
		s1200.status = typex.DEV_UP
		return nil
	}
	go func(ctx context.Context) {
		// 数据缓冲区,最大4KB
		dataBuffer := make([]byte, common.T_4KB)
		for {
			select {
			case <-ctx.Done():
				{
					return
				}
			default:
				{
				}
			}
			s1200.lock.Lock()
			// CMD 参数无用
			n, err := s1200.Read([]byte(""), dataBuffer)
			s1200.lock.Unlock()
			if err != nil {
				glogger.GLogger.Error(err)
				s1200.status = typex.DEV_DOWN
				return
			}
			ok, err := s1200.RuleEngine.WorkDevice(
				s1200.RuleEngine.GetDevice(s1200.PointId),
				string(dataBuffer[:n]),
			)
			// glogger.GLogger.Debug(string(dataBuffer[:n]))
			if !ok {
				glogger.GLogger.Error(err)
			}
		}

	}(cctx.Ctx)
	return nil
}

// 从设备里面读数据出来
func (s1200 *s1200plc) OnRead(cmd []byte, data []byte) (int, error) {
	return s1200.Read(cmd, data)
}

// 把数据写入设备
//
// db.Address:int, db.Start:int, db.Size:int, rData[]

func (s1200 *s1200plc) OnWrite(cmd []byte, data []byte) (int, error) {
	blocks := []SiemensDataPoint{}
	if err := json.Unmarshal(data, &blocks); err != nil {
		return 0, err
	}
	return s1200.Write(cmd, data)
}

// 设备当前状态
func (s1200 *s1200plc) Status() typex.DeviceState {
	if s1200.client == nil {
		return typex.DEV_DOWN
	}
	return s1200.status

}

// 停止设备
func (s1200 *s1200plc) Stop() {
	s1200.status = typex.DEV_DOWN
	if s1200.CancelCTX != nil {
		s1200.CancelCTX()
	}
}

// 设备属性，是一系列属性描述
func (s1200 *s1200plc) Property() []typex.DeviceProperty {
	return []typex.DeviceProperty{}
}

// 真实设备
func (s1200 *s1200plc) Details() *typex.Device {
	return s1200.RuleEngine.GetDevice(s1200.PointId)
}

// 状态
func (s1200 *s1200plc) SetState(status typex.DeviceState) {
	s1200.status = status
}

// 驱动
func (s1200 *s1200plc) Driver() typex.XExternalDriver {
	return nil
}

func (s1200 *s1200plc) OnDCACall(UUID string, Command string, Args interface{}) typex.DCAResult {
	return typex.DCAResult{}
}
func (s1200 *s1200plc) OnCtrl(cmd []byte, args []byte) ([]byte, error) {
	return []byte{}, nil
}
func (s1200 *s1200plc) Write(cmd []byte, data []byte) (int, error) {
	return 0, nil
}

// 字节格式:[dbNumber1, start1, size1, dbNumber2, start2, size2]
// 读: db --> dbNumber, start, size, buffer[]
var rData = [common.T_2KB]byte{} // 一次最大接受2KB数据

func (s1200 *s1200plc) Read(cmd []byte, data []byte) (int, error) {
	values := []SiemensDataPoint{}
	for uuid, db := range s1200.SiemensDataPoints {
		//DB 4字节
		if db.Type == "DB" {
			// 00.00.00.01 | 00.00.00.02 | 00.00.00.03 | 00.00.00.04
			if err := s1200.client.AGReadDB(*db.Address, *db.Start, *db.Size, rData[:]); err != nil {
				s1200.SiemensDataPoints[uuid].Status = 0
				glogger.GLogger.Error(err)
				return 0, err
			}
			count := db.Size
			if *db.Size*2 > 2000 {
				*count = 2000
			}
			Value := hex.EncodeToString(rData[:*count])
			values = append(values, SiemensDataPoint{
				DeviceUuid: db.DeviceUuid,
				Tag:        db.Tag,
				Address:    db.Address,
				Type:       db.Type,
				Start:      db.Start,
				Size:       db.Size,
				Value:      Value,
			})
			s1200.SiemensDataPoints[uuid].Value = Value
			s1200.SiemensDataPoints[uuid].Status = 1
			s1200.SiemensDataPoints[uuid].LastFetchTime = uint64(time.Now().UnixMilli())
		}
		//
		if db.Type == "MB" {
			// 00.00.00.01 | 00.00.00.02 | 00.00.00.03 | 00.00.00.04
			if err := s1200.client.AGReadMB(*db.Start, *db.Size, rData[:]); err != nil {
				glogger.GLogger.Error(err)
				return 0, err
			}
			count := db.Size
			if *db.Size*2 > 2000 {
				*count = 2000
			}
			Value := hex.EncodeToString(rData[:*count])
			values = append(values, SiemensDataPoint{
				Tag:     db.Tag,
				Type:    db.Type,
				Address: db.Address,
				Start:   db.Start,
				Size:    db.Size,
				Value:   Value,
			})
			s1200.SiemensDataPoints[uuid].Value = Value
			s1200.SiemensDataPoints[uuid].Status = 1
			s1200.SiemensDataPoints[uuid].LastFetchTime = uint64(time.Now().UnixMilli())
		}
		if *db.Frequency < 100 {
			*db.Frequency = 100 // 不能太快
		}
		time.Sleep(time.Duration(*db.Frequency) * time.Millisecond)
	}
	bytes, _ := json.Marshal(values)
	copy(data, bytes)
	return len(bytes), nil
}
