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
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/divan/gorilla-xmlrpc/xml"
	"github.com/gorilla/rpc"

	mc "github.com/hshsobug/mc-master/cmd"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var globalHTTPParameters HTTPParameters
var exePath string

func main() {
	// 获取exe绝对路径
	exePath, err := os.Executable()
	if err != nil {
		log.Println("Executable Error:", err)
		return
	}

	log.Println("Executable exePath:", exePath)

	// 获取命令行参数 ./mc.exe 6666
	log.Println("os.Args:", os.Args)

	port := os.Args[1]
	// 集成xmlrpc
	RPC := rpc.NewServer()
	xmlrpcCodec := xml.NewCodec()
	RPC.RegisterCodec(xmlrpcCodec, "text/xml")
	RPC.RegisterService(new(HTTPService), "")
	http.Handle("/xmlrpc/SftpSetParameters", RPC)
	http.Handle("/xmlrpc/SftpStartTransfer", RPC)
	http.Handle("/xmlrpc/SftpGetStat", RPC)
	http.Handle("/xmlrpc/SftpStopTransfer", RPC)

	// 将 http.ListenAndServe 放在 goroutine 中运行
	// go func() {
	log.Println("Starting XML-RPC server on localhost:" + port)
	log.Println(http.ListenAndServe(":"+port, nil))
	// }()

}

type HTTPService struct{}
type HTTPParameters struct {
	Username           string
	Password           string
	Host               string
	Port               string
	ProxyType          string
	ProxyHost          string
	ProxyPort          string
	ProxyUser          string
	ProxyPwd           string
	Compression        string
	Verbose            string
	Minioadminuser     string
	Minioadminpassword string
	BucketIsExists     bool
}
type HTTPResponse struct {
	Message string
}

// SftpSetParameters - 设置登录参数
func (s *HTTPService) SftpSetParameters(r *http.Request, args *HTTPParameters, reply *HTTPResponse) error {
	log.Printf("Received parameters: %+v", args)

	// 更新设置变量,重置设置标志
	globalHTTPParameters = *args

	// 需要设置环境
	log.Println("globalHttpParameters setting.")
	httpHostParams := []string{exePath, "config", "host", "add", "minio-server", "http://" + globalHTTPParameters.Host + ":" + globalHTTPParameters.Port, globalHTTPParameters.Minioadminuser, globalHTTPParameters.Minioadminpassword}
	if e := mc.Main(httpHostParams); e != nil {
		log.Println("minio config host add minio-server error.", e)
		reply.Message = e.Error()
		return nil
	} else {
		log.Println("minio config host add minio-server success.")
	}

	reply.Message = ""
	return nil
}

// HTTPTransferParameters - 传输参数
type HTTPTransferParameters struct {
	Cmd string
	Src string
	Dst string
}

// SftpStartTransfer - 处理文件传输请求
func (s *HTTPService) SftpStartTransfer(r *http.Request, args *HTTPTransferParameters, reply *HTTPResponse) error {
	log.Printf("Received transfer parameters: %+v", args)
	reply.Message = ""

	// 在这里处理文件传输逻辑
	// 根据args.Cmd的值（reput、put、reget、get）来决定是上传还是下载
	// args.Src和args.Dst分别表示源文件和目标文件
	// 多个文件用 | 隔开，可以根据需要分割字符串来处理多个文件

	// 上传前检查桶名是否存在，不存在则创建
	if !globalHTTPParameters.BucketIsExists {
		endpoint := globalHTTPParameters.Host + ":" + globalHTTPParameters.Port
		accessKeyID := globalHTTPParameters.Minioadminuser
		secretAccessKey := globalHTTPParameters.Minioadminpassword
		useSSL := false

		// 初始化minio客户端对象
		minioClient, err := minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
			Secure: useSSL,
		})
		if err != nil {
			log.Fatalln(err)
		}

		// 检查bucket是否存在
		exists, err := minioClient.BucketExists(context.Background(), globalHTTPParameters.Username)
		if err != nil {
			log.Println("minio BucketExists error.", err)
		}
		if exists {
			log.Printf("Bucket %s already exists\n", globalHTTPParameters.Username)
			globalHTTPParameters.BucketIsExists = exists
		} else {
			log.Printf("Bucket %s does not exist\n", globalHTTPParameters.Username)
			bucket := "minio-server/" + globalHTTPParameters.Username
			mbParams := []string{exePath, "mb", bucket}
			log.Println("minio mb " + bucket)
			if e := mc.Main(mbParams); e != nil {
				reply.Message = e.Error()
				log.Println("minio mb error.", e)
			} else {
				log.Println("minio mb success.")
				globalHTTPParameters.BucketIsExists = true
			}
		}
	}

	// 示例：打印参数并返回成功消息
	cmds := strings.Split(args.Cmd, "|")
	srcs := strings.Split(args.Src, "|")
	dsts := strings.Split(args.Dst, "|")
	log.Printf("Cmds: %v, Srcs: %v, Dsts: %v", cmds, srcs, dsts)

	// sobug
	go func() {
		// 另起线程提交上传任务
		uploadParams := []string{exePath, cmds[0], srcs[0], dsts[0], "minio-server/" + globalHTTPParameters.Username}
		if e := mc.Main(uploadParams); e != nil {
			log.Println("Main upload error.", e)
			reply.Message = e.Error()
		} else {
			log.Println("Main upload success.")
		}
	}()

	return nil
}

// HTTPGetStatParameters - 获取传输状态参数
type HTTPGetStatParameters struct {
}

// SftpGetStat - 获取传输状态
func (s *HTTPService) SftpGetStat(r *http.Request, args *HTTPGetStatParameters, reply *HTTPResponse) error {
	log.Printf("Received SftpGetStat parameters: %+v", args)
	// 获取上传文件的传输状态
	if mc.ProgressReaderInstance != nil {
		reply.Message = mc.GetProgressStr(mc.ProgressReaderInstance)
	} else {
		log.Printf("mc.ProgressReaderInstance is nil")
		reply.Message = "mc.ProgressReaderInstance is nil"
	}
	return nil
}

// SftpStopTransfer - 停止传输接口
func (s *HTTPService) SftpStopTransfer(r *http.Request, args *HTTPGetStatParameters, reply *HTTPResponse) error {
	log.Printf("Received SftpStopTransfer parameters: %+v", args)
	mc.CancelFilePut() // 停止传输
	reply.Message = ""
	return nil
}
