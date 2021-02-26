package dispather

import (
	"bytes"
	"io"
	"runtime"

	"github.com/Dliv3/Venom/global"
	"github.com/Dliv3/Venom/node"
	"github.com/Dliv3/Venom/protocol"
	"github.com/Dliv3/Venom/utils"
)

func CopyStdin2Node(input io.Reader, output *node.Node, c chan bool) {

	orgBuf := make([]byte, 0x10)
	cmdBuf := make([]byte, 0x10)
	cmdBufIndex := 0

	for {
		// 在terminal开启的情况下，input.Read每次只能读到一个字节
		// 这在某种程度上会降低通信的效率，但是可以较好地支持shell交互
		count, err := input.Read(orgBuf)

		// fmt.Println(orgBuf[:count])

		var buf []byte

		if runtime.GOOS == "windows" {
			buf = bytes.Replace(orgBuf[:count], []byte("\r"), []byte(""), -1)
			count = len(buf)
		} else {
			buf = orgBuf
		}
		// fmt.Println(buf[:count])
		if count > 0 {
			data := protocol.ShellPacketCmd{
				Start:  1,
				CmdLen: uint32(count),
				Cmd:    buf[:count],
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

			if buf[0] == 0x7f && cmdBufIndex > 0 {
				// 模拟删除操作
				cmdBufIndex--
			} else {
				cmdBuf[cmdBufIndex] = buf[0]
				cmdBufIndex++
			}

			// 注意：在terminal模式下，输入回车读到的字符是\x0d，而不是'\n'
			if string(cmdBuf[:cmdBufIndex]) == "exit\x0d" {
				break
			}

			if cmdBufIndex == len(cmdBuf)-1 || buf[0] == 0xd {
				cmdBufIndex = 0
			}
		}
		if err != nil {
			break
		}
	}
	c <- true
	// fmt.Println("CopyStdin2Node Exit")

	return
}

func CopyNode2Stdout(input *node.Node, output io.Writer, c chan bool) {
	for {
		var packetHeader protocol.PacketHeader
		var shellPacketRet protocol.ShellPacketRet
		err := node.CurrentNode.CommandBuffers[protocol.SHELL].ReadPacket(&packetHeader, &shellPacketRet)
		if err != nil {
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
