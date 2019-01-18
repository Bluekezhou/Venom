package dispather

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"

	"github.com/Dliv3/Venom/global"
	"github.com/Dliv3/Venom/netio"
	"github.com/Dliv3/Venom/node"
	"github.com/Dliv3/Venom/protocol"
	"github.com/Dliv3/Venom/utils"
)

var ERR_UNKNOWN_CMD = errors.New("unknown command type")
var ERR_PROTOCOL_SEPARATOR = errors.New("unknown separator")
var ERR_TARGET_NODE = errors.New("can not find target node")
var ERR_FILE_EXISTS = errors.New("remote file already exists")
var ERR_FILE_NOT_EXISTS = errors.New("remote file not exists")

// AgentClient Admin节点作为Client
func AgentClient(conn net.Conn) {
	result, peerNode := node.ClentInitConnection(conn)
	if result {
		log.Println("[+]Connect to a new node success")
		go node.CurrentNode.CommandHandler(peerNode)
	}
}

// AgentServer Admin节点作为Server
func AgentServer(conn net.Conn) {
	log.Println("[+]Remote Connection: ", conn.RemoteAddr())
	result, peerNode := node.ServerInitConnection(conn)
	if result {
		log.Println("[+]A new node connect to this node success")
		go node.CurrentNode.CommandHandler(peerNode)
	}
}

// InitAgentHandler Agent处理Admin发出的命令
func InitAgentHandler() {
	go handleSyncCmd()
	go handleListenCmd()
	go handleConnectCmd()
	go handleDownloadCmd()
	go handleUploadCmd()
}

func handleSyncCmd() {
	for {
		// fmt.Println("Nodes", node.Nodes)

		var packetHeader protocol.PacketHeader
		var syncPacket protocol.SyncPacket
		node.CurrentNode.CommandBuffers[protocol.SYNC].ReadPacket(&packetHeader, &syncPacket)

		// 重新初始化网络拓扑，这样当有节点断开时网络拓扑会实时改变
		node.GNetworkTopology.InitNetworkMap()

		node.GNetworkTopology.ResolveNetworkMapData(syncPacket.NetworkMap)

		// 通信的对端节点
		var peerNodeID = utils.Array32ToUUID(packetHeader.SrcHashID)

		// nextNode为下一跳
		nextNode := node.Nodes[node.GNetworkTopology.RouteTable[peerNodeID]]

		// 递归向其他节点发送sync同步路由表请求
		for i := range node.Nodes {
			if node.Nodes[i].HashID != peerNodeID && node.Nodes[i].DirectConnection {
				tempPacketHeader := protocol.PacketHeader{
					Separator: global.PROTOCOL_SEPARATOR,
					SrcHashID: utils.UUIDToArray32(node.CurrentNode.HashID),
					DstHashID: utils.UUIDToArray32(node.Nodes[i].HashID),
					CmdType:   protocol.SYNC,
				}
				networkMap := node.GNetworkTopology.GenerateNetworkMapData()
				tempSyncPacket := protocol.SyncPacket{
					NetworkMapLen: uint64(len(networkMap)),
					NetworkMap:    networkMap,
				}

				node.Nodes[i].WritePacket(tempPacketHeader, tempSyncPacket)

				node.CurrentNode.CommandBuffers[protocol.SYNC].ReadPacket(&tempPacketHeader, &tempSyncPacket)

				node.GNetworkTopology.ResolveNetworkMapData(tempSyncPacket.NetworkMap)
			}
		}

		// 生成路由表
		node.GNetworkTopology.UpdateRouteTable()

		// fmt.Println("RouteTable", node.GNetworkTopology.RouteTable)

		// 创建Node结构体
		for key, value := range node.GNetworkTopology.RouteTable {
			if _, ok := node.Nodes[key]; !ok {
				node.Nodes[key] = &node.Node{
					HashID:               key,
					Conn:                 node.Nodes[value].Conn,
					ConnReadLock:         &sync.Mutex{},
					ConnWriteLock:        &sync.Mutex{},
					Socks5SessionIDLock:  &sync.Mutex{},
					Socks5DataBufferLock: &sync.RWMutex{},
				}
			}
		}

		// // 生成节点信息
		// node.GNodeInfo.UpdateNoteInfo()

		packetHeader = protocol.PacketHeader{
			Separator: global.PROTOCOL_SEPARATOR,
			SrcHashID: utils.UUIDToArray32(node.CurrentNode.HashID),
			DstHashID: packetHeader.SrcHashID,
			CmdType:   protocol.SYNC,
		}
		networkMap := node.GNetworkTopology.GenerateNetworkMapData()
		syncPacket = protocol.SyncPacket{
			NetworkMapLen: uint64(len(networkMap)),
			NetworkMap:    networkMap,
		}
		nextNode.WritePacket(packetHeader, syncPacket)

		// fmt.Println(node.CurrentNode.HashID)
		// fmt.Println(node.GNetworkTopology.RouteTable)
		// fmt.Println(node.GNetworkTopology.NetworkMap)
	}
}

func handleListenCmd() {
	for {
		var packetHeader protocol.PacketHeader
		var listenPacketCmd protocol.ListenPacketCmd
		node.CurrentNode.CommandBuffers[protocol.LISTEN].ReadPacket(&packetHeader, &listenPacketCmd)

		// adminNode := node.Nodes[node.GNetworkTopology.RouteTable[utils.Array32ToUUID(packetHeader.SrcHashID)]]

		// 网络拓扑同步完成之后即可直接使用以及构造好的节点结构体
		adminNode := node.Nodes[utils.Array32ToUUID(packetHeader.SrcHashID)]

		err := netio.Init(
			"listen",
			fmt.Sprintf("0.0.0.0:%d", listenPacketCmd.Port),
			AgentServer, false)

		var listenPacketRet protocol.ListenPacketRet
		if err != nil {
			listenPacketRet.Success = 0
			listenPacketRet.Msg = []byte(fmt.Sprintf("%s", err))
		} else {
			listenPacketRet.Success = 1
		}
		listenPacketRet.MsgLen = uint32(len(listenPacketRet.Msg))
		packetHeader = protocol.PacketHeader{
			Separator: global.PROTOCOL_SEPARATOR,
			CmdType:   protocol.LISTEN,
			SrcHashID: utils.UUIDToArray32(node.CurrentNode.HashID),
			DstHashID: packetHeader.SrcHashID,
		}
		adminNode.WritePacket(packetHeader, listenPacketRet)
	}
}

func handleConnectCmd() {
	for {
		var packetHeader protocol.PacketHeader
		var connectPacketCmd protocol.ConnectPacketCmd

		node.CurrentNode.CommandBuffers[protocol.CONNECT].ReadPacket(&packetHeader, &connectPacketCmd)

		adminNode := node.Nodes[utils.Array32ToUUID(packetHeader.SrcHashID)]

		err := netio.Init(
			"connect",
			fmt.Sprintf("%s:%d", utils.Uint32ToIp(connectPacketCmd.IP).String(), connectPacketCmd.Port),
			AgentClient, false)

		var connectPacketRet protocol.ConnectPacketRet
		if err != nil {
			connectPacketRet.Success = 0
			connectPacketRet.Msg = []byte(fmt.Sprintf("%s", err))
		} else {
			connectPacketRet.Success = 1
		}
		connectPacketRet.MsgLen = uint32(len(connectPacketRet.Msg))
		packetHeader = protocol.PacketHeader{
			Separator: global.PROTOCOL_SEPARATOR,
			CmdType:   protocol.CONNECT,
			SrcHashID: utils.UUIDToArray32(node.CurrentNode.HashID),
			DstHashID: packetHeader.SrcHashID,
		}
		adminNode.WritePacket(packetHeader, connectPacketRet)
	}
}

func handleDownloadCmd() {
	for {
		var packetHeader protocol.PacketHeader
		var downloadPacketCmd protocol.DownloadPacketCmd

		node.CurrentNode.CommandBuffers[protocol.DOWNLOAD].ReadPacket(&packetHeader, &downloadPacketCmd)

		adminNode := node.Nodes[utils.Array32ToUUID(packetHeader.SrcHashID)]

		filePath := string(downloadPacketCmd.Path)

		var downloadPacketRet protocol.DownloadPacketRet
		var file *os.File
		var fileSize int64
		// 如果文件存在，则下载
		if utils.FileExists(filePath) {
			var err error
			file, err = os.Open(filePath)
			if err != nil {
				downloadPacketRet.Success = 0
				downloadPacketRet.Msg = []byte(fmt.Sprintf("%s", err))
			} else {
				defer file.Close()
				downloadPacketRet.Success = 1
				fileSize = utils.GetFileSize(filePath)
				downloadPacketRet.FileLen = uint64(fileSize)
			}
		} else {
			downloadPacketRet.Success = 0
			downloadPacketRet.Msg = []byte(fmt.Sprintf("%s", ERR_FILE_NOT_EXISTS))
		}

		downloadPacketRet.MsgLen = uint32(len(downloadPacketRet.Msg))

		var retPacketHeader protocol.PacketHeader
		retPacketHeader.CmdType = protocol.DOWNLOAD
		retPacketHeader.Separator = global.PROTOCOL_SEPARATOR
		retPacketHeader.SrcHashID = packetHeader.DstHashID
		retPacketHeader.DstHashID = packetHeader.SrcHashID

		adminNode.WritePacket(retPacketHeader, downloadPacketRet)

		if downloadPacketRet.Success == 0 {
			continue
		}

		var cmdPacketHeader protocol.PacketHeader
		node.CurrentNode.CommandBuffers[protocol.DOWNLOAD].ReadPacket(&cmdPacketHeader, &downloadPacketCmd)

		if downloadPacketCmd.StillDownload == 0 {
			continue
		}

		adminNode.WritePacket(retPacketHeader, downloadPacketRet)

		var dataBlockSize = uint64(global.MAX_PACKET_SIZE - 4)
		loop := int64(downloadPacketRet.FileLen / dataBlockSize)
		remainder := downloadPacketRet.FileLen % dataBlockSize

		var size int64
		for ; loop >= 0; loop-- {
			var buf []byte
			if loop > 0 {
				buf = make([]byte, dataBlockSize)
			} else {
				buf = make([]byte, remainder)
			}
			n, err := io.ReadFull(file, buf)
			if n > 0 {
				size += int64(n)
				dataPacket := protocol.FileDataPacket{
					DataLen: uint32(n),
					Data:    buf[0:n],
				}
				retPacketHeader := protocol.PacketHeader{
					Separator: global.PROTOCOL_SEPARATOR,
					SrcHashID: utils.UUIDToArray32(node.CurrentNode.HashID),
					DstHashID: packetHeader.SrcHashID,
					CmdType:   protocol.DOWNLOAD,
				}
				adminNode.WritePacket(retPacketHeader, dataPacket)
			}
			if err != nil {
				if err != io.EOF {
					log.Println("[-]Read File Error")
				}
				break
			}
		}
	}
}

func handleUploadCmd() {
	for {
		/* ------ before upload ------- */
		var packetHeader protocol.PacketHeader
		var uploadPacketCmd protocol.UploadPacketCmd
		node.CurrentNode.CommandBuffers[protocol.UPLOAD].ReadPacket(&packetHeader, &uploadPacketCmd)

		adminNode := node.Nodes[utils.Array32ToUUID(packetHeader.SrcHashID)]

		packetHeaderRet := protocol.PacketHeader{
			CmdType:   protocol.UPLOAD,
			Separator: global.PROTOCOL_SEPARATOR,
			SrcHashID: packetHeader.DstHashID,
			DstHashID: packetHeader.SrcHashID,
		}

		var uploadPacketRet protocol.UploadPacketRet

		var filePath = string(uploadPacketCmd.Path)

		var file *os.File
		// 如果文件不存在，则上传
		if !utils.FileExists(filePath) {
			var err error
			file, err = os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0644)
			if err != nil {
				uploadPacketRet.Success = 0
				uploadPacketRet.Msg = []byte(fmt.Sprintf("%s", err))
			} else {
				uploadPacketRet.Success = 1
				defer file.Close()
			}
		} else {
			uploadPacketRet.Success = 0
			uploadPacketRet.Msg = []byte(fmt.Sprintf("%s %s", filePath, ERR_FILE_EXISTS))
		}
		uploadPacketRet.MsgLen = uint32(len(uploadPacketRet.Msg))

		adminNode.WritePacket(packetHeaderRet, uploadPacketRet)

		if uploadPacketRet.Success == 0 || file == nil {
			continue
		}

		// /* ----- upload file -------- */
		node.CurrentNode.CommandBuffers[protocol.UPLOAD].ReadPacket(&packetHeader, &uploadPacketCmd)

		var uploadPacketRet2 protocol.UploadPacketRet

		var dataBlockSize = uint64(global.MAX_PACKET_SIZE - 4)
		loop := int64(uploadPacketCmd.FileLen / dataBlockSize)
		remainder := uploadPacketCmd.FileLen % dataBlockSize
		for loop >= 0 {
			if remainder != 0 {
				var fileDataPacket protocol.FileDataPacket
				var packetHeaderRet protocol.PacketHeader
				node.CurrentNode.CommandBuffers[protocol.UPLOAD].ReadPacket(&packetHeaderRet, &fileDataPacket)
				_, err := file.Write(fileDataPacket.Data)
				if err != nil {
					uploadPacketRet2.Success = 0
					uploadPacketRet2.Msg = []byte(fmt.Sprintf("%s", err))
				}
			}
			loop--
		}
		file.Close()

		uploadPacketRet2.Success = 1
		uploadPacketRet2.MsgLen = uint32(len(uploadPacketRet.Msg))
		adminNode.WritePacket(packetHeaderRet, uploadPacketRet2)
	}
}