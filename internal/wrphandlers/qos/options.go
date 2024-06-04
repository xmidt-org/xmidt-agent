// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"errors"
	"fmt"
	"time"
)

const (
	// Priority queue defaults.
	DefaultMaxQueueBytes   = 1 * 1024 * 1024 // 1MB max/queue
	DefaultMaxMessageBytes = 256 * 1024      // 256 KB

	// QOS expires defaults.
	DefaultLowQOSExpires      = time.Minute * 15
	DefaultMediumQOSExpires   = time.Minute * 20
	DefaultHighQOSExpires     = time.Minute * 25
	DefaultCriticalQOSExpires = time.Minute * 30
)

// MaxQueueBytes is the allowable max size of the qos' priority queue, based on the sum of all queued wrp message's payload.
// Note, the default zero behavior is a queue with a 1MB size constraint.
func MaxQueueBytes(s int64) Option {
	return optionFunc(
		func(h *Handler) error {
			if s < 0 {
				return fmt.Errorf("%w: negative MaxQueueBytes", ErrMisconfiguredQOS)
			} else if s == 0 {
				s = DefaultMaxQueueBytes
			}

			h.maxQueueBytes = s

			return nil
		})
}

// MaxMessageBytes is the largest allowable wrp message payload.
// Note, the default zero behavior is a 256KB payload size constraint.
func MaxMessageBytes(s int) Option {
	return optionFunc(
		func(h *Handler) error {
			if s < 0 {
				return fmt.Errorf("%w: negative MaxMessageBytes", ErrMisconfiguredQOS)
			} else if s == 0 {
				s = DefaultMaxMessageBytes
			}

			h.maxMessageBytes = s

			return nil
		})
}

// Priority determines what is used [newest, oldest message] for QualityOfService tie breakers and trimming,
// with the default being to prioritize the newest messages.
func Priority(p PriorityType) Option {
	return optionFunc(
		func(h *Handler) (err error) {
			h.tieBreaker, err = priority(p)
			h.priority = p

			return err
		})
}

// priority determines which tie breakers are used during normal enqueueing.
func priority(p PriorityType) (enqueueTieBreaker tieBreaker, err error) {
	// Determine what will be used as a QualityOfService tie breaker during normal enqueueing.
	switch p {
	case NewestType:
		// Prioritize the newest messages.
		enqueueTieBreaker = PriorityNewestMsg
	case OldestType:
		// Prioritize the oldest messages.
		enqueueTieBreaker = PriorityOldestMsg
	default:
		return nil, errors.Join(fmt.Errorf("%w: %s", ErrPriorityTypeInvalid, p), ErrMisconfiguredQOS)
	}

	return enqueueTieBreaker, nil
}

// LowQOSExpires determines when low qos messages are trimmed.
func LowQOSExpires(t time.Duration) Option {
	return optionFunc(
		func(h *Handler) (err error) {
			if t < 0 {
				return fmt.Errorf("%w: negative LowQOSExpires", ErrMisconfiguredQOS)
			}

			h.lowQOSExpires = t
			return err
		})
}

// MediumQOSExpires determines when medium qos messages are trimmed.
func MediumQOSExpires(t time.Duration) Option {
	return optionFunc(
		func(h *Handler) (err error) {
			if t < 0 {
				return fmt.Errorf("%w: negative MediumQOSExpires", ErrMisconfiguredQOS)
			}

			h.mediumQOSExpires = t
			return err
		})
}

// HighQOSExpires determines when high qos messages are trimmed.
func HighQOSExpires(t time.Duration) Option {
	return optionFunc(
		func(h *Handler) (err error) {
			if t < 0 {
				return fmt.Errorf("%w: negative HighQOSExpires", ErrMisconfiguredQOS)
			}

			h.highQOSExpires = t
			return err
		})
}

// CriticalQOSExpires determines when critical qos messages are trimmed.
func CriticalQOSExpires(t time.Duration) Option {
	return optionFunc(
		func(h *Handler) (err error) {
			if t < 0 {
				return fmt.Errorf("%w: negative CriticalQOSExpires", ErrMisconfiguredQOS)
			}

			h.criticalQOSExpires = t
			return err
		})
}
