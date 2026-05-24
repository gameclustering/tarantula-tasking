package bootstrap

import (
	"os"

	"gameclustering.com/internal/core"
	"github.com/rs/zerolog"
)

func CreateTestLog() {
	core.AppLog = zerolog.New(os.Stderr).With().Timestamp().Logger().With().Caller().Logger()
	core.AppLog.Info().Msg("Initialized test app log")
}
