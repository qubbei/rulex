// Copyright (C) 2023 wwhai
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package service

import (
	"encoding/json"
	"runtime"

	"github.com/hootrhino/rulex/component/interdb"
	"github.com/hootrhino/rulex/ossupport"
	"github.com/hootrhino/rulex/plugin/rulex_api_server/model"
	"github.com/hootrhino/rulex/typex"
	"go.bug.st/serial"
)

type UartConfigDto struct {
	Timeout  int
	Uart     string
	BaudRate int
	DataBits int
	Parity   string
	StopBits int
}
type HwPortDto struct {
	UUID     string
	Name     string        // 接口名称
	Type     string        // 接口类型, UART(串口),USB(USB),FD(通用文件句柄)
	Alias    string        // 别名
	Busy     bool          // 是否被占
	OccupyBy string        // 被谁占用了
	Config   UartConfigDto // 配置, 串口配置、或者网卡、USB等
}

func (u UartConfigDto) JsonString() string {
	if bytes, err := json.Marshal(u); err != nil {
		return ""
	} else {
		return string(bytes)
	}
}

/*
*
* 所有的接口列表配置
*
 */
func AllHwPort() ([]model.MHwPort, error) {
	ports := []model.MHwPort{}
	return ports, interdb.DB().
		Model(&model.MHwPort{}).Find(&ports).Error
}

/*
*
* 配置WIFI HwPort
*
 */
func UpdateHwPortConfig(MHwPort model.MHwPort) error {
	Model := model.MHwPort{}
	return interdb.DB().
		Model(Model).
		Where("uuid=?", MHwPort.UUID).
		Updates(MHwPort).Error
}

/*
*
* 获取HwPort的配置信息
*
 */
func GetHwPortConfig(uuid string) (model.MHwPort, error) {
	MHwPort := model.MHwPort{}
	err := interdb.DB().
		Where("uuid=?", uuid).
		Find(&MHwPort).Error
	return MHwPort, err
}

/*
*
* 初始化网卡配置参数
*
 */
func InitHwPortConfig() error {
	for _, portName := range GetOsPort() {
		Port := model.MHwPort{
			UUID: portName,
			Name: portName,
			Type: "UART",
			Alias: func() string {
				// Alias Ext
				return portName
			}(),
			Description: portName,
		}
		uartCfg := UartConfigDto{
			Timeout:  3000,
			Uart:     portName,
			BaudRate: 9600,
			DataBits: 8,
			Parity:   "N",
			StopBits: 1,
		}
		Port.Config = uartCfg.JsonString()
		err1 := interdb.DB().
			Model(Port).Where("uuid", portName).
			FirstOrCreate(&Port).Error
		if err1 != nil {
			return err1
		}
	}

	return nil
}

/*
*
* 获取系统串口, 这个接口比较特殊，当运行在特殊硬件上的时候，某些系统占用的直接不显示
*
 */
func GetOsPort() []string {
	var ports []string
	if runtime.GOOS == "windows" {
		ports, _ = serial.GetPortsList()
	} else {
		ports, _ = ossupport.GetPortsListUnix()
	}
	List := []string{}
	for _, port := range ports {
		if typex.DefaultVersion.Product == "EEKIIH3" {
			// H3的下列串口被网卡占用
			if port == "/dev/ttyS0" {
				continue
			}
			if port == "/dev/ttyS3" {
				continue
			}
			if port == "/dev/ttyUSB0" { // 4G
				continue
			}
			if port == "/dev/ttyUSB1" { // Lora
				continue
			}
			if port == "/dev/ttyUSB2" { // 外部扩展卡
				continue
			}
		}
		List = append(List, port)
	}
	return List
}
