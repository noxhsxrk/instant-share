package network

import (
	"fmt"
	"net"
)

func GetLocalIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to get network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			return ip.String(), nil
		}
	}

	return "", fmt.Errorf("no valid local IP address found")
}

func GetAvailablePort(preferred int) (int, error) {
	if preferred > 0 {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", preferred))
		if err == nil {
			listener.Close()
			return preferred, nil
		}
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
