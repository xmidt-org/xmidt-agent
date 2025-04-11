// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0
package quic

import (
	"context"
	"io"
	"net"
	"net/url"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/mock"
)

type MockDialer struct {
	mock.Mock
}

func NewMockDialer() *MockDialer { return &MockDialer{} }

func (m *MockDialer) DialQuic(ctx context.Context, inUrl *url.URL) (quic.Connection, error) {
	args := m.Called(ctx, inUrl)
	return args.Get(0).(quic.Connection), args.Error(1)
}

type MockRedirector struct {
	mock.Mock
}

func NewMockRedirector() *MockRedirector { return &MockRedirector{} }

func (m *MockRedirector) GetUrl(ctx context.Context, inUrl *url.URL) (*url.URL, error) {
	args := m.Called(ctx, inUrl)
	return args.Get(0).(*url.URL), args.Error(1)
}

type MockStream struct {
	mock.Mock
	buf       []byte
	readCount int
}

func NewMockStream(buf []byte) *MockStream {
	return &MockStream{
		buf: buf,
	}
}

func (m *MockStream) StreamID() quic.StreamID {
	args := m.Called()
	return args.Get(0).(quic.StreamID)
}

func (m *MockStream) CancelRead(code quic.StreamErrorCode) {
	m.Called(code)
}

func (m *MockStream) CancelWrite(code quic.StreamErrorCode) {
	m.Called(code)
}

func (m *MockStream) Write(buf []byte) (int, error) {
	args := m.Called(buf)
	return args.Get(0).(int), args.Error(1)
}

func (m *MockStream) Read(buf []byte) (int, error) {
	args := m.Called(buf)
	if args.Error(1) != nil {
		return 0, args.Error(1)
	}

	if m.readCount >= len(m.buf) {
		return 0, io.EOF
	}

	n := copy(buf, m.buf[m.readCount:])
	m.readCount += n
	return n, nil
}

func (m *MockStream) SetReadDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockStream) SetWriteDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockStream) SetDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockStream) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockStream) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

type MockConnection struct {
	mock.Mock
}

func NewMockConnection() *MockConnection { return &MockConnection{} }

// AcceptStream mocks base method.
func (m *MockConnection) AcceptStream(ctx context.Context) (quic.Stream, error) {
	args := m.Called(ctx)
	return args.Get(0).(quic.Stream), args.Error(1)
}

// AcceptUniStream mocks base method.
func (m *MockConnection) AcceptUniStream(ctx context.Context) (quic.ReceiveStream, error) {
	args := m.Called(ctx)
	return args.Get(0).(quic.ReceiveStream), args.Error(1)
}

// CloseWithError mocks base method.
func (m *MockConnection) CloseWithError(code quic.ApplicationErrorCode, desc string) error {
	args := m.Called(code, desc)
	return args.Error(0)
}

// ConnectionState mocks base method.
func (m *MockConnection) ConnectionState() quic.ConnectionState {
	args := m.Called()
	return args.Get(0).(quic.ConnectionState)
}

// Context mocks base method.
func (m *MockConnection) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

// LocalAddr mocks base method.
func (m *MockConnection) LocalAddr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

func (m *MockConnection) OpenStream() (quic.Stream, error) {
	args := m.Called()
	return args.Get(0).(quic.Stream), args.Error(1)
}

func (m *MockConnection) OpenStreamSync(ctx context.Context) (quic.Stream, error) {
	args := m.Called(ctx)
	return args.Get(0).(quic.Stream), args.Error(1)
}

func (m *MockConnection) OpenUniStream() (quic.SendStream, error) {
	args := m.Called()
	return args.Get(0).(quic.SendStream), args.Error(1)
}

func (m *MockConnection) OpenUniStreamSync(ctx context.Context) (quic.SendStream, error) {
	args := m.Called(ctx)
	return args.Get(0).(quic.SendStream), args.Error(1)
}

func (m *MockConnection) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Error(1)
}

// RemoteAddr mocks base method.
func (m *MockConnection) RemoteAddr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

// SendDatagram mocks base method.
func (m *MockConnection) SendDatagram(payload []byte) error {
	args := m.Called(payload)
	return args.Error(0)
}
