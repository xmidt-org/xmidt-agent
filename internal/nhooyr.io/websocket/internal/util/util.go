// SPDX-FileCopyrightText: 2023 Anmol Sethi <hi@nhooyr.io>
// SPDX-License-Identifier: ISC

package util

// WriterFunc is used to implement one off io.Writers.
type WriterFunc func(p []byte) (int, error)

func (f WriterFunc) Write(p []byte) (int, error) {
	return f(p)
}

// ReaderFunc is used to implement one off io.Readers.
type ReaderFunc func(p []byte) (int, error)

func (f ReaderFunc) Read(p []byte) (int, error) {
	return f(p)
}
