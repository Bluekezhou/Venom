package dispather

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/Dliv3/Venom/global"
	"github.com/Dliv3/Venom/node"
	"github.com/Dliv3/Venom/protocol"
	"github.com/Dliv3/Venom/utils"
)

func CopyStdoutPipe2Node(input io.Reader, output *node.Node, c chan bool) {
	buf := make([]byte, global.MAX_PACKET_SIZE-8)
	for {
		count, err := input.Read(buf)
		data := protocol.ShellPacketRet{
			Success: 1,
			DataLen: uint32(count),
			Data:    buf[:count],
		}
		packetHeader := protocol.PacketHeader{
			Separator: global.PROTOCOL_SEPARATOR,
			CmdType:   protocol.SHELL,
			SrcHashID: utils.UUIDToArray32(node.CurrentNode.HashID),
			DstHashID: utils.UUIDToArray32(output.HashID),
		}
		if err != nil {
			fmt.Println("bash exited")
			// bash进程退出，给CopyNode2StdinPipe线程发送一个退出的消息
			// 这里和处理网络连接异常断开的方式保持一致
			data := protocol.ShellPacketCmd{
				// 如果是0，exit不会被发送给命令行，CopyNode2StdinPipe
				// 如果是1，不会触发continue操作，handleShellCmd
				Start:  2,
				CmdLen: uint32(5),
				Cmd:    []byte("exit\n"),
			}
			packet := protocol.Packet{
				Separator: global.PROTOCOL_SEPARATOR,
				CmdType:   protocol.SHELL,
				SrcHashID: utils.UUIDToArray32(output.HashID),
				DstHashID: utils.UUIDToArray32(node.CurrentNode.HashID),
			}

			packet.PackData(data)
			node.CurrentNode.CommandBuffers[protocol.SHELL].WriteLowLevelPacket(packet)
			if count > 0 {
				output.WritePacket(packetHeader, data)
			}
			break
		}
		if count > 0 {
			output.WritePacket(packetHeader, data)
		}
	}
	c <- true
	// fmt.Println("CopyStdoutPipe2Node Exit")

	return
}

func CopyNode2StdinPipe(input *node.Node, output io.Writer, c chan bool, cmd *exec.Cmd) {
	for {
		var packetHeader protocol.PacketHeader
		var shellPacketCmd protocol.ShellPacketCmd
		err := node.CurrentNode.CommandBuffers[protocol.SHELL].ReadPacket(&packetHeader, &shellPacketCmd)
		if shellPacketCmd.Start == 0 {
			break
		}
		if err != nil {
			break
		}
		output.Write(shellPacketCmd.Cmd)
		if string(shellPacketCmd.Cmd) == "exit\n" {
			break
		}
	}
	c <- true
	// fmt.Println("CopyNode2StdinPipe Exit")

	return
}
