// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package net

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"testing"
)

type NetworkWrapperSuite struct {
	suite.Suite
	networkWrapper NetworkWrapper
}

func TestNetworkWrapperSuite(t *testing.T) {
	suite.Run(t, new(NetworkWrapperSuite))
}

func (suite *NetworkWrapperSuite) SetupTest() {
	suite.networkWrapper = NewNetworkWrapper()
}

func (suite *NetworkWrapperSuite) TestDefaultInterface() {
	result, err := suite.networkWrapper.DefaultInterface()
	fmt.Println(result)
	suite.NoError(err)
}
