package clustering

import (
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

const (
	SET_OPERATOR_NUM int = 8
)

func (m *DataServiceProvider) runSetData(num int) {
	m.DWait.Wait()
	core.AppLog.Info().Msgf("starting set operator %d", num)
	for sd := range m.DSet {
		if sd.Opt == core.SET_OPT_CLOSE {
			core.AppLog.Debug().Msgf("closing set data operator %d", num)
			return
		}
		switch sd.Opt {
		case core.CREATE_DATA_REQUEST:
			ki, err := m.create(sd)
			if err != nil {
				sd.Resp <- &protocol.Response{Successful: false, Message: err.Error()}
			} else {
				var data []*protocol.Data
				data = append(data, &protocol.Data{Header: &protocol.Header{Revision: ki.Header.Revision}})
				sd.Resp <- &protocol.Response{Successful: true, Data: &protocol.DataSet{List: data}}
			}
		case core.UPDATE_DATA_REQUEST:
			ki, err := m.update(sd)
			if err != nil {
				sd.Resp <- &protocol.Response{Successful: false, Message: err.Error()}
			} else {
				var data []*protocol.Data
				data = append(data, &protocol.Data{Header: &protocol.Header{Revision: ki.Header.Revision}})
				sd.Resp <- &protocol.Response{Successful: true, Data: &protocol.DataSet{List: data}}
			}
		case core.DELETE_DATA_REQUEST:
			ki, err := m.delete(sd)
			if err != nil {
				sd.Resp <- &protocol.Response{Successful: false, Message: err.Error()}
			} else {
				var data []*protocol.Data
				data = append(data, &protocol.Data{Header: &protocol.Header{Revision: ki.Header.Revision}})
				sd.Resp <- &protocol.Response{Successful: true, Data: &protocol.DataSet{List: data}}
			}
		case core.RESET_DATA_REQUEST:
			ki, err := m.reset(sd)
			if err != nil {
				sd.Resp <- &protocol.Response{Successful: false, Message: err.Error()}
			} else {
				var data []*protocol.Data
				data = append(data, &protocol.Data{Header: &protocol.Header{Revision: ki.Header.Revision}})
				sd.Resp <- &protocol.Response{Successful: true, Data: &protocol.DataSet{List: data}}
			}
		default:
			sd.Resp <- &protocol.Response{Successful: false, Message: fmt.Sprintf("set opt not supported %d", sd.Opt)}
		}
	}
}
