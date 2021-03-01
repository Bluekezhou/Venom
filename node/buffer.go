package node

import (
	"errors"
	"io"
	"log"
	"sync"

	"github.com/Dliv3/Venom/global"
	"github.com/Dliv3/Venom/protocol"
)

// Buffer is an internal commucation bridge
type Buffer struct {
	// data channel
	Chan chan interface{}
	// error channel
	Error chan string
}

// NewBuffer 创建一个Buffer
func NewBuffer() *Buffer {
	return &Buffer{
		Chan:  make(chan interface{}, global.BUFFER_SIZE),
		Error: make(chan string),
	}
}

// ReadLowLevelPacket 从缓冲区读取最底层的数据并解析
func (buffer *Buffer) ReadLowLevelPacket() (protocol.Packet, error) {
	select {
	case packet := <-buffer.Chan:
		switch packet.(type) {
		case protocol.Packet:
			return packet.(protocol.Packet), nil
		case error:
			return protocol.Packet{}, io.EOF
		default:
			return protocol.Packet{}, errors.New("Data Type Error")
		}
	case err := <-buffer.Error:
		return protocol.Packet{}, errors.New(err)
	}
}

// ReadPacket 从缓冲区读取格式化数据
func (buffer *Buffer) ReadPacket(packetHeader *protocol.PacketHeader, packetData interface{}) error {
	packet, err := buffer.ReadLowLevelPacket()
	if err != nil {
		return err
	}
	if packetHeader != nil {
		packet.ResolveHeader(packetHeader)
	}
	if packetData != nil {
		packet.ResolveData(packetData)
	}
	return nil
}

// WriteLowLevelPacket 把最底层数据包写入缓冲区
func (buffer *Buffer) WriteLowLevelPacket(packet protocol.Packet) {
	buffer.Chan <- packet
}

// WriteBytes 把二进制数据写入缓冲区
func (buffer *Buffer) WriteBytes(data []byte) {
	buffer.Chan <- data
}

// ReadBytes 从缓冲区读取二进制数据
func (buffer *Buffer) ReadBytes() ([]byte, error) {
	if buffer == nil {
		return nil, errors.New("Buffer is null")
	}
	data := <-buffer.Chan
	// select {
	// case <-time.After(time.Second * TIME_OUT):
	// 	return nil, errors.New("TimeOut")
	// case data := <-buffer.Chan:
	// 	switch data.(type) {
	// 	case []byte:
	// 		return data.([]byte), nil
	// 	// Fix Bug : socks5连接不会断开的问题
	// 	case error:
	// 		return nil, io.EOF
	// 	default:
	// 		return nil, errors.New("Data Type Error")
	// 	}
	// }
	switch data.(type) {
	case []byte:
		return data.([]byte), nil
	// Fix Bug : socks5连接不会断开的问题
	case error:
		return nil, io.EOF
	default:
		return nil, errors.New("Data Type Error")
	}
}

// WriteCloseMessage Fix Bug : socks5连接不会断开的问题
func (buffer *Buffer) WriteCloseMessage() {
	if buffer != nil {
		buffer.Chan <- io.EOF
	}
}

// WriteErrorMessage 发送错误信息到缓冲区
func (buffer *Buffer) WriteErrorMessage(err string) {
	if buffer != nil {
		buffer.Error <- err
	}
}

type DataBuffer struct {
	// 数据信道缓冲区
	DataBuffer     [global.TCP_MAX_CONNECTION]*Buffer
	DataBufferLock *sync.RWMutex

	// Session ID
	SessionID     uint16
	SessionIDLock *sync.Mutex
}

func NewDataBuffer() *DataBuffer {
	return &DataBuffer{
		SessionIDLock:  &sync.Mutex{},
		DataBufferLock: &sync.RWMutex{},
	}
}

func (dataBuffer *DataBuffer) GetDataBuffer(sessionID uint16) *Buffer {
	if int(sessionID) > len(dataBuffer.DataBuffer) {
		log.Println("[-]DataBuffer sessionID error: ", sessionID)
		return nil
	}
	dataBuffer.DataBufferLock.RLock()
	defer dataBuffer.DataBufferLock.RUnlock()
	return dataBuffer.DataBuffer[sessionID]
}

func (dataBuffer *DataBuffer) NewDataBuffer(sessionID uint16) {
	dataBuffer.DataBufferLock.Lock()
	defer dataBuffer.DataBufferLock.Unlock()
	dataBuffer.DataBuffer[sessionID] = NewBuffer()
}

func (dataBuffer *DataBuffer) RealseDataBuffer(sessionID uint16) {
	dataBuffer.DataBufferLock.Lock()
	defer dataBuffer.DataBufferLock.Unlock()
	dataBuffer.DataBuffer[sessionID] = nil
}

func (dataBuffer *DataBuffer) GetSessionID() uint16 {
	dataBuffer.SessionIDLock.Lock()
	defer dataBuffer.SessionIDLock.Unlock()
	id := dataBuffer.SessionID
	dataBuffer.SessionID = (dataBuffer.SessionID + 1) % global.TCP_MAX_CONNECTION
	return id
}
