package main

import (
	"net/http"
	"strconv"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/util"
)

type CloudMetaGet struct {
	*CloudService
}

func (s *CloudMetaGet) AccessControl() int32 {
	return core.ADMIN_ACCESS_CONTROL
}

func (s *CloudMetaGet) Request(rs core.OnSession, w http.ResponseWriter, r *http.Request) {
	taskIdStr := r.PathValue("taskId")
	taskId, err := strconv.ParseUint(taskIdStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: "invalid taskId"}))
		return
	}
	results, err := s.queryByTaskId(taskId)
	if err != nil {
		w.Write(util.ToJson(core.OnSession{Successful: false, Message: err.Error()}))
		return
	}
	w.Write(util.ToJson(results))
}
