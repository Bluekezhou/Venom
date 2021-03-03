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

// CopyStdoutPipe2Node 把子进程输出发送到目标节点
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
			node.CurrentNode.CommandBuffers[protocol.SHELL].WriteErrorMessage("Shell-Exit")
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

// CopyNode2StdinPipe 把从目标节点接受的数据发送到子进程
func CopyNode2StdinPipe(input *node.Node, output io.Writer, c chan bool, cmd *exec.Cmd) {
	for {
		var packetHeader protocol.PacketHeader
		var shellPacketCmd protocol.ShellPacketCmd
		err := node.CurrentNode.CommandBuffers[protocol.SHELL].ReadPacket(&packetHeader, &shellPacketCmd)

		if err != nil {
			fmt.Printf("err %s\n", err.Error())
			if err.Error() == "EOF" {
				// 网络中断，先把shell退出
				fmt.Println("internet error")
				output.Write([]byte("exit\n"))
				err = node.CurrentNode.CommandBuffers[protocol.SHELL].ReadPacket(&packetHeader, &shellPacketCmd)
			}

			if err.Error() != "Shell-Exit" {
				fmt.Println("error should be Shell-Exit, check it")
			}
			break
		}

		if shellPacketCmd.Start == 0 {
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
