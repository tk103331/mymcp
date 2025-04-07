package bind

import (
	"context"
	"mcphosting/manager/data"
)

type Setting struct {
	ctx *context.Context
}

func (s *Setting) LoadSettings() (*data.Settings, error) {
	return data.LoadSettings()
}

func (s *Setting) SaveSettings(settings *data.Settings) error {
	return data.SaveSettings(settings)
}
