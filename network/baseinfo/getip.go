package baseinfo


import (
    "fmt"
    "net"
)

func getip() {
    interfaces, err := net.Interfaces()
    if err != nil {
        fmt.Println("Error getting network interfaces:", err)
        return
    }

    for _, iface := range interfaces {
        addrs, err := iface.Addrs()
        if err != nil {
            fmt.Println("Error getting addresses for interface:", iface.Name, err)
            continue
        }

        fmt.Printf("Interface: %s\n", iface.Name)
        for _, addr := range addrs {
            var ip net.IP
            switch v := addr.(type) {
            case *net.IPNet:
                ip = v.IP
            case *net.IPAddr:
                ip = v.IP
            }

            if ip == nil {
                continue
            }

            // Check if the IP is a global unicast address (indicating it's a public IP)
            if ip.IsGlobalUnicast() {
                fmt.Printf("  IP address: %s\n", ip.String())
            }
        }
    }
}
