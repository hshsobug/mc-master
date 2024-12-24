// Copyright (c) 2015-2021 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main // import "github.com/minio/mc"

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/divan/gorilla-xmlrpc/xml"
	"github.com/gorilla/rpc"

	mc "github.com/minio/mc/cmd"
)

func main() {
	// ./mc.exe 6666
	log.Println("os.Args:", os.Args)

	port := os.Args[1]
	// 集成xmlrpc
	RPC := rpc.NewServer()
	xmlrpcCodec := xml.NewCodec()
	RPC.RegisterCodec(xmlrpcCodec, "text/xml")
	RPC.RegisterService(new(SftpService), "")
	http.Handle("/xmlrpc/SftpSetParameters", RPC)
	http.Handle("/xmlrpc/SftpStartTransfer", RPC)

	// 将 http.ListenAndServe 放在 goroutine 中运行
	// go func() {
	log.Println("Starting XML-RPC server on localhost:" + port)
	log.Fatalln(http.ListenAndServe(":"+port, nil))
	// }()

}

type SftpService struct{}
type SftpParameters struct {
	Username    string
	Password    string
	Host        string
	Port        int
	ProxyType   int
	ProxyHost   string
	ProxyPort   int
	ProxyUser   string
	ProxyPwd    string
	Compression bool
	Verbose     bool
}
type SftpResponse struct {
	Message string
}

// SftpSetParameters - 设置登录参数
func (s *SftpService) SftpSetParameters(r *http.Request, args *SftpParameters, reply *SftpResponse) error {
	log.Printf("Received parameters: %+v", args)

	reply.Message = "Success"
	return nil
}

// SftpTransferParameters - 传输参数
type SftpTransferParameters struct {
	Cmd string
	Src string
	Dst string
}

// SftpStartTransfer - 处理文件传输请求
func (s *SftpService) SftpStartTransfer(r *http.Request, args *SftpTransferParameters, reply *SftpResponse) error {
	log.Printf("Received transfer parameters: %+v", args)

	// 在这里处理文件传输逻辑
	// 根据args.Cmd的值（reput、put、reget、get）来决定是上传还是下载
	// args.Src和args.Dst分别表示源文件和目标文件
	// 多个文件用 | 隔开，可以根据需要分割字符串来处理多个文件

	// 示例：打印参数并返回成功消息
	cmds := strings.Split(args.Cmd, "|")
	srcs := strings.Split(args.Src, "|")
	dsts := strings.Split(args.Dst, "|")
	log.Printf("Cmds: %v, Srcs: %v, Dsts: %v", cmds, srcs, dsts)

	log.Println("mc main.go update mc run.")
	// sobug
	params := []string{"G:\\mc.exe", cmds[0], srcs[0], dsts[0], "minio-server/sobug4"}
	if e := mc.Main(params); e != nil {
		log.Println("Main error.", e)
		reply.Message = e.Error()
	} else {
		reply.Message = "Transfer started"
	}

	return nil
}
