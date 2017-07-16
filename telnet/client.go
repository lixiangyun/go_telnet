package telnet

import (
	"bytes"
	"errors"
	"fmt"
	//	"io"
	"net"
)

/*
	telnet的命令格式
	-----------------------
	  IAC | 命令码 | 选项码 |
	-----------------------
*/
const (
	CMD_EOF   = 236 //	文件结束符
	CMD_SUSP  = 237 //	挂起当前进程（作业控制）
	CMD_ABORT = 238 //	异常中止进程
	CMD_EOR   = 239 //	记录结束符i
	CMD_SE    = 240 //	子选项结束
	CMD_NOP   = 241 //	无操作
	CMD_DM    = 242 //	数据标记
	CMD_BRK   = 243 //	中断
	CMD_IP    = 244 //	中断进程
	CMD_AO    = 245 //	异常中止输出
	CMD_AYT   = 246 //	对方是否还在运行？
	CMD_EC    = 247 //	转义字符
	CMD_EL    = 248 //	删除行
	CMD_GA    = 249 //	继续进行
	CMD_SB    = 250 //	子选项开始
	CMD_WILL  = 251 //	同意启动（enable）选项
	CMD_WONT  = 252 //	拒绝启动选项
	CMD_DO    = 253 //	认可选项请求
	CMD_DONT  = 254 //	拒绝选项请求
	CMD_IAC   = 255 //	命令解释符
)

/*
	选项协商：4种请求
	1）WILL：发送方本身将激活选项
	2）DO：发送方想叫接受端激活选项
	3）WONT：发送方本身想禁止选项
	4）DONT：发送方想让接受端去禁止选项

	发送者	接收者	说明
	WILL    DO      发送者想激活某选项，接受者接收该选项请求
	WILL    DONT    发送者想激活某选项，接受者拒绝该选项请求
	DO      WILL    发送者希望接收者激活某选项，接受者接受该请求
	DO      DONT    发送者希望接收6者激活某选项，接受者拒绝该请求
	WONT    DONT    发送者希望使某选项无效，接受者必须接受该请求
	DONT    WONT    发送者希望对方使某选项无效，接受者必须接受该请求

*/

/* 选项码 */

const (
	OP_BIN_TRANS  = 0  //   二进制传输
	OP_ECHO       = 1  //	回显
	OP_SUP_GA     = 3  //	抑制继续进行
	OP_STATUS     = 5  //	状态
	OP_TIMER_MARK = 6  //	定时标记
	OP_TERM_TYPE  = 24 //	终端类型
	OP_WIN_SIZE   = 31 //	窗口大小
	OP_TERM_RATE  = 32 //	终端速度
	OP_FLOW_CTRL  = 33 //	远程流量控制
	OP_LINE_MODE  = 34 //	行方式
	OP_ENV_VAR    = 36 //	环境变量
)

const (
	MODE_CHAR       = 1
	MODE_LINE       = 2
	MY_TERM_TYPE    = "LINUX"
	CONNECT_TIMEOUT = 10
)

type Client struct {
	ServerIP   string
	ServerPort string
	TimeOut    int

	socket net.Conn

	sendcmd chan []byte
	recvque chan string
	sendque chan string

	handler func(recv string)
}

func NewClient(ip string, port string) *Client {
	return &Client{ServerIP: ip, ServerPort: port}
}

func (c *Client) Connect(handler func(recv string)) error {

	var err error

	serveraddr := c.ServerIP + ":" + c.ServerPort

	//fmt.Println(serveraddr)

	c.handler = handler
	c.recvque = make(chan string, 128)
	c.sendque = make(chan string, 128)
	c.sendcmd = make(chan []byte, 1024)

	c.socket, err = net.Dial("tcp", serveraddr)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	fmt.Println(c)

	fmt.Println("telnet.Connect success")

	return nil
}

type cmdoption struct {
	cmd    byte
	option []byte
}

type parsebuf struct {
	buf   []byte
	parse int
}

func rsp_do(req cmdoption) cmdoption {
	var rsp cmdoption
	rsp.option = make([]byte, 1)

	switch req.option[0] {
	case OP_ECHO:
		fallthrough
	case OP_SUP_GA:
		fallthrough
	case OP_BIN_TRANS:
		fallthrough
	case OP_TERM_TYPE:
		{
			rsp.cmd = CMD_WILL
			rsp.option[0] = req.option[0]
		}
	default:
		{
			rsp.cmd = CMD_WONT
			rsp.option[0] = req.option[0]
		}
	}
	return rsp
}

func rsp_will(req cmdoption) cmdoption {
	var rsp cmdoption
	rsp.option = make([]byte, 1)

	switch req.option[0] {
	case OP_SUP_GA:
		fallthrough
	case OP_BIN_TRANS:
		fallthrough
	case OP_TERM_TYPE:
		{
			rsp.cmd = CMD_DO
			rsp.option[0] = req.option[0]
		}
	default:
		{
			rsp.cmd = CMD_DONT
			rsp.option[0] = req.option[0]
		}
	}
	return rsp
}

func rsp_dont(req cmdoption) cmdoption {
	var rsp cmdoption
	rsp.option = make([]byte, 1)

	rsp.cmd = CMD_WONT
	rsp.option[0] = req.option[0]
	return rsp
}

func rsp_wont(req cmdoption) cmdoption {
	var rsp cmdoption
	rsp.option = make([]byte, 1)

	rsp.cmd = CMD_DONT
	rsp.option[0] = req.option[0]
	return rsp
}

func rsp_sb(req cmdoption) cmdoption {
	var rsp cmdoption
	rsp.option = make([]byte, 2)

	switch req.option[0] {
	case OP_TERM_TYPE:
		{
			rsp.cmd = CMD_SB
			rsp.option[0] = OP_TERM_TYPE
			rsp.option[1] = 0
			rsp.option = append(rsp.option, []byte("LINUX")...)
			rsp.option = append(rsp.option, 5)
			rsp.option = append(rsp.option, CMD_IAC)
			rsp.option = append(rsp.option, CMD_SE)
		}
	default:
		{
			rsp.cmd = 0
			return rsp
		}
	}
	return rsp
}

func rspcmdopt(req cmdoption) cmdoption {
	var rsp cmdoption

	switch req.cmd {
	case CMD_DO:
		rsp = rsp_do(req)
	case CMD_WILL:
		rsp = rsp_will(req)
	case CMD_DONT:
		rsp = rsp_dont(req)
	case CMD_WONT:
		rsp = rsp_wont(req)
	case CMD_SB:
		rsp = rsp_sb(req)
	default:
		rsp.cmd = 0
	}

	return rsp
}

func getcmdopt(p *parsebuf) *cmdoption {
	parse := p.parse

	if parse+1 > len(p.buf) {
		return nil
	}

	if p.buf[parse] != CMD_IAC {
		p.parse = parse + 1
		return nil
	}

	if parse+2 > len(p.buf) {
		return nil
	}

	if p.buf[parse] == CMD_IAC && p.buf[parse+1] == CMD_IAC {
		p.parse = parse + 2
		return nil
	}

	var co cmdoption
	co.option = make([]byte, 1)

	co.cmd = p.buf[parse+1]
	co.option[0] = p.buf[parse+2]

	parse = parse + 2

	if co.cmd == CMD_SB {
		for parse < len(p.buf) {
			co.option = append(co.option, p.buf[parse])
			if p.buf[parse] == CMD_SE && p.buf[parse-1] == CMD_IAC {
				parse++
				break
			}
			parse++
		}
	}

	p.parse = parse
	return &co
}

func cmdProc(buf []byte, sendcmd chan []byte) []byte {

	var p parsebuf

	fmt.Println("cmd proc...")

	p.buf = buf
	p.parse = 0

	for {
		co := getcmdopt(&p)
		if nil == co {
			break
		}

		rsp := rspcmdopt(*co)
		if rsp.cmd != 0 {

			cmdbuf := make([]byte, 0)
			cmdbuf = append(cmdbuf, rsp.cmd)
			cmdbuf = append(cmdbuf, rsp.option...)

			sendcmd <- cmdbuf
		}
	}

	if p.parse < len(p.buf) {
		return p.buf[p.parse:]
	} else {
		return nil
	}
}

func send(c *Client, buf []byte) error {
	var temp = 0
	total := len(buf)

	fmt.Println("send...")

	for {
		n, err := c.socket.Write(buf[temp:])

		fmt.Println(buf[temp:])

		if err != nil {
			return err
		}

		if n+temp >= total {
			break
		}

		temp += n
	}

	return nil
}

func recv(c *Client) ([]byte, error) {
	var buf [512]byte

	fmt.Println("recv...")

	result := bytes.NewBuffer(nil)

	for {

		n, err := c.socket.Read(buf[0:])

		result.Write(buf[0:n])

		if err == nil {
			break
		} else {
			return nil, err
		}
	}

	fmt.Println(result.Bytes())

	return result.Bytes(), nil
}

func recvtask(c *Client) {

	fmt.Println("Recv Task init")

	lastbuf := make([]byte, 0)

	for {
		tempbuf, err := recv(c)

		if err != nil {
			fmt.Println("RecvTask failed: ", err.Error())
			break
		}

		//debug

		fmt.Println(tempbuf)
		//fmt.Println(string(tempbuf))

		lastbuf = append(lastbuf, tempbuf...)

		for i, v := range lastbuf {
			if v == CMD_IAC {
				tempbuf = cmdProc(lastbuf[i:], c.sendcmd)
				if tempbuf != nil {
					lastbuf = append(lastbuf[0:i], tempbuf...)
				}
				break
			}
		}

		c.recvque <- string(lastbuf[0:])

		lastbuf = make([]byte, 0)
	}
}

func sendtask(c *Client) {

	fmt.Println("Send Task init")

	err := errors.New("")

	for {

		select {
		case buf := <-c.sendcmd:
			err = send(c, buf)
		case str := <-c.sendque:
			err = send(c, []byte(str))
		}

		if err != nil {
			fmt.Println("send failed: ", err.Error())
			break
		}
	}
}

func (c *Client) Process() error {

	if nil == c.handler {
		return errors.New("The handler mast been register.")
	}

	go recvtask(c)
	go sendtask(c)

	for {
		recv := <-c.recvque
		c.handler(recv)
	}

	return errors.New("shutdown")
}

func (c *Client) Write(send string) {
	c.sendque <- send
}

func (c *Client) Delete() {
	c.socket.Close()
}
