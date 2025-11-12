package main

import (
	"fmt"

	"github.com/chindeo/screen"
)

func main() {
	connector := screen.NewDeviceConnector("192.168.20.156", 12345)
	// 连接到服务器
	if err := connector.Connect(); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer connector.Close()

	// 开始屏幕捕获
	if err := connector.StartScreenCapture(); err != nil {
		fmt.Printf("Failed to start screen capture: %v\n", err)
		return
	}
	defer connector.StopScreenCapture()
	if connector.Conn == nil {
		fmt.Printf("Connection not established\n")
		return
	}

	// 查询设备信息
	deviceInfo, err := connector.QueryDeviceInfo()
	if err != nil {
		fmt.Printf("Failed to query device info: %v\n", err)
		return
	}
	fmt.Printf("Device Info: %+v\n", deviceInfo)
	connector.SaveToDesk()
}
