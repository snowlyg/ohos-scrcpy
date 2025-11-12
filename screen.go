package screen

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

// DeviceConnector 客户端用于与scrcpy_server通信
type DeviceConnector struct {
	Host string
	Port int
	Conn net.Conn

	R *io.PipeReader
	W *io.PipeWriter
	F *os.File
}

// 命令类型（必须与服务器保持一致）
const (
	CMD_QUERY_DEVICE_INFO    = 1
	CMD_GET_SCREEN_FRAME     = 2
	CMD_START_SCREEN_CAPTURE = 3
	CMD_STOP_SCREEN_CAPTURE  = 4
	CMD_EXIT                 = 5
)

// 数据包类型（服务器 -> 客户端）
const (
	PKT_DEVICE_INFO  = 1
	PKT_SCREEN_FRAME = 2
	PKT_ACK          = 3
	PKT_ERROR        = 4
	PKT_UNKNOWN      = 255
)

// DeviceInfo 设备信息结构
type DeviceInfo struct {
	Model        string
	Brand        string
	Manufacturer string
	MarketName   string
	OsVersion    string
	ApiVersion   int32
	Dpi          int32
	ScreenWidth  int32
	ScreenHeight int32
	CpuArch      string
}

// NewDeviceConnector 创建一个新的DeviceConnector实例
func NewDeviceConnector(host string, port int) *DeviceConnector {
	dc := &DeviceConnector{
		Host: host,
		Port: port,
	}
	if dc.R == nil || dc.W != nil {
		dc.R, dc.W = io.Pipe()
	}
	if dc.F == nil {
		f, err := os.Create("./output.h264")
		if err != nil {
			panic(err)
		}
		dc.F = f
	}
	return dc
}

// Connect 连接到服务器
func (dc *DeviceConnector) Connect() error {
	var addr string
	if strings.Contains(dc.Host, ":") && !strings.HasPrefix(dc.Host, "[") {
		addr = fmt.Sprintf("[%s]:%d", dc.Host, dc.Port)
	} else {
		addr = fmt.Sprintf("%s:%d", dc.Host, dc.Port)
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	dc.Conn = conn
	return nil
}

// Close 关闭连接
func (dc *DeviceConnector) Close() {
	if dc.Conn != nil {
		dc.Conn.Close()
		dc.Conn = nil
	}
	if dc.R != nil {
		dc.R.Close()
	}
	if dc.W != nil {
		dc.W.Close()
	}
	if dc.F != nil {
		dc.F.Close()
	}
}

// SendCommand 发送命令到服务器
func (dc *DeviceConnector) SendCommand(cmdType int, payload []byte) error {
	if dc.Conn == nil {
		return fmt.Errorf("connection not established")
	}

	// 准备头部（uint8_t type, uint32_t length）
	header := make([]byte, 5)
	header[0] = byte(cmdType)
	binary.LittleEndian.PutUint32(header[1:], uint32(len(payload)))

	// 发送头部和负载
	if _, err := dc.Conn.Write(header); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := dc.Conn.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// RecvPacket 接收数据包
func (dc *DeviceConnector) RecvPacket() (int, []byte, error) {
	if dc.Conn == nil {
		return 0, nil, fmt.Errorf("connection not established")
	}

	// 接收头部
	header := make([]byte, 5)
	if _, err := io.ReadFull(dc.Conn, header); err != nil {
		return 0, nil, err
	}

	pktType := int(header[0])
	length := binary.LittleEndian.Uint32(header[1:])

	// 接收数据主体
	var data []byte
	if length > 0 {
		data = make([]byte, length)
		if _, err := io.ReadFull(dc.Conn, data); err != nil {
			return 0, nil, err
		}
	}

	return pktType, data, nil
}

// GetDeviceInfo 解析数据并返回更友好的设备信息
func GetDeviceInfo(data []byte) (*DeviceInfo, error) {
	model := cleanString(data[0:32])
	brand := cleanString(data[32:64])
	manufacturer := cleanString(data[64:96])
	marketName := cleanString(data[96:128])
	osVersion := cleanString(data[128:160])
	apiVersion := int32(binary.LittleEndian.Uint32(data[160:164]))
	dpi := int32(binary.LittleEndian.Uint32(data[164:168]))
	screenWidth := int32(binary.LittleEndian.Uint32(data[168:172]))
	screenHeight := int32(binary.LittleEndian.Uint32(data[172:176]))
	cpuArch := cleanString(data[176:192])

	return &DeviceInfo{
		Model:        model,
		Brand:        brand,
		Manufacturer: manufacturer,
		MarketName:   marketName,
		OsVersion:    osVersion,
		ApiVersion:   apiVersion,
		Dpi:          dpi,
		ScreenWidth:  screenWidth,
		ScreenHeight: screenHeight,
		CpuArch:      cpuArch,
	}, nil
}

func cleanString(b []byte) string {
	// 找到第一个null字节
	i := bytes.IndexByte(b, 0)
	if i == -1 {
		i = len(b)
	}

	// 截取有效部分并转换为字符串
	s := string(b[:i])

	return strings.TrimSpace(s)
}

// QueryDeviceInfo 查询设备信息
func (dc *DeviceConnector) QueryDeviceInfo() (*DeviceInfo, error) {
	if err := dc.SendCommand(CMD_QUERY_DEVICE_INFO, nil); err != nil {
		return nil, err
	}

	pktType, data, err := dc.RecvPacket()
	if err != nil {
		return nil, err
	}

	if pktType == PKT_DEVICE_INFO {
		// 解析DeviceInfo结构
		return GetDeviceInfo(data)
	}

	return nil, fmt.Errorf("unexpected packet type: %d", pktType)
}

// StartScreenCapture 开始屏幕捕获
func (dc *DeviceConnector) StartScreenCapture() error {
	fmt.Println("Starting screen capture...")
	return dc.SendCommand(CMD_START_SCREEN_CAPTURE, nil)
}

// StopScreenCapture 停止屏幕捕获
func (dc *DeviceConnector) StopScreenCapture() error {
	fmt.Println("Stopping screen capture...")
	return dc.SendCommand(CMD_STOP_SCREEN_CAPTURE, nil)
}

// Exit 退出
func (dc *DeviceConnector) Exit() error {
	fmt.Println("Exiting...")
	return dc.SendCommand(CMD_EXIT, nil)
}

func (sr *DeviceConnector) SendToPipe() {
	// 接收帧数据
	for {
		// 接收头部
		header := make([]byte, 5)
		if _, err := io.ReadFull(sr.Conn, header); err != nil {
			log.Printf("ReadFull get error %s\n", err.Error())
			continue
		}

		pktType := int(header[0])
		if pktType != PKT_SCREEN_FRAME {
			// fmt.Printf("Received unknown packet type: %d\n", pktType)
			continue
		}

		length := binary.LittleEndian.Uint32(header[1:])
		log.Printf("Received packet length: %d, type: %d\n", length, pktType)

		data := make([]byte, length)
		if _, err := io.ReadFull(sr.Conn, data); err != nil {
			log.Printf("ReadFull get error %s\n", err.Error())
			continue
		}
		n, err := sr.W.Write(data)
		if err != nil {
			log.Printf("Write get error %s\n", err.Error())
		}
		log.Printf("Wrote %d bytes to pipe\n", n)
	}
}

func (sr *DeviceConnector) SaveToDesk() {
	if sr.F == nil {
		f, err := os.Create("./output.h264")
		if err != nil {
			panic(err)
		}
		sr.F = f
	}
	// 接收帧数据
	for {
		// 接收头部
		header := make([]byte, 5)
		if _, err := io.ReadFull(sr.Conn, header); err != nil {
			log.Printf("ReadFull get error %s\n", err.Error())
			continue
		}

		pktType := int(header[0])
		if pktType != PKT_SCREEN_FRAME {
			// fmt.Printf("Received unknown packet type: %d\n", pktType)
			continue
		}

		length := binary.LittleEndian.Uint32(header[1:])
		log.Printf("Received packet length: %d, type: %d\n", length, pktType)

		data := make([]byte, length)
		if _, err := io.ReadFull(sr.Conn, data); err != nil {
			log.Printf("ReadFull get error %s\n", err.Error())
			continue
		}
		n, err := sr.F.Write(data)
		if err != nil {
			log.Printf("Write get error %s\n", err.Error())
		}
		log.Printf("Wrote %d bytes to pipe\n", n)
	}
}
