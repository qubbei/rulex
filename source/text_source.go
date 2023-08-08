package source

import (
	"bufio"
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/hootrhino/rulex/common"
	"github.com/hootrhino/rulex/glogger"
	"github.com/hootrhino/rulex/typex"
	"github.com/hootrhino/rulex/utils"
	"io"
	"os"
	"path"
	"strings"
)

// 从txt文件读取数据，作为数据源
type textInEndSource struct {
	typex.XStatus
	mainConfig common.TextConfig
	status     typex.SourceState
}

func NewTextInEndSource(e typex.RuleX) typex.XSource {
	h := textInEndSource{}
	h.RuleEngine = e
	return &h
}
func (*textInEndSource) Configs() *typex.XConfig {
	return &typex.XConfig{}
}
func (tt *textInEndSource) Init(inEndId string, configMap map[string]interface{}) error {
	tt.PointId = inEndId
	if err := utils.BindSourceConfig(configMap, &tt.mainConfig); err != nil {
		return err
	}
	return nil
}

func (tt *textInEndSource) Start(cctx typex.CCTX) error {
	tt.Ctx = cctx.Ctx
	tt.CancelCTX = cctx.CancelCTX

	filepath := tt.mainConfig.Path
	name := tt.mainConfig.Name
	file, err := os.Open(fmt.Sprintf("%v/%v", filepath, name))
	if err != nil {
		glogger.GLogger.Error(err)
		return nil
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		glogger.GLogger.Error(err)
		return nil
	}
	err = watcher.Add(filepath)
	if err != nil {
		glogger.GLogger.Error(err)
		return nil
	}

	go func(ctx context.Context) {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					name := event.Name
					if path.Ext(name) == "" {
						return
					}
					buf := bufio.NewReader(file)
					for {
						line, err := buf.ReadString('\n')
						if strings.TrimSpace(line) == "" {
							if err != nil && err == io.EOF {
								break
							}
							continue
						}
						if work, err := tt.RuleEngine.WorkInEnd(tt.RuleEngine.GetInEnd(tt.PointId), line); !work {
							glogger.GLogger.Error(err)
						}
						if err != nil {
							if err == io.EOF {
								glogger.GLogger.Info("File read ok!")
								break
							}
							glogger.GLogger.Error("Read file error!", err)
							return
						}
					}
				}
				if event.Op&fsnotify.Create == fsnotify.Create {
					name := event.Name
					if path.Ext(name) == "" {
						watcher.Add(name)
						return
					}
					file, err = os.Open(event.Name)
					if err != nil {
						glogger.GLogger.Error(err)
						return
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					glogger.GLogger.Error(err)
					return
				}
				glogger.GLogger.Error(err)
			}
		}
	}(tt.Ctx)
	tt.status = typex.SOURCE_UP
	return nil
}

func (tt *textInEndSource) DataModels() []typex.XDataModel {
	return tt.XDataModels
}

func (tt *textInEndSource) Stop() {
	tt.CancelCTX()
	tt.status = typex.SOURCE_STOP
}
func (tt *textInEndSource) Reload() {

}
func (tt *textInEndSource) Pause() {

}
func (tt *textInEndSource) Status() typex.SourceState {
	return tt.status
}

func (tt *textInEndSource) Test(inEndId string) bool {
	return true
}

func (tt *textInEndSource) Enabled() bool {
	return tt.Enable
}
func (tt *textInEndSource) Details() *typex.InEnd {
	return tt.RuleEngine.GetInEnd(tt.PointId)
}

func (*textInEndSource) Driver() typex.XExternalDriver {
	return nil
}

// 拓扑
func (*textInEndSource) Topology() []typex.TopologyPoint {
	return []typex.TopologyPoint{}
}

// 来自外面的数据
func (*textInEndSource) DownStream([]byte) (int, error) {
	return 0, nil
}

// 上行数据
func (*textInEndSource) UpStream([]byte) (int, error) {
	return 0, nil
}
