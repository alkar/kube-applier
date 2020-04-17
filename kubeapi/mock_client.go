// Code generated by MockGen. DO NOT EDIT.
// Source: kubeapi/client.go

// Package kubeapi is a generated GoMock package.
package kubeapi

import (
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockClientInterface is a mock of ClientInterface interface
type MockClientInterface struct {
	ctrl     *gomock.Controller
	recorder *MockClientInterfaceMockRecorder
}

// MockClientInterfaceMockRecorder is the mock recorder for MockClientInterface
type MockClientInterfaceMockRecorder struct {
	mock *MockClientInterface
}

// NewMockClientInterface creates a new mock instance
func NewMockClientInterface(ctrl *gomock.Controller) *MockClientInterface {
	mock := &MockClientInterface{ctrl: ctrl}
	mock.recorder = &MockClientInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockClientInterface) EXPECT() *MockClientInterfaceMockRecorder {
	return m.recorder
}

// PrunableResourceGVKs mocks base method
func (m *MockClientInterface) PrunableResourceGVKs() ([]string, []string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PrunableResourceGVKs")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].([]string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// PrunableResourceGVKs indicates an expected call of PrunableResourceGVKs
func (mr *MockClientInterfaceMockRecorder) PrunableResourceGVKs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PrunableResourceGVKs", reflect.TypeOf((*MockClientInterface)(nil).PrunableResourceGVKs))
}
