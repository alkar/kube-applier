// Code generated by MockGen. DO NOT EDIT.
// Source: kube/client.go

// Package kube is a generated GoMock package.
package kube

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

// Apply mocks base method
func (m *MockClientInterface) Apply(path, namespace string, dryRun, prune, strict, kustomize bool) (string, string, error) {
	ret := m.ctrl.Call(m, "Apply", path, namespace, dryRun, prune, strict, kustomize)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Apply indicates an expected call of Apply
func (mr *MockClientInterfaceMockRecorder) Apply(path, namespace, dryRun, prune, strict, kustomize interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Apply", reflect.TypeOf((*MockClientInterface)(nil).Apply), path, namespace, dryRun, prune, strict, kustomize)
}

// CheckVersion mocks base method
func (m *MockClientInterface) CheckVersion() error {
	ret := m.ctrl.Call(m, "CheckVersion")
	ret0, _ := ret[0].(error)
	return ret0
}

// CheckVersion indicates an expected call of CheckVersion
func (mr *MockClientInterfaceMockRecorder) CheckVersion() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CheckVersion", reflect.TypeOf((*MockClientInterface)(nil).CheckVersion))
}

// GetNamespaceStatus mocks base method
func (m *MockClientInterface) GetNamespaceStatus(namespace string) (AutomaticDeploymentOption, error) {
	ret := m.ctrl.Call(m, "GetNamespaceStatus", namespace)
	ret0, _ := ret[0].(AutomaticDeploymentOption)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNamespaceStatus indicates an expected call of GetNamespaceStatus
func (mr *MockClientInterfaceMockRecorder) GetNamespaceStatus(namespace interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNamespaceStatus", reflect.TypeOf((*MockClientInterface)(nil).GetNamespaceStatus), namespace)
}

// GetNamespaceUserSecretName mocks base method
func (m *MockClientInterface) GetNamespaceUserSecretName(namespace, username string) (string, error) {
	ret := m.ctrl.Call(m, "GetNamespaceUserSecretName", namespace, username)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNamespaceUserSecretName indicates an expected call of GetNamespaceUserSecretName
func (mr *MockClientInterfaceMockRecorder) GetNamespaceUserSecretName(namespace, username interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNamespaceUserSecretName", reflect.TypeOf((*MockClientInterface)(nil).GetNamespaceUserSecretName), namespace, username)
}

// GetUserDataFromSecret mocks base method
func (m *MockClientInterface) GetUserDataFromSecret(namespace, secret string) (string, string, error) {
	ret := m.ctrl.Call(m, "GetUserDataFromSecret", namespace, secret)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetUserDataFromSecret indicates an expected call of GetUserDataFromSecret
func (mr *MockClientInterfaceMockRecorder) GetUserDataFromSecret(namespace, secret interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserDataFromSecret", reflect.TypeOf((*MockClientInterface)(nil).GetUserDataFromSecret), namespace, secret)
}

// CreateTempConfig mocks base method
func (m *MockClientInterface) CreateTempConfig(namespace, serviceAccount string) (string, string, error) {
	ret := m.ctrl.Call(m, "CreateTempConfig", namespace, serviceAccount)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateTempConfig indicates an expected call of CreateTempConfig
func (mr *MockClientInterfaceMockRecorder) CreateTempConfig(namespace, serviceAccount interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTempConfig", reflect.TypeOf((*MockClientInterface)(nil).CreateTempConfig), namespace, serviceAccount)
}
