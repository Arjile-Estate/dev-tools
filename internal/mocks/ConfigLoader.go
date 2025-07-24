package mocks

import (
	"dev-tools/internal/config"

	"github.com/stretchr/testify/mock"
)

// ConfigLoader is a mock type for the ConfigLoader type
type ConfigLoader struct {
	mock.Mock
}

// LoadConfig provides a mock function with given fields: path
func (_m *ConfigLoader) LoadConfig(path string) (*config.Config, error) {
	ret := _m.Called(path)

	var r0 *config.Config
	if rf, ok := ret.Get(0).(func(string) *config.Config); ok {
		r0 = rf(path)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*config.Config)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(path)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
