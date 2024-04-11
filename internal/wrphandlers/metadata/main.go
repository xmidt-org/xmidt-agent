package metadata

import (
    "fmt"
    "net"
)

func localAddresses() {
    ifaces, err := net.Interfaces()
    if err != nil {
        fmt.Print(fmt.Errorf("localAddresses: %+v\n", err.Error()))
        return
    }

    for _, i := range ifaces {
        addrs, err := i.Addrs()
        if err != nil {
            fmt.Print(fmt.Errorf("localAddresses: %+v\n", err.Error()))
        }

        for _, a := range addrs {
            fmt.Printf("%v - %v - %v - %v\n", i.Name,  a, i.Flags&net.FlagUp != 0, i.Flags&net.FlagRunning)
        }
    }
}

func main() {
    fmt.Println("Starting")
    localAddresses()
}