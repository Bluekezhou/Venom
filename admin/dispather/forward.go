package dispather

import (
	"io"

	"github.com/Dliv3/Venom/global"
	"github.com/Dliv3/Venom/node"
	"github.com/Dliv3/Venom/protocol"
	"github.com/Dliv3/Venom/utils"
	"github.com/Dliv3/Venom/utils/terminal"
)

// CopyStdin2Node 把键盘输入发送到远端
func CopyStdin2Node(stdReader *utils.CancelableReader, output *node.Node, c chan bool) {

	bufTerm := terminal.NewBufTerm(0x100)
	buf := make([]byte, 1)

	for {
		// 在terminal开启的情况下，input.Read每次只能读到一个字节
		// 这在某种程度上会降低通信的效率，但是可以较好地支持shell交互
		_, err := utils.StdReader.Read(buf)
		if err != nil {
			break
		}

		data := protocol.ShellPacketCmd{
			Start:  1,
			CmdLen: uint32(len(buf)),
			Cmd:    buf,
		}
		packetHeader := protocol.PacketHeader{
			Separator: global.PROTOCOL_SEPARATOR,
			CmdType:   protocol.SHELL,
			SrcHashID: utils.UUIDToArray32(node.CurrentNode.HashID),
			DstHashID: utils.UUIDToArray32(output.HashID),
		}
		writeErr := output.WritePacket(packetHeader, data)
		if writeErr != nil {
			// 强制结束 CommandBuffers[protocol.SHELL]
			node.CurrentNode.CommandBuffers[protocol.SHELL].WriteCloseMessage()
		}

		cmdBuf := bufTerm.TerminalEmu(buf[0])
		// 注意：在terminal模式下，输入回车读到的字符是\x0d，而不是'\n'
		if string(cmdBuf) == "exit\x0d" {
			// fmt.Println("exiting shell mode!!!")
			break
		}
	}
	c <- true
	// fmt.Println("CopyStdin2Node Exit")

	return
}

// CopyNode2Stdout 接收peerNode发送过来的数据
func CopyNode2Stdout(input *node.Node, output io.Writer, c chan bool) {
	for {
		var packetHeader protocol.PacketHeader
		var shellPacketRet protocol.ShellPacketRet
		err := node.CurrentNode.CommandBuffers[protocol.SHELL].ReadPacket(&packetHeader, &shellPacketRet)
		if err != nil {
			// fmt.Println("press any key to exit shell mode")
			utils.StdReader.SendCancelMessage()
			break
		}
		if shellPacketRet.Success == 0 {
			break
		}
		output.Write(shellPacketRet.Data)
	}
	c <- true
	// fmt.Println("CopyNode2Stdout Exit")

	return
}
