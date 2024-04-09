// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: LicenseRef-COMCAST

package integrationTests

import (
	"fmt"
	"os"
	"testing"

	"github.com/xmidt-org/idock"
	"github.com/xmidt-org/xmidt-agent"
)

func TestMain(m *testing.M) {
	if testFlags() == "" {
		fmt.Println("Skipping integration tests.")
		os.Exit(0)
	}

	//os.Setenv("TZ", "UTC/UTC")
	infra := idock.New(
		idock.DockerComposeFile("docker.yml"),
		idock.RequireDockerTCPPorts(6200, 6201, 6202, 6203, 6204, 6500, 6501, 6502, 6503, 6504),
		//idock.AfterDocker(),
		idock.Program(func() { _, _ = xmidt_agent.XmidtAgent([]string{"-f", "xmidt_agent.yaml"}, true) }),
		idock.RequireProgramTCPPorts(18111, 18112, 18113),
	)

	err := infra.Start()
	if err != nil {
		panic(err)
	}

	returnCode := m.Run()

	infra.Stop()

	if returnCode != 0 {
		os.Exit(returnCode)
	}
}
