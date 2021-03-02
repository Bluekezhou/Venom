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
func CopyStdin2Node(stdReader *utils.CancelableReader, output *node.Node, c chan bool, istty bool) {

	bufTerm := terminal.NewBufTerm(0x100)
	buf := make([]byte, 1)

	for {
		if istty {
			// 在terminal开启的情况下，input.Read每次只能读到一个字节
			_, err := utils.StdReader.Read(buf)
			if err != nil {
				break
			}
		} else {
			var line string
			var err error
			// 目标不支持tty，在本地模拟terminal功能
			terminal.GTerminal.SetPrompt("$ ")
			if line, err = terminal.GTerminal.ReadLine(); err != nil {
				break
			}

			buf = append([]byte(line), 0xd)
			// fmt.Printf("len buf %d\n", len(buf))
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

		var cmdBuf []byte
		if istty {
			cmdBuf = bufTerm.TerminalEmu(buf[0])
		} else {
			cmdBuf = buf
		}

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
